package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	mikrotik "github.com/Butterfly-Student/go-ros"
	mkdomain "github.com/Butterfly-Student/go-ros/domain"
	"github.com/google/uuid"

	"mikmongo/internal/model"
	"mikmongo/internal/repository"
)

// PPPProfileConfig holds MikroTik-only fields for PPP profiles (not stored in DB).
type PPPProfileConfig struct {
	LocalAddress   *string
	RemoteAddress  *string
	ParentQueue    *string
	QueueType      *string
	DNSServer      *string
	SessionTimeout *string
	IdleTimeout    *string
	// RateLimit overrides model.RateLimit and upload/download-derived limit when set.
	RateLimit *string
}

// BandwidthProfileService handles bandwidth profile business logic
type BandwidthProfileService struct {
	profileRepo repository.BandwidthProfileRepository
	routerSvc   *RouterService
	cache       CacheClient // nil → caching disabled (graceful degradation)
}

// NewBandwidthProfileService creates a new bandwidth profile service
func NewBandwidthProfileService(profileRepo repository.BandwidthProfileRepository, routerSvc *RouterService, cache CacheClient) *BandwidthProfileService {
	return &BandwidthProfileService{
		profileRepo: profileRepo,
		routerSvc:   routerSvc,
		cache:       cache,
	}
}

// Create creates a new bandwidth profile and syncs PPP profile to MikroTik.
// mtCfg carries optional MikroTik-only fields; pass nil to use defaults.
func (s *BandwidthProfileService) Create(ctx context.Context, profile *model.BandwidthProfile, mtCfg *PPPProfileConfig) error {
	if mtCfg == nil {
		mtCfg = &PPPProfileConfig{}
	}

	// Connect to MikroTik first
	routerID, err := uuid.Parse(profile.RouterID)
	if err != nil {
		return err
	}

	mt, err := s.routerSvc.GetMikrotikClient(ctx, routerID)
	if err != nil {
		return err
	}

	if err := s.addPPPProfileOnRouter(ctx, mt, profile, mtCfg); err != nil {
		return fmt.Errorf("failed to sync to mikrotik: %w", err)
	}

	// Save to database only if MikroTik succeeded
	if err := s.profileRepo.Create(ctx, profile); err != nil {
		// Rollback: remove from MikroTik
		existing, _ := mt.PPP.GetProfileByName(ctx, profile.Name)
		if existing != nil {
			_ = mt.PPP.RemoveProfile(ctx, existing.ID)
		}
		return err
	}

	s.invalidateBWProfile(ctx, profile.ID, profile.RouterID, profile.Name)
	return nil
}

func (s *BandwidthProfileService) pppRateLimit(profile *model.BandwidthProfile, mtCfg *PPPProfileConfig) string {
	if mtCfg != nil && mtCfg.RateLimit != nil && *mtCfg.RateLimit != "" {
		return *mtCfg.RateLimit
	}
	if profile.RateLimit != nil && *profile.RateLimit != "" {
		return *profile.RateLimit
	}
	return fmt.Sprintf("%dk/%dk", profile.UploadSpeed, profile.DownloadSpeed)
}

// buildPPPProfile builds the RouterOS PPP profile payload (name, rate-limit, local/remote-address, session-timeout, parent-queue, queue-type, dns, idle-timeout).
func (s *BandwidthProfileService) buildPPPProfile(profile *model.BandwidthProfile, mtCfg *PPPProfileConfig) *mkdomain.PPPProfile {
	if mtCfg == nil {
		mtCfg = &PPPProfileConfig{}
	}
	pppProfile := &mkdomain.PPPProfile{
		Name:      profile.Name,
		RateLimit: s.pppRateLimit(profile, mtCfg),
	}
	if mtCfg.LocalAddress != nil {
		pppProfile.LocalAddress = *mtCfg.LocalAddress
	}
	if mtCfg.RemoteAddress != nil {
		pppProfile.RemoteAddress = *mtCfg.RemoteAddress
	}
	if mtCfg.ParentQueue != nil {
		pppProfile.ParentQueue = *mtCfg.ParentQueue
	}
	if mtCfg.QueueType != nil {
		pppProfile.QueueType = *mtCfg.QueueType
	}
	if mtCfg.DNSServer != nil {
		pppProfile.DNSServer = *mtCfg.DNSServer
	}
	if mtCfg.SessionTimeout != nil {
		pppProfile.SessionTimeout = *mtCfg.SessionTimeout
	}
	if mtCfg.IdleTimeout != nil {
		pppProfile.IdleTimeout = *mtCfg.IdleTimeout
	}
	return pppProfile
}

// addPPPProfileOnRouter creates a PPP profile on the router. Fails if the name already exists (no silent update on create).
func (s *BandwidthProfileService) addPPPProfileOnRouter(ctx context.Context, mt *mikrotik.Client, profile *model.BandwidthProfile, mtCfg *PPPProfileConfig) error {
	existing, err := mt.PPP.GetProfileByName(ctx, profile.Name)
	if err != nil {
		return fmt.Errorf("mikrotik: lookup ppp profile: %w", err)
	}
	if existing != nil {
		return fmt.Errorf("%w: %q", ErrMikrotikPPPProfileExists, profile.Name)
	}
	return mt.PPP.AddProfile(ctx, s.buildPPPProfile(profile, mtCfg))
}

// updatePPPProfileOnRouter updates an existing PPP profile by RouterOS .id (resolved by previous DB name, then new name if renamed).
func (s *BandwidthProfileService) updatePPPProfileOnRouter(ctx context.Context, mt *mikrotik.Client, dbBefore *model.BandwidthProfile, dbAfter *model.BandwidthProfile, mtCfg *PPPProfileConfig) error {
	var existing *mkdomain.PPPProfile
	existing, err := mt.PPP.GetProfileByName(ctx, dbBefore.Name)
	if err != nil {
		return fmt.Errorf("mikrotik: lookup ppp profile: %w", err)
	}
	if existing == nil && dbBefore.Name != dbAfter.Name {
		existing, err = mt.PPP.GetProfileByName(ctx, dbAfter.Name)
		if err != nil {
			return fmt.Errorf("mikrotik: lookup ppp profile: %w", err)
		}
	}
	if existing == nil {
		return fmt.Errorf("%w: %q", ErrMikrotikPPPProfileNotFound, dbBefore.Name)
	}
	ppp := s.buildPPPProfile(dbAfter, mtCfg)
	return mt.PPP.UpdateProfile(ctx, existing.ID, ppp)
}

// rollbackPPPProfileAfterFailedDB reverts RouterOS state after a failed DB write. nameOnRouter is the profile name currently on the router (after a failed update, usually dbAfter.Name).
func (s *BandwidthProfileService) rollbackPPPProfileAfterFailedDB(ctx context.Context, mt *mikrotik.Client, nameOnRouter string, backTo *model.BandwidthProfile, mtCfg *PPPProfileConfig) error {
	existing, err := mt.PPP.GetProfileByName(ctx, nameOnRouter)
	if err != nil || existing == nil {
		return nil
	}
	ppp := s.buildPPPProfile(backTo, mtCfg)
	return mt.PPP.UpdateProfile(ctx, existing.ID, ppp)
}

// GetByID gets profile by ID with cache-aside.
func (s *BandwidthProfileService) GetByID(ctx context.Context, id uuid.UUID) (*model.BandwidthProfile, error) {
	if s.cache != nil {
		if raw, err := s.cache.Get(ctx, keyBWProfile(id.String())); err == nil {
			var m model.BandwidthProfile
			if json.Unmarshal([]byte(raw), &m) == nil {
				return &m, nil
			}
		}
	}

	m, err := s.profileRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if s.cache != nil {
		if b, err := json.Marshal(m); err == nil {
			_ = s.cache.Set(ctx, keyBWProfile(id.String()), b, ttlBWProfile)
		}
	}
	return m, nil
}

// GetByCode gets profile by code
func (s *BandwidthProfileService) GetByCode(ctx context.Context, code string) (*model.BandwidthProfile, error) {
	return s.profileRepo.GetByCode(ctx, code)
}

// GetByRouterAndName gets profile by router ID and name
func (s *BandwidthProfileService) GetByRouterAndName(ctx context.Context, routerID uuid.UUID, name string) (*model.BandwidthProfile, error) {
	return s.profileRepo.GetByRouterAndName(ctx, routerID, name)
}

// List lists profiles with pagination
func (s *BandwidthProfileService) List(ctx context.Context, limit, offset int) ([]model.BandwidthProfile, int64, error) {
	profiles, err := s.profileRepo.List(ctx, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	count, err := s.profileRepo.Count(ctx)
	return profiles, count, err
}

// ListActive lists active profiles
func (s *BandwidthProfileService) ListActive(ctx context.Context) ([]model.BandwidthProfile, error) {
	return s.profileRepo.ListActive(ctx)
}

// ListByRouterID lists profiles by router ID
func (s *BandwidthProfileService) ListByRouterID(ctx context.Context, routerID uuid.UUID, limit, offset int) ([]model.BandwidthProfile, int64, error) {
	profiles, err := s.profileRepo.ListByRouterID(ctx, routerID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	count, err := s.profileRepo.CountByRouterID(ctx, routerID)
	return profiles, count, err
}

// ListActiveByRouterID lists active profiles by router ID
func (s *BandwidthProfileService) ListActiveByRouterID(ctx context.Context, routerID uuid.UUID) ([]model.BandwidthProfile, error) {
	return s.profileRepo.ListActiveByRouterID(ctx, routerID)
}

// Update updates a profile and syncs PPP profile to MikroTik.
// mtCfg carries optional MikroTik-only fields; pass nil to use defaults.
func (s *BandwidthProfileService) Update(ctx context.Context, profile *model.BandwidthProfile, mtCfg *PPPProfileConfig) error {
	if mtCfg == nil {
		mtCfg = &PPPProfileConfig{}
	}

	// Get existing profile
	existingProfile, err := s.profileRepo.GetByID(ctx, uuid.MustParse(profile.ID))
	if err != nil {
		return err
	}

	// Connect to MikroTik
	routerID, err := uuid.Parse(profile.RouterID)
	if err != nil {
		return err
	}

	mt, err := s.routerSvc.GetMikrotikClient(ctx, routerID)
	if err != nil {
		return err
	}

	if err := s.updatePPPProfileOnRouter(ctx, mt, existingProfile, profile, mtCfg); err != nil {
		return fmt.Errorf("failed to sync to mikrotik: %w", err)
	}

	// Update database only if MikroTik succeeded
	if err := s.profileRepo.Update(ctx, profile); err != nil {
		_ = s.rollbackPPPProfileAfterFailedDB(ctx, mt, profile.Name, existingProfile, &PPPProfileConfig{})
		return err
	}

	s.invalidateBWProfile(ctx, profile.ID, profile.RouterID, profile.Name)
	return nil
}

// Delete soft-deletes a profile and removes PPP profile from MikroTik
func (s *BandwidthProfileService) Delete(ctx context.Context, id uuid.UUID) error {
	// Get profile before delete
	profile, err := s.profileRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	// Remove from MikroTik first
	routerID, err := uuid.Parse(profile.RouterID)
	if err != nil {
		return err
	}

	mt, err := s.routerSvc.GetMikrotikClient(ctx, routerID)
	if err != nil {
		return fmt.Errorf("failed to connect to router: %w", err)
	}

	// Remove PPP profile from MikroTik
	existing, _ := mt.PPP.GetProfileByName(ctx, profile.Name)
	if existing != nil {
		if err := mt.PPP.RemoveProfile(ctx, existing.ID); err != nil {
			return fmt.Errorf("failed to remove from mikrotik: %w", err)
		}
	}

	// Soft delete from database only if MikroTik succeeded
	if err := s.profileRepo.Delete(ctx, id); err != nil {
		return err
	}

	s.invalidateBWProfile(ctx, profile.ID, profile.RouterID, profile.Name)
	return nil
}

// GetPPPProfile fetches the live PPP profile from MikroTik with a short cache.
// Returns nil if the router is unreachable — callers must handle gracefully.
func (s *BandwidthProfileService) GetPPPProfile(ctx context.Context, profile *model.BandwidthProfile) (*mkdomain.PPPProfile, error) {
	cacheKey := keyMtProfile(profile.RouterID, profile.Name)

	if s.cache != nil {
		if raw, err := s.cache.Get(ctx, cacheKey); err == nil {
			var p mkdomain.PPPProfile
			if json.Unmarshal([]byte(raw), &p) == nil {
				return &p, nil
			}
		}
	}

	routerID, err := uuid.Parse(profile.RouterID)
	if err != nil {
		return nil, err
	}
	mt, err := s.routerSvc.GetMikrotikClient(ctx, routerID)
	if err != nil {
		return nil, err
	}
	p, err := mt.PPP.GetProfileByName(ctx, profile.Name)
	if err != nil {
		return nil, err
	}

	if s.cache != nil {
		if b, err := json.Marshal(p); err == nil {
			_ = s.cache.Set(ctx, cacheKey, b, ttlMtProfile)
		}
	}
	return p, nil
}

// GetPPPProfilesBatch fetches live PPP profile data for a slice of profiles concurrently.
// Returns map[profileName]*PPPProfile. Missing entries mean router was unreachable for that item.
func (s *BandwidthProfileService) GetPPPProfiles(ctx context.Context, profiles []model.BandwidthProfile) map[string]*mkdomain.PPPProfile {
	if len(profiles) == 0 {
		return nil
	}
	type result struct {
		name string
		data *mkdomain.PPPProfile
	}
	ch := make(chan result, len(profiles))
	var wg sync.WaitGroup
	for i := range profiles {
		wg.Add(1)
		go func(p *model.BandwidthProfile) {
			defer wg.Done()
			ppp, err := s.GetPPPProfile(ctx, p)
			if err == nil {
				ch <- result{p.Name, ppp}
			}
		}(&profiles[i])
	}
	wg.Wait()
	close(ch)
	m := make(map[string]*mkdomain.PPPProfile, len(profiles))
	for r := range ch {
		m[r.name] = r.data
	}
	return m
}

// invalidateBWProfile removes cached DB model and MikroTik data for a profile.
func (s *BandwidthProfileService) invalidateBWProfile(ctx context.Context, id, routerID, name string) {
	if s.cache != nil {
		_ = s.cache.Del(ctx, keyBWProfile(id), keyMtProfile(routerID, name))
	}
}
