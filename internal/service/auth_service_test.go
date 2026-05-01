package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	"mikmongo/internal/model"
	"mikmongo/internal/service/mocks"
	"mikmongo/pkg/jwt"
)

func newTestJWTService() *jwt.Service {
	return jwt.NewService("test-secret-key-32-chars-long!!!", 15*time.Minute, 7*24*time.Hour)
}

func newAuthServiceWithMocks() (
	*AuthService,
	*mocks.MockUserRepository,
	*mocks.MockRedisClient,
) {
	userRepo := &mocks.MockUserRepository{}
	redisClient := &mocks.MockRedisClient{}
	svc := NewAuthService(userRepo, newTestJWTService(), redisClient)
	return svc, userRepo, redisClient
}

func hashPassword(password string) string {
	h, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	return string(h)
}

func TestLogin_Success(t *testing.T) {
	ctx := context.Background()
	svc, userRepo, redisClient := newAuthServiceWithMocks()

	userID := uuid.New()
	user := &model.User{
		ID:           userID.String(),
		Email:        "admin@test.com",
		PasswordHash: hashPassword("password123"),
		Role:         "admin",
		IsActive:     true,
	}

	userRepo.On("GetByEmail", ctx, "admin@test.com").Return(user, nil)
	userRepo.On("UpdateLastLogin", ctx, userID, "", mock.AnythingOfType("time.Time")).Return(nil)
	redisClient.On("BlacklistToken", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	resp, err := svc.Login(ctx, "admin@test.com", "password123")
	require.NoError(t, err)
	assert.NotEmpty(t, resp.AccessToken)
	assert.NotEmpty(t, resp.RefreshToken)
	assert.Equal(t, userID.String(), resp.User.ID)
	assert.Empty(t, resp.User.PasswordHash, "password hash should be cleared")
}

func TestLogin_InvalidPassword(t *testing.T) {
	ctx := context.Background()
	svc, userRepo, _ := newAuthServiceWithMocks()

	user := &model.User{
		ID:           uuid.New().String(),
		Email:        "admin@test.com",
		PasswordHash: hashPassword("correct-password"),
		IsActive:     true,
	}

	userRepo.On("GetByEmail", ctx, "admin@test.com").Return(user, nil)

	resp, err := svc.Login(ctx, "admin@test.com", "wrong-password")
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "invalid credentials")
}

func TestLogin_UserNotFound(t *testing.T) {
	ctx := context.Background()
	svc, userRepo, _ := newAuthServiceWithMocks()

	userRepo.On("GetByEmail", ctx, "noone@test.com").Return(nil, assert.AnError)

	resp, err := svc.Login(ctx, "noone@test.com", "password")
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "invalid credentials")
}

func TestLogin_InactiveUser(t *testing.T) {
	ctx := context.Background()
	svc, userRepo, _ := newAuthServiceWithMocks()

	user := &model.User{
		ID:           uuid.New().String(),
		Email:        "inactive@test.com",
		PasswordHash: hashPassword("password123"),
		IsActive:     false,
	}

	userRepo.On("GetByEmail", ctx, "inactive@test.com").Return(user, nil)

	resp, err := svc.Login(ctx, "inactive@test.com", "password123")
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "inactive")
}

func TestRefreshToken_Valid(t *testing.T) {
	ctx := context.Background()
	svc, userRepo, redisClient := newAuthServiceWithMocks()

	userID := uuid.New()
	user := &model.User{
		ID:       userID.String(),
		Email:    "admin@test.com",
		Role:     "admin",
		IsActive: true,
	}

	// Generate a real refresh token
	jwtSvc := newTestJWTService()
	_, refreshToken, err := jwtSvc.GenerateTokenPair(userID.String(), "admin@test.com", "admin")
	require.NoError(t, err)

	redisClient.On("IsBlacklisted", ctx, mock.Anything).Return(false, nil)
	redisClient.On("BlacklistToken", ctx, mock.Anything, mock.AnythingOfType("time.Duration")).Return(nil)
	userRepo.On("GetByID", ctx, userID).Return(user, nil)

	resp, err := svc.RefreshToken(ctx, refreshToken)
	require.NoError(t, err)
	assert.NotEmpty(t, resp.AccessToken)
	assert.NotEmpty(t, resp.RefreshToken)
}

func TestRefreshToken_Blacklisted(t *testing.T) {
	ctx := context.Background()
	svc, _, redisClient := newAuthServiceWithMocks()

	// Generate a real refresh token
	jwtSvc := newTestJWTService()
	userID := uuid.New()
	_, refreshToken, err := jwtSvc.GenerateTokenPair(userID.String(), "admin@test.com", "admin")
	require.NoError(t, err)

	redisClient.On("IsBlacklisted", ctx, mock.Anything).Return(true, nil)

	resp, err := svc.RefreshToken(ctx, refreshToken)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "revoked")
}

func TestRefreshToken_InvalidToken(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newAuthServiceWithMocks()

	resp, err := svc.RefreshToken(ctx, "invalid.token.string")
	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestChangePassword_Success(t *testing.T) {
	ctx := context.Background()
	svc, userRepo, redisClient := newAuthServiceWithMocks()

	userID := uuid.New()
	user := &model.User{
		ID:           userID.String(),
		PasswordHash: hashPassword("old-password"),
		IsActive:     true,
	}

	userRepo.On("GetByID", ctx, userID).Return(user, nil)
	userRepo.On("Update", ctx, mock.AnythingOfType("*model.User")).Return(nil)
	redisClient.On("SetPasswordChangedAt", ctx, userID.String(), mock.AnythingOfType("time.Time"), mock.AnythingOfType("time.Duration")).Return(nil)

	err := svc.ChangePassword(ctx, userID, "old-password", "new-password-123")
	require.NoError(t, err)

	// Password hash should have changed
	assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte("new-password-123")))
}

func TestChangePassword_WrongOldPassword(t *testing.T) {
	ctx := context.Background()
	svc, userRepo, _ := newAuthServiceWithMocks()

	userID := uuid.New()
	user := &model.User{
		ID:           userID.String(),
		PasswordHash: hashPassword("correct-password"),
	}

	userRepo.On("GetByID", ctx, userID).Return(user, nil)

	err := svc.ChangePassword(ctx, userID, "wrong-password", "new-password-123")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid current password")
}

func TestChangePassword_UserNotFound(t *testing.T) {
	ctx := context.Background()
	svc, userRepo, _ := newAuthServiceWithMocks()

	userID := uuid.New()
	userRepo.On("GetByID", ctx, userID).Return(nil, assert.AnError)

	err := svc.ChangePassword(ctx, userID, "password", "new-password")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "user not found")
}

func TestLogout_CallsBlacklistWithCorrectJTI(t *testing.T) {
	ctx := context.Background()
	svc, _, redisClient := newAuthServiceWithMocks()

	jti := "specific-jti-abc123"
	ttl := 15 * time.Minute
	redisClient.On("BlacklistToken", ctx, jti, ttl).Return(nil).Once()

	err := svc.Logout(ctx, jti, ttl)
	require.NoError(t, err)
	redisClient.AssertCalled(t, "BlacklistToken", ctx, jti, ttl)
}

func TestLogout_PropagatesRedisError(t *testing.T) {
	ctx := context.Background()
	svc, _, redisClient := newAuthServiceWithMocks()

	redisClient.On("BlacklistToken", ctx, "some-jti", 15*time.Minute).
		Return(errors.New("redis down"))

	err := svc.Logout(ctx, "some-jti", 15*time.Minute)
	assert.ErrorContains(t, err, "redis down")
}

func TestRefreshToken_BlacklistsOldTokenJTI(t *testing.T) {
	ctx := context.Background()
	svc, userRepo, redisClient := newAuthServiceWithMocks()

	userID := uuid.New()
	jwtSvc := newTestJWTService()
	_, refreshToken, err := jwtSvc.GenerateTokenPair(userID.String(), "admin@test.com", "admin")
	require.NoError(t, err)

	// Parse the token to extract the JTI
	claims, err := jwtSvc.Validate(refreshToken)
	require.NoError(t, err)
	oldJTI := claims.ID

	user := &model.User{
		ID:       userID.String(),
		Email:    "admin@test.com",
		Role:     "admin",
		IsActive: true,
	}

	redisClient.On("IsBlacklisted", ctx, oldJTI).Return(false, nil)
	// The old refresh token JTI should be blacklisted during refresh
	redisClient.On("BlacklistToken", ctx, oldJTI, mock.AnythingOfType("time.Duration")).Return(nil)
	userRepo.On("GetByID", ctx, userID).Return(user, nil)

	resp, err := svc.RefreshToken(ctx, refreshToken)
	require.NoError(t, err)
	assert.NotEmpty(t, resp.AccessToken)

	redisClient.AssertCalled(t, "BlacklistToken", ctx, oldJTI, mock.AnythingOfType("time.Duration"))
}

func TestCreateUser_NilBearerKey(t *testing.T) {
	ctx := context.Background()
	svc, userRepo, _ := newAuthServiceWithMocks()

	user := &model.User{
		FullName: "Test User",
		Email:    "test@example.com",
		Role:     "technician",
		IsActive: true,
	}

	userRepo.On("Create", ctx, mock.AnythingOfType("*model.User")).Return(nil)

	err := svc.CreateUser(ctx, user, "password123")
	require.NoError(t, err)
	// BearerKey should be nil (zero value for *string), not an empty string
	assert.Nil(t, user.BearerKey, "BearerKey must be nil to avoid unique constraint violation on empty string")
}

func TestGetUser_ClearsSensitiveFields(t *testing.T) {
	ctx := context.Background()
	svc, userRepo, _ := newAuthServiceWithMocks()

	userID := uuid.New()
	bearerKey := "some-bearer-key"
	user := &model.User{
		ID:           userID.String(),
		Email:        "admin@test.com",
		PasswordHash: hashPassword("password123"),
		BearerKey:    &bearerKey,
		Role:         "admin",
		IsActive:     true,
	}

	userRepo.On("GetByID", ctx, userID).Return(user, nil)

	result, err := svc.GetUser(ctx, userID)
	require.NoError(t, err)
	assert.Empty(t, result.PasswordHash, "PasswordHash should be cleared")
	assert.Nil(t, result.BearerKey, "BearerKey should be nil after GetUser")
}
