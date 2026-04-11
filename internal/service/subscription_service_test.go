package service

import (
	"context"
	"errors"
	"testing"

	mkdomain "github.com/Butterfly-Student/go-ros/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"mikmongo/internal/domain/subscription"
	"mikmongo/internal/model"
	"mikmongo/internal/service/mocks"
)

// testMikrotikProvider is a local MikrotikProvider for tests (avoids import cycle).
type testMikrotikProvider struct {
	adapter MikrotikClientAdapter
	err     error
}

func (p *testMikrotikProvider) GetMikrotikAdapter(_ context.Context, _ uuid.UUID) (MikrotikClientAdapter, error) {
	return p.adapter, p.err
}

// newSubServiceWithMocks wires a SubscriptionService with all dependencies mocked.
func newSubServiceWithMocks() (
	*SubscriptionService,
	*mocks.MockSubscriptionRepository,
	*mocks.MockBandwidthProfileRepository,
	*mocks.MockSystemSettingRepository,
	*mocks.MockMikrotikClientAdapter,
	*testMikrotikProvider,
) {
	subRepo := &mocks.MockSubscriptionRepository{}
	profileRepo := &mocks.MockBandwidthProfileRepository{}
	settingRepo := &mocks.MockSystemSettingRepository{}
	adapter := &mocks.MockMikrotikClientAdapter{}
	provider := &testMikrotikProvider{adapter: adapter}

	svc := NewSubscriptionService(
		subRepo, profileRepo, settingRepo,
		subscription.NewDomain(),
		provider,
		nil, // cache disabled in tests
	)
	return svc, subRepo, profileRepo, settingRepo, adapter, provider
}

// testIDs holds common IDs used across tests
type testIDs struct {
	routerID uuid.UUID
	planID   uuid.UUID
	subID    uuid.UUID
}

func newTestIDs() testIDs {
	return testIDs{
		routerID: uuid.New(),
		planID:   uuid.New(),
		subID:    uuid.New(),
	}
}

func TestCreate_Success(t *testing.T) {
	ctx := context.Background()
	svc, subRepo, profileRepo, _, adapter, _ := newSubServiceWithMocks()
	ids := newTestIDs()

	profile := &model.BandwidthProfile{
		ID:       ids.planID.String(),
		RouterID: ids.routerID.String(),
		Name:     "plan-10m",
	}
	sub := &model.Subscription{
		RouterID: ids.routerID.String(),
		PlanID:   ids.planID.String(),
		Username: "user1",
		Password: "pass1234",
	}

	profileRepo.On("GetByID", ctx, ids.planID).Return(profile, nil)
	adapter.On("GetSecretByName", ctx, "user1").Return(nil, nil).Once()
	adapter.On("AddSecret", ctx, mock.AnythingOfType("*domain.PPPSecret")).Return(nil).Once()
	adapter.On("GetSecretByName", ctx, "user1").Return(&mkdomain.PPPSecret{ID: "*1", Name: "user1"}, nil).Once()
	subRepo.On("Create", ctx, mock.AnythingOfType("*model.Subscription")).Return(nil)

	err := svc.Create(ctx, sub, nil)
	require.NoError(t, err)
	adapter.AssertCalled(t, "AddSecret", ctx, mock.AnythingOfType("*domain.PPPSecret"))
	subRepo.AssertCalled(t, "Create", ctx, mock.AnythingOfType("*model.Subscription"))
	assert.Equal(t, "*1", *sub.MtPPPID)
}

func TestCreate_RouterConnectionFails(t *testing.T) {
	ctx := context.Background()
	svc, subRepo, profileRepo, _, _, provider := newSubServiceWithMocks()
	ids := newTestIDs()

	provider.adapter = nil
	provider.err = errors.New("router unreachable")

	profile := &model.BandwidthProfile{
		ID:       ids.planID.String(),
		RouterID: ids.routerID.String(),
		Name:     "plan-10m",
	}
	sub := &model.Subscription{
		RouterID: ids.routerID.String(),
		PlanID:   ids.planID.String(),
		Username: "user1",
		Password: "pass1234",
	}
	profileRepo.On("GetByID", ctx, ids.planID).Return(profile, nil)

	err := svc.Create(ctx, sub, nil)
	assert.ErrorContains(t, err, "router unreachable")
	subRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestCreate_MikrotikAddSecretFails(t *testing.T) {
	ctx := context.Background()
	svc, subRepo, profileRepo, _, adapter, _ := newSubServiceWithMocks()
	ids := newTestIDs()

	profile := &model.BandwidthProfile{
		ID:       ids.planID.String(),
		RouterID: ids.routerID.String(),
		Name:     "plan-10m",
	}
	sub := &model.Subscription{
		RouterID: ids.routerID.String(),
		PlanID:   ids.planID.String(),
		Username: "user1",
		Password: "pass1234",
	}
	profileRepo.On("GetByID", ctx, ids.planID).Return(profile, nil)
	adapter.On("GetSecretByName", ctx, "user1").Return(nil, nil).Once()
	adapter.On("AddSecret", ctx, mock.AnythingOfType("*domain.PPPSecret")).Return(errors.New("ppp error")).Once()

	err := svc.Create(ctx, sub, nil)
	assert.ErrorContains(t, err, "ppp error")
	subRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestCreate_DBSaveFails_RollbackMikrotik(t *testing.T) {
	ctx := context.Background()
	svc, subRepo, profileRepo, _, adapter, _ := newSubServiceWithMocks()
	ids := newTestIDs()

	profile := &model.BandwidthProfile{
		ID:       ids.planID.String(),
		RouterID: ids.routerID.String(),
		Name:     "plan-10m",
	}
	sub := &model.Subscription{
		RouterID: ids.routerID.String(),
		PlanID:   ids.planID.String(),
		Username: "user1",
		Password: "pass1234",
	}

	profileRepo.On("GetByID", ctx, ids.planID).Return(profile, nil)
	adapter.On("GetSecretByName", ctx, "user1").Return(nil, nil).Once()
	adapter.On("AddSecret", ctx, mock.AnythingOfType("*domain.PPPSecret")).Return(nil).Once()
	adapter.On("GetSecretByName", ctx, "user1").Return(&mkdomain.PPPSecret{ID: "*1", Name: "user1"}, nil).Once()
	subRepo.On("Create", ctx, mock.AnythingOfType("*model.Subscription")).Return(errors.New("db error"))
	// rollback path: MtPPPID is now set to "*1", so RemoveSecret is called directly
	adapter.On("RemoveSecret", ctx, "*1").Return(nil)

	err := svc.Create(ctx, sub, nil)
	assert.ErrorContains(t, err, "db error")
	adapter.AssertCalled(t, "RemoveSecret", ctx, "*1")
}

func TestCreate_SecretAlreadyExistsOnRouter(t *testing.T) {
	ctx := context.Background()
	svc, subRepo, profileRepo, _, adapter, _ := newSubServiceWithMocks()
	ids := newTestIDs()

	profile := &model.BandwidthProfile{
		ID:       ids.planID.String(),
		RouterID: ids.routerID.String(),
		Name:     "plan-10m",
	}
	sub := &model.Subscription{
		RouterID: ids.routerID.String(),
		PlanID:   ids.planID.String(),
		Username: "user1",
		Password: "pass1234",
	}
	profileRepo.On("GetByID", ctx, ids.planID).Return(profile, nil)
	adapter.On("GetSecretByName", ctx, "user1").Return(&mkdomain.PPPSecret{ID: "*9", Name: "user1"}, nil).Once()

	err := svc.Create(ctx, sub, nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrMikrotikPPPSecretExists)
	adapter.AssertNotCalled(t, "AddSecret", mock.Anything, mock.Anything)
	subRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestActivate_Success(t *testing.T) {
	ctx := context.Background()
	svc, subRepo, _, _, adapter, _ := newSubServiceWithMocks()
	ids := newTestIDs()

	sub := &model.Subscription{
		ID:       ids.subID.String(),
		RouterID: ids.routerID.String(),
		Username: "user1",
		Status:   "pending",
	}

	subRepo.On("GetByID", ctx, ids.subID).Return(sub, nil)
	// MtPPPID is nil → getPPPID calls GetSecretByName
	adapter.On("GetSecretByName", ctx, "user1").Return(&mkdomain.PPPSecret{ID: "*1"}, nil)
	adapter.On("EnableSecret", ctx, "*1").Return(nil)
	subRepo.On("Update", ctx, mock.AnythingOfType("*model.Subscription")).Return(nil)

	err := svc.Activate(ctx, ids.subID)
	require.NoError(t, err)
	adapter.AssertCalled(t, "EnableSecret", ctx, "*1")

	// Verify status was set to active
	updatedSub := subRepo.Calls[1].Arguments[1].(*model.Subscription)
	assert.Equal(t, "active", updatedSub.Status)
	assert.NotNil(t, updatedSub.ActivatedAt)
}

func TestIsolate_Success(t *testing.T) {
	ctx := context.Background()
	svc, subRepo, profileRepo, settingRepo, adapter, _ := newSubServiceWithMocks()
	ids := newTestIDs()

	sub := &model.Subscription{
		ID:       ids.subID.String(),
		RouterID: ids.routerID.String(),
		PlanID:   ids.planID.String(),
		Username: "user1",
		Status:   "active",
	}
	profile := &model.BandwidthProfile{
		ID:                 ids.planID.String(),
		RouterID:           ids.routerID.String(),
		Name:               "plan-10m",
		IsolateProfileName: nil, // fall back to system setting
	}

	subRepo.On("GetByID", ctx, ids.subID).Return(sub, nil)
	profileRepo.On("GetByID", ctx, ids.planID).Return(profile, nil)
	settingRepo.On("GetByGroupAndKey", ctx, "isolate", "pppoe_profile").
		Return(nil, errors.New("not found"))
	// Isolate calls applyProfile → getPPPID → GetSecretByName (MtPPPID is nil)
	adapter.On("GetSecretByName", ctx, "user1").Return(&mkdomain.PPPSecret{ID: "*1"}, nil)
	adapter.On("UpdateSecret", ctx, "*1", mock.AnythingOfType("*domain.PPPSecret")).Return(nil)
	subRepo.On("Update", ctx, mock.AnythingOfType("*model.Subscription")).Return(nil)

	err := svc.Isolate(ctx, ids.subID, "overdue payment")
	require.NoError(t, err)
	adapter.AssertCalled(t, "UpdateSecret", ctx, "*1", mock.AnythingOfType("*domain.PPPSecret"))

	updatedSub := subRepo.Calls[1].Arguments[1].(*model.Subscription)
	assert.Equal(t, "isolated", updatedSub.Status)
}

func TestSuspend_Success(t *testing.T) {
	ctx := context.Background()
	svc, subRepo, _, _, adapter, _ := newSubServiceWithMocks()
	ids := newTestIDs()

	sub := &model.Subscription{
		ID:       ids.subID.String(),
		RouterID: ids.routerID.String(),
		Username: "user1",
		Status:   "active",
	}

	subRepo.On("GetByID", ctx, ids.subID).Return(sub, nil)
	adapter.On("GetSecretByName", ctx, "user1").Return(&mkdomain.PPPSecret{ID: "*1"}, nil)
	adapter.On("DisableSecret", ctx, "*1").Return(nil)
	subRepo.On("Update", ctx, mock.AnythingOfType("*model.Subscription")).Return(nil)

	err := svc.Suspend(ctx, ids.subID, "non-payment")
	require.NoError(t, err)
	adapter.AssertCalled(t, "DisableSecret", ctx, "*1")

	updatedSub := subRepo.Calls[1].Arguments[1].(*model.Subscription)
	assert.Equal(t, "suspended", updatedSub.Status)
}

func TestTerminate_Success(t *testing.T) {
	ctx := context.Background()
	svc, subRepo, _, _, adapter, _ := newSubServiceWithMocks()
	ids := newTestIDs()

	sub := &model.Subscription{
		ID:       ids.subID.String(),
		RouterID: ids.routerID.String(),
		Username: "user1",
		Status:   "active",
	}

	subRepo.On("GetByID", ctx, ids.subID).Return(sub, nil)
	adapter.On("GetSecretByName", ctx, "user1").Return(&mkdomain.PPPSecret{ID: "*1"}, nil)
	adapter.On("RemoveSecret", ctx, "*1").Return(nil)
	subRepo.On("Update", ctx, mock.AnythingOfType("*model.Subscription")).Return(nil)

	err := svc.Terminate(ctx, ids.subID)
	require.NoError(t, err)
	adapter.AssertCalled(t, "RemoveSecret", ctx, "*1")

	updatedSub := subRepo.Calls[1].Arguments[1].(*model.Subscription)
	assert.Equal(t, "terminated", updatedSub.Status)
	assert.NotNil(t, updatedSub.TerminatedAt)
}
