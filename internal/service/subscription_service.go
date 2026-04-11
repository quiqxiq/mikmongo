package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	mkdomain "github.com/Butterfly-Student/go-ros/domain"
	"github.com/google/uuid"

	"mikmongo/internal/domain/subscription"
	"mikmongo/internal/model"
	"mikmongo/internal/repository"
)

// PPPSecretConfig holds MikroTik-only fields for PPP secrets (not stored in DB).
type PPPSecretConfig struct {
	Service        *string
	LocalAddress   *string
	RemoteAddress  *string // overrides StaticIP for RouterOS remote-address when set
	Comment        *string // overrides default "sub:<id>" when set
	Routes         *string
	LimitBytesIn   *int64
	LimitBytesOut  *int64
}

// SubscriptionService handles subscription business logic
type SubscriptionService struct {
	subRepo        repository.SubscriptionRepository
	profileRepo    repository.BandwidthProfileRepository
	settingRepo    repository.SystemSettingRepository
	subDomain      *subscription.Domain
	routerProvider MikrotikProvider
	cache          CacheClient // nil → caching disabled (graceful degradation)
}

// NewSubscriptionService creates a new subscription service
func NewSubscriptionService(
	subRepo repository.SubscriptionRepository,
	profileRepo repository.BandwidthProfileRepository,
	settingRepo repository.SystemSettingRepository,
	subDomain *subscription.Domain,
	routerProvider MikrotikProvider,
	cache CacheClient,
) *SubscriptionService {
	return &SubscriptionService{
		subRepo:        subRepo,
		profileRepo:    profileRepo,
		settingRepo:    settingRepo,
		subDomain:      subDomain,
		routerProvider: routerProvider,
		cache:          cache,
	}
}

// Create creates a new subscription and creates PPP secret in MikroTik.
// mtCfg carries optional MikroTik-only fields; pass nil to use defaults.
func (s *SubscriptionService) Create(ctx context.Context, sub *model.Subscription, mtCfg *PPPSecretConfig) error {
	if mtCfg == nil {
		mtCfg = &PPPSecretConfig{}
	}
	// Validate profile belongs to the same router
	planID, err := uuid.Parse(sub.PlanID)
	if err != nil {
		return fmt.Errorf("invalid plan ID: %w", err)
	}
	profile, err := s.profileRepo.GetByID(ctx, planID)
	if err != nil {
		return fmt.Errorf("plan not found: %w", err)
	}
	if profile.RouterID != sub.RouterID {
		return fmt.Errorf("profile does not belong to the specified router")
	}

	if sub.Password == "" {
		pass, err := s.subDomain.GeneratePassword(12)
		if err != nil {
			return fmt.Errorf("failed to generate password: %w", err)
		}
		sub.Password = pass
	}
	if err := s.subDomain.ValidateCredentials(sub.Username, sub.Password); err != nil {
		return err
	}
	sub.Status = "pending"

	// Connect to MikroTik first
	routerID, _ := uuid.Parse(sub.RouterID)
	mt, err := s.routerProvider.GetMikrotikAdapter(ctx, routerID)
	if err != nil {
		return fmt.Errorf("failed to connect to router: %w", err)
	}

	if err := s.createInMikroTik(ctx, mt, sub, profile, mtCfg); err != nil {
		return fmt.Errorf("failed to create in mikrotik: %w", err)
	}

	// Save to database only if MikroTik succeeded
	if err := s.subRepo.Create(ctx, sub); err != nil {
		// Rollback: remove from MikroTik
		_ = s.removeFromMikroTik(ctx, mt, sub)
		return fmt.Errorf("failed to save subscription: %w", err)
	}

	s.invalidateSub(ctx, sub.ID, sub.RouterID, sub.Username)
	return nil
}

// GetByID gets subscription by ID with cache-aside.
func (s *SubscriptionService) GetByID(ctx context.Context, id uuid.UUID) (*model.Subscription, error) {
	if s.cache != nil {
		if raw, err := s.cache.Get(ctx, keySubscription(id.String())); err == nil {
			var m model.Subscription
			if json.Unmarshal([]byte(raw), &m) == nil {
				return &m, nil
			}
		}
	}

	m, err := s.subRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if s.cache != nil {
		if b, err := json.Marshal(m); err == nil {
			_ = s.cache.Set(ctx, keySubscription(id.String()), b, ttlSub)
		}
	}
	return m, nil
}

// GetByCustomerID gets subscriptions by customer ID
func (s *SubscriptionService) GetByCustomerID(ctx context.Context, customerID uuid.UUID) ([]model.Subscription, error) {
	return s.subRepo.GetByCustomerID(ctx, customerID)
}

// GetByUsername gets subscription by username
func (s *SubscriptionService) GetByUsername(ctx context.Context, username string) (*model.Subscription, error) {
	return s.subRepo.GetByUsername(ctx, username)
}

// List lists subscriptions with pagination
func (s *SubscriptionService) List(ctx context.Context, limit, offset int) ([]model.Subscription, int64, error) {
	subs, err := s.subRepo.List(ctx, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	count, err := s.subRepo.Count(ctx)
	return subs, count, err
}

// ListByRouterID lists subscriptions by router ID
func (s *SubscriptionService) ListByRouterID(ctx context.Context, routerID uuid.UUID, limit, offset int) ([]model.Subscription, int64, error) {
	subs, err := s.subRepo.ListByRouterID(ctx, routerID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	count, err := s.subRepo.CountByRouterID(ctx, routerID)
	return subs, count, err
}

// Update updates a subscription and updates PPP secret in MikroTik.
// mtCfg carries optional MikroTik-only fields; pass nil to use defaults.
func (s *SubscriptionService) Update(ctx context.Context, sub *model.Subscription, mtCfg *PPPSecretConfig) error {
	if mtCfg == nil {
		mtCfg = &PPPSecretConfig{}
	}

	// Get existing subscription
	existingSub, err := s.subRepo.GetByID(ctx, uuid.MustParse(sub.ID))
	if err != nil {
		return err
	}

	// Validate profile belongs to the same router if PlanID changed
	if sub.PlanID != "" && sub.PlanID != existingSub.PlanID {
		planID, err := uuid.Parse(sub.PlanID)
		if err != nil {
			return fmt.Errorf("invalid plan ID: %w", err)
		}
		profile, err := s.profileRepo.GetByID(ctx, planID)
		if err != nil {
			return fmt.Errorf("plan not found: %w", err)
		}
		if profile.RouterID != sub.RouterID {
			return fmt.Errorf("profile does not belong to the specified router")
		}
	}

	// Connect to MikroTik
	routerID, _ := uuid.Parse(sub.RouterID)
	mt, err := s.routerProvider.GetMikrotikAdapter(ctx, routerID)
	if err != nil {
		return fmt.Errorf("failed to connect to router: %w", err)
	}

	// Get profile for MikroTik sync
	planID, _ := uuid.Parse(sub.PlanID)
	profile, err := s.profileRepo.GetByID(ctx, planID)
	if err != nil {
		return fmt.Errorf("plan not found: %w", err)
	}

	if err := s.updateInMikroTik(ctx, mt, sub, profile, mtCfg, existingSub); err != nil {
		return fmt.Errorf("failed to update in mikrotik: %w", err)
	}

	if err := s.subRepo.Update(ctx, sub); err != nil {
		oldPlanID, perr := uuid.Parse(existingSub.PlanID)
		if perr == nil {
			if oldProfile, gerr := s.profileRepo.GetByID(ctx, oldPlanID); gerr == nil {
				_ = s.updateInMikroTik(ctx, mt, existingSub, oldProfile, &PPPSecretConfig{}, existingSub)
			}
		}
		return fmt.Errorf("failed to update subscription: %w", err)
	}

	s.invalidateSub(ctx, sub.ID, sub.RouterID, sub.Username)
	return nil
}

// Delete deletes a subscription and removes PPP secret from MikroTik
func (s *SubscriptionService) Delete(ctx context.Context, id uuid.UUID) error {
	// Get subscription before delete
	sub, err := s.subRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	// Connect to MikroTik
	routerID, _ := uuid.Parse(sub.RouterID)
	mt, err := s.routerProvider.GetMikrotikAdapter(ctx, routerID)
	if err != nil {
		return fmt.Errorf("failed to connect to router: %w", err)
	}

	// Remove from MikroTik first
	if err := s.removeFromMikroTik(ctx, mt, sub); err != nil {
		return fmt.Errorf("failed to remove from mikrotik: %w", err)
	}

	if err := s.subRepo.Delete(ctx, id); err != nil {
		return err
	}

	s.invalidateSub(ctx, sub.ID, sub.RouterID, sub.Username)
	return nil
}

// Activate activates a subscription: enable PPP secret + set status=active
func (s *SubscriptionService) Activate(ctx context.Context, id uuid.UUID) error {
	sub, err := s.subRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if err := s.subDomain.ValidateStatusTransition(sub.Status, "active"); err != nil {
		return err
	}

	// Connect to MikroTik and enable PPP secret
	routerID, _ := uuid.Parse(sub.RouterID)
	mt, err := s.routerProvider.GetMikrotikAdapter(ctx, routerID)
	if err != nil {
		return fmt.Errorf("failed to connect to router: %w", err)
	}

	if err := s.enableInMikroTik(ctx, mt, sub); err != nil {
		return fmt.Errorf("failed to enable in mikrotik: %w", err)
	}

	now := time.Now()
	sub.Status = "active"
	sub.ActivatedAt = &now
	if err := s.subRepo.Update(ctx, sub); err != nil {
		return err
	}
	s.invalidateSub(ctx, sub.ID, sub.RouterID, sub.Username)
	return nil
}

// buildPPPSecret builds RouterOS /ppp/secret payload: name, password, service (default pppoe), profile, local/remote-address, comment, routes, byte limits.
func (s *SubscriptionService) buildPPPSecret(sub *model.Subscription, profile *model.BandwidthProfile, mtCfg *PPPSecretConfig) *mkdomain.PPPSecret {
	if mtCfg == nil {
		mtCfg = &PPPSecretConfig{}
	}
	secret := &mkdomain.PPPSecret{
		Name:     sub.Username,
		Password: sub.Password,
		Profile:  profile.Name,
		Service:  "pppoe",
	}
	if mtCfg.Comment != nil && *mtCfg.Comment != "" {
		secret.Comment = *mtCfg.Comment
	} else {
		secret.Comment = fmt.Sprintf("sub:%s", sub.ID)
	}
	if mtCfg.Service != nil && *mtCfg.Service != "" {
		secret.Service = *mtCfg.Service
	}
	if mtCfg.LocalAddress != nil {
		secret.LocalAddress = *mtCfg.LocalAddress
	}
	if mtCfg.RemoteAddress != nil && *mtCfg.RemoteAddress != "" {
		secret.RemoteAddress = *mtCfg.RemoteAddress
	} else if sub.StaticIP != nil {
		secret.RemoteAddress = *sub.StaticIP
	}
	if mtCfg.Routes != nil {
		secret.Routes = *mtCfg.Routes
	}
	if mtCfg.LimitBytesIn != nil {
		secret.LimitBytesIn = *mtCfg.LimitBytesIn
	}
	if mtCfg.LimitBytesOut != nil {
		secret.LimitBytesOut = *mtCfg.LimitBytesOut
	}
	return secret
}

// createInMikroTik creates PPP secret in MikroTik and captures the RouterOS ID. Fails if the username already exists on the router.
func (s *SubscriptionService) createInMikroTik(ctx context.Context, mt MikrotikClientAdapter, sub *model.Subscription, profile *model.BandwidthProfile, mtCfg *PPPSecretConfig) error {
	existing, err := mt.GetSecretByName(ctx, sub.Username)
	if err != nil {
		return fmt.Errorf("mikrotik: lookup ppp secret: %w", err)
	}
	if existing != nil {
		return fmt.Errorf("%w: %q", ErrMikrotikPPPSecretExists, sub.Username)
	}

	secret := s.buildPPPSecret(sub, profile, mtCfg)
	if err := mt.AddSecret(ctx, secret); err != nil {
		return err
	}

	created, err := mt.GetSecretByName(ctx, sub.Username)
	if err != nil {
		return fmt.Errorf("mikrotik: read ppp secret after add: %w", err)
	}
	if created == nil {
		return fmt.Errorf("mikrotik: ppp secret %q not found after add", sub.Username)
	}
	sub.MtPPPID = &created.ID
	return nil
}

// resolvePPPSecretID returns RouterOS .id for the subscription's PPP secret (stored ID or live lookup by username).
func (s *SubscriptionService) resolvePPPSecretID(ctx context.Context, mt MikrotikClientAdapter, sub *model.Subscription) (string, error) {
	if sub.MtPPPID != nil && *sub.MtPPPID != "" {
		return *sub.MtPPPID, nil
	}
	existing, err := mt.GetSecretByName(ctx, sub.Username)
	if err != nil {
		return "", fmt.Errorf("mikrotik: lookup ppp secret: %w", err)
	}
	if existing == nil {
		return "", fmt.Errorf("%w: name=%q", ErrMikrotikPPPSecretNotFound, sub.Username)
	}
	return existing.ID, nil
}

// updateInMikroTik updates an existing PPP secret. secretLookup is the subscription row used to resolve RouterOS .id (before username/plan changes).
func (s *SubscriptionService) updateInMikroTik(ctx context.Context, mt MikrotikClientAdapter, desired *model.Subscription, profile *model.BandwidthProfile, mtCfg *PPPSecretConfig, secretLookup *model.Subscription) error {
	if secretLookup == nil {
		secretLookup = desired
	}
	id, err := s.resolvePPPSecretID(ctx, mt, secretLookup)
	if err != nil {
		return err
	}
	return mt.UpdateSecret(ctx, id, s.buildPPPSecret(desired, profile, mtCfg))
}

// getIsolateProfile returns the isolate profile name from system_settings or the profile's override
func (s *SubscriptionService) getIsolateProfile(ctx context.Context, profileOverride *string) string {
	if profileOverride != nil && *profileOverride != "" {
		return *profileOverride
	}
	setting, err := s.settingRepo.GetByGroupAndKey(ctx, "isolate", "pppoe_profile")
	if err != nil || setting.Value == nil {
		return "isolate"
	}
	return *setting.Value
}

// Isolate changes subscription profile to the isolate profile on MikroTik
func (s *SubscriptionService) Isolate(ctx context.Context, id uuid.UUID, reason string) error {
	sub, err := s.subRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if sub.Status == "isolated" {
		return nil
	}

	var isolateProfileOverride *string
	planID, err := uuid.Parse(sub.PlanID)
	if err == nil {
		if profile, err := s.profileRepo.GetByID(ctx, planID); err == nil {
			isolateProfileOverride = profile.IsolateProfileName
		}
	}

	routerID, _ := uuid.Parse(sub.RouterID)
	mt, err := s.routerProvider.GetMikrotikAdapter(ctx, routerID)
	if err != nil {
		return fmt.Errorf("failed to connect to router: %w", err)
	}

	isolateProfile := s.getIsolateProfile(ctx, isolateProfileOverride)
	if err := s.applyProfile(ctx, mt, sub, isolateProfile); err != nil {
		return fmt.Errorf("failed to apply isolate profile: %w", err)
	}

	r := reason
	sub.SuspendReason = &r
	sub.Status = "isolated"
	if err := s.subRepo.Update(ctx, sub); err != nil {
		return err
	}
	s.invalidateSub(ctx, sub.ID, sub.RouterID, sub.Username)
	return nil
}

// applyProfile sets a new profile name on the PPP secret
func (s *SubscriptionService) applyProfile(ctx context.Context, mt MikrotikClientAdapter, sub *model.Subscription, profileName string) error {
	id, err := s.resolvePPPSecretID(ctx, mt, sub)
	if err != nil {
		return err
	}
	return mt.UpdateSecret(ctx, id, &mkdomain.PPPSecret{Profile: profileName})
}

// Restore reverts subscription from isolated back to its original bandwidth profile
func (s *SubscriptionService) Restore(ctx context.Context, id uuid.UUID) error {
	sub, err := s.subRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	planID, err := uuid.Parse(sub.PlanID)
	if err != nil {
		return fmt.Errorf("invalid plan ID: %w", err)
	}
	profile, err := s.profileRepo.GetByID(ctx, planID)
	if err != nil {
		return err
	}

	routerID, _ := uuid.Parse(sub.RouterID)
	mt, err := s.routerProvider.GetMikrotikAdapter(ctx, routerID)
	if err != nil {
		return fmt.Errorf("failed to connect to router: %w", err)
	}

	if err := s.applyProfile(ctx, mt, sub, profile.Name); err != nil {
		return fmt.Errorf("failed to restore profile: %w", err)
	}

	sub.Status = "active"
	sub.SuspendReason = nil
	if err := s.subRepo.Update(ctx, sub); err != nil {
		return err
	}
	s.invalidateSub(ctx, sub.ID, sub.RouterID, sub.Username)
	return nil
}

// Suspend disables the PPP secret on MikroTik and marks the subscription as suspended
func (s *SubscriptionService) Suspend(ctx context.Context, id uuid.UUID, reason string) error {
	sub, err := s.subRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	routerID, _ := uuid.Parse(sub.RouterID)
	mt, err := s.routerProvider.GetMikrotikAdapter(ctx, routerID)
	if err != nil {
		return fmt.Errorf("failed to connect to router: %w", err)
	}

	if err := s.disableInMikroTik(ctx, mt, sub); err != nil {
		return fmt.Errorf("failed to disable in mikrotik: %w", err)
	}

	r := reason
	sub.SuspendReason = &r
	sub.Status = "suspended"
	if err := s.subRepo.Update(ctx, sub); err != nil {
		return err
	}
	s.invalidateSub(ctx, sub.ID, sub.RouterID, sub.Username)
	return nil
}

// disableInMikroTik disables the PPP secret
func (s *SubscriptionService) disableInMikroTik(ctx context.Context, mt MikrotikClientAdapter, sub *model.Subscription) error {
	id, err := s.resolvePPPSecretID(ctx, mt, sub)
	if err != nil {
		return err
	}
	return mt.DisableSecret(ctx, id)
}

// enableInMikroTik enables the PPP secret
func (s *SubscriptionService) enableInMikroTik(ctx context.Context, mt MikrotikClientAdapter, sub *model.Subscription) error {
	id, err := s.resolvePPPSecretID(ctx, mt, sub)
	if err != nil {
		return err
	}
	return mt.EnableSecret(ctx, id)
}

// Terminate removes the subscription from MikroTik and marks it as terminated
func (s *SubscriptionService) Terminate(ctx context.Context, id uuid.UUID) error {
	sub, err := s.subRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	routerID, _ := uuid.Parse(sub.RouterID)
	mt, err := s.routerProvider.GetMikrotikAdapter(ctx, routerID)
	if err != nil {
		return fmt.Errorf("failed to connect to router: %w", err)
	}

	if err := s.removeFromMikroTik(ctx, mt, sub); err != nil {
		return fmt.Errorf("failed to remove from mikrotik: %w", err)
	}

	now := time.Now()
	sub.Status = "terminated"
	sub.TerminatedAt = &now
	if err := s.subRepo.Update(ctx, sub); err != nil {
		return err
	}
	s.invalidateSub(ctx, sub.ID, sub.RouterID, sub.Username)
	return nil
}

// removeFromMikroTik removes the PPP secret from MikroTik
func (s *SubscriptionService) removeFromMikroTik(ctx context.Context, mt MikrotikClientAdapter, sub *model.Subscription) error {
	id, err := s.resolvePPPSecretID(ctx, mt, sub)
	if err != nil {
		if errors.Is(err, ErrMikrotikPPPSecretNotFound) {
			return nil
		}
		return err
	}
	return mt.RemoveSecret(ctx, id)
}

// GetPPPSecret fetches the live PPP secret from MikroTik with a short cache.
// Returns nil if the router is unreachable — callers must handle gracefully.
func (s *SubscriptionService) GetPPPSecret(ctx context.Context, sub *model.Subscription) (*mkdomain.PPPSecret, error) {
	cacheKey := keyMtSecret(sub.RouterID, sub.Username)

	if s.cache != nil {
		if raw, err := s.cache.Get(ctx, cacheKey); err == nil {
			var secret mkdomain.PPPSecret
			if json.Unmarshal([]byte(raw), &secret) == nil {
				return &secret, nil
			}
		}
	}

	routerID, err := uuid.Parse(sub.RouterID)
	if err != nil {
		return nil, err
	}
	mt, err := s.routerProvider.GetMikrotikAdapter(ctx, routerID)
	if err != nil {
		return nil, err
	}
	secret, err := mt.GetSecretByName(ctx, sub.Username)
	if err != nil {
		return nil, err
	}

	if s.cache != nil {
		if b, err := json.Marshal(secret); err == nil {
			_ = s.cache.Set(ctx, cacheKey, b, ttlMtSecret)
		}
	}
	return secret, nil
}

// GetPPPSecretsBatch fetches live PPP secret data for a slice of subscriptions concurrently.
// Returns map[username]*PPPSecret. Missing entries mean router was unreachable for that item.
func (s *SubscriptionService) GetPPPSecrets(ctx context.Context, subs []model.Subscription) map[string]*mkdomain.PPPSecret {
	if len(subs) == 0 {
		return nil
	}
	type result struct {
		username string
		data     *mkdomain.PPPSecret
	}
	ch := make(chan result, len(subs))
	var wg sync.WaitGroup
	for i := range subs {
		wg.Add(1)
		go func(sub *model.Subscription) {
			defer wg.Done()
			secret, err := s.GetPPPSecret(ctx, sub)
			if err == nil {
				ch <- result{sub.Username, secret}
			}
		}(&subs[i])
	}
	wg.Wait()
	close(ch)
	m := make(map[string]*mkdomain.PPPSecret, len(subs))
	for r := range ch {
		m[r.username] = r.data
	}
	return m
}

// invalidateSub removes cached DB model and MikroTik data for a subscription.
func (s *SubscriptionService) invalidateSub(ctx context.Context, id, routerID, username string) {
	if s.cache != nil {
		_ = s.cache.Del(ctx, keySubscription(id), keyMtSecret(routerID, username))
	}
}
