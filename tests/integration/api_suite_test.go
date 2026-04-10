//go:build integration

package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"mikmongo/internal/domain"
	"mikmongo/internal/handler"
	"mikmongo/internal/middleware"
	"mikmongo/internal/model"
	"mikmongo/internal/repository"
	"mikmongo/internal/repository/postgres"
	"mikmongo/internal/router"
	"mikmongo/internal/service"
	"mikmongo/pkg/jwt"
	casbinpkg "mikmongo/internal/casbin"
)

const testSecret = "test-secret-key-must-be-32chars!"

// buildTestRouterFull constructs a full Gin router and returns both the engine and handler registry.
// Callers can inject providers into the registry before using the engine.
func buildTestRouterFull(t *testing.T, suite *TestSuite) (*gin.Engine, *handler.Registry) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	jwtSvc := jwt.NewService(testSecret, 15*time.Minute, 7*24*time.Hour)
	repos := postgres.NewRepository(suite.DB)

	repoReg := &repository.Registry{
		CustomerRepo:             repos.CustomerRepo,
		InvoiceRepo:              repos.InvoiceRepo,
		PaymentRepo:              repos.PaymentRepo,
		RouterDeviceRepo:         repos.RouterDeviceRepo,
		UserRepo:                 repos.UserRepo,
		BandwidthProfileRepo:     repos.BandwidthProfileRepo,
		SubscriptionRepo:         repos.SubscriptionRepo,
		CustomerRegistrationRepo: repos.CustomerRegistrationRepo,
		InvoiceItemRepo:          repos.InvoiceItemRepo,
		PaymentAllocationRepo:    repos.PaymentAllocationRepo,
		SystemSettingRepo:        repos.SystemSettingRepo,
		SequenceCounterRepo:      repos.SequenceCounterRepo,
		MessageTemplateRepo:      repos.MessageTemplateRepo,
		AuditLogRepo:             repos.AuditLogRepo,
		Transactor:               repos.Transactor,
		HotspotSaleRepo:          repos.HotspotSaleRepo,
		SalesAgentRepo:           repos.SalesAgentRepo,
		AgentInvoiceRepo:         repos.AgentInvoiceRepo,
		CashEntryRepo:            repos.CashEntryRepo,
		PettyCashFundRepo:        repos.PettyCashFundRepo,
	}

	domainReg := domain.NewRegistry(
		domain.NewCustomerDomain(),
		domain.NewBillingDomain(),
		domain.NewPaymentDomain(),
		domain.NewRouterDomain(),
		domain.NewSubscriptionDomain(),
		domain.NewRegistrationDomain(),
		domain.NewNotificationDomain(),
	)

	svcReg := service.NewRegistry(repoReg, domainReg, jwtSvc, testSecret, suite.DB, suite.RedisClient, zap.NewNop(), nil)
	handlerReg := handler.NewRegistry(svcReg, repos.SystemSettingRepo, jwtSvc)

	// Wire HotspotSale + SalesAgent + AgentInvoice handlers.
	// VoucherGenerator is nil: HotspotSaleHandler only calls ListSales in tests (not GenerateBatch).
	hotspotSaleSvc := service.NewHotspotSaleService(nil, repos.HotspotSaleRepo, repos.SalesAgentRepo)
	handlerReg.HotspotSale = handler.NewHotspotSaleHandler(hotspotSaleSvc)
	handlerReg.SalesAgent = handler.NewSalesAgentHandler(repos.SalesAgentRepo, repos.SystemSettingRepo)
	handlerReg.AgentInvoice = handler.NewAgentInvoiceHandler(svcReg.AgentInvoice)
	handlerReg.AgentPortal = handler.NewAgentPortalHandler(repos.SalesAgentRepo, svcReg.AgentInvoice, hotspotSaleSvc, jwtSvc)
	handlerReg.CashManagement = handler.NewCashManagementHandler(svcReg.CashManagement)

	enforcer, err := casbinpkg.NewEnforcer(suite.DB)
	require.NoError(t, err)
	mwReg := middleware.NewRegistry(zap.NewNop(), jwtSvc, suite.RedisClient, enforcer, []string{"*"}, "")

	return router.New(handlerReg, mwReg), handlerReg
}

// buildTestRouter constructs a full Gin router using real dependencies scoped to suite.DB.
func buildTestRouter(t *testing.T, suite *TestSuite) *gin.Engine {
	t.Helper()
	r, _ := buildTestRouterFull(t, suite)
	return r
}

// makeRequest sends an HTTP request to the router and returns the recorder.
func makeRequest(t *testing.T, r *gin.Engine, method, path, token string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = json.Marshal(body)
		require.NoError(t, err)
	}

	req, err := http.NewRequest(method, path, bytes.NewBuffer(bodyBytes))
	require.NoError(t, err)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// loginAs POSTs to /api/v1/auth/login and returns the access_token.
func loginAs(t *testing.T, r *gin.Engine, email, password string) string {
	t.Helper()
	w := makeRequest(t, r, http.MethodPost, "/api/v1/auth/login", "", map[string]string{
		"email":    email,
		"password": password,
	})
	require.Equal(t, http.StatusOK, w.Code, "login failed: %s", w.Body.String())

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data, ok := resp["data"].(map[string]interface{})
	require.True(t, ok, "response.data is not an object")
	token, ok := data["access_token"].(string)
	require.True(t, ok, "access_token not found in response")
	return token
}

// loginAsWithRefresh POSTs to /api/v1/auth/login and returns both access and refresh tokens.
func loginAsWithRefresh(t *testing.T, r *gin.Engine, email, password string) (accessToken, refreshToken string) {
	t.Helper()
	w := makeRequest(t, r, http.MethodPost, "/api/v1/auth/login", "", map[string]string{
		"email":    email,
		"password": password,
	})
	require.Equal(t, http.StatusOK, w.Code, "login failed: %s", w.Body.String())

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data, ok := resp["data"].(map[string]interface{})
	require.True(t, ok, "response.data is not an object")
	accessToken, _ = data["access_token"].(string)
	refreshToken, _ = data["refresh_token"].(string)
	return accessToken, refreshToken
}

// createAPIUser inserts a user with a real bcrypt hash so authSvc.Login succeeds.
// Returns email, plaintext password, and user ID.
func createAPIUser(t *testing.T, suite *TestSuite, role string) (email, password, id string) {
	t.Helper()
	id = uuid.New().String()
	password = "Password123!"
	email = fmt.Sprintf("api-%s@test.com", id[:8])

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	require.NoError(t, err)

	u := &model.User{
		ID:           id,
		FullName:     "API Test User",
		Email:        email,
		PasswordHash: string(hash),
		BearerKey:    func() *string { s := uuid.New().String(); return &s }(), // must be unique per user
		Role:         role,
		IsActive:     true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	err = suite.DB.WithContext(suite.Ctx).Create(u).Error
	require.NoError(t, err)
	return email, password, id
}

// buildRootTestRouter builds a full Gin router using suite.RootDB (non-transactional connection pool).
// Use this for load/concurrent tests where the per-test transaction is not goroutine-safe.
func buildRootTestRouter(t *testing.T, suite *TestSuite) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	jwtSvc := jwt.NewService(testSecret, 15*time.Minute, 7*24*time.Hour)
	repos := postgres.NewRepository(suite.RootDB)

	repoReg := &repository.Registry{
		CustomerRepo:             repos.CustomerRepo,
		InvoiceRepo:              repos.InvoiceRepo,
		PaymentRepo:              repos.PaymentRepo,
		RouterDeviceRepo:         repos.RouterDeviceRepo,
		UserRepo:                 repos.UserRepo,
		BandwidthProfileRepo:     repos.BandwidthProfileRepo,
		SubscriptionRepo:         repos.SubscriptionRepo,
		CustomerRegistrationRepo: repos.CustomerRegistrationRepo,
		InvoiceItemRepo:          repos.InvoiceItemRepo,
		PaymentAllocationRepo:    repos.PaymentAllocationRepo,
		SystemSettingRepo:        repos.SystemSettingRepo,
		SequenceCounterRepo:      repos.SequenceCounterRepo,
		MessageTemplateRepo:      repos.MessageTemplateRepo,
		AuditLogRepo:             repos.AuditLogRepo,
		Transactor:               repos.Transactor,
		HotspotSaleRepo:          repos.HotspotSaleRepo,
		SalesAgentRepo:           repos.SalesAgentRepo,
		AgentInvoiceRepo:         repos.AgentInvoiceRepo,
		CashEntryRepo:            repos.CashEntryRepo,
		PettyCashFundRepo:        repos.PettyCashFundRepo,
	}

	domainReg := domain.NewRegistry(
		domain.NewCustomerDomain(),
		domain.NewBillingDomain(),
		domain.NewPaymentDomain(),
		domain.NewRouterDomain(),
		domain.NewSubscriptionDomain(),
		domain.NewRegistrationDomain(),
		domain.NewNotificationDomain(),
	)

	svcReg := service.NewRegistry(repoReg, domainReg, jwtSvc, testSecret, suite.RootDB, suite.RedisClient, zap.NewNop(), nil)
	handlerReg := handler.NewRegistry(svcReg, repos.SystemSettingRepo, jwtSvc)

	hotspotSaleSvc := service.NewHotspotSaleService(nil, repos.HotspotSaleRepo, repos.SalesAgentRepo)
	handlerReg.HotspotSale = handler.NewHotspotSaleHandler(hotspotSaleSvc)
	handlerReg.SalesAgent = handler.NewSalesAgentHandler(repos.SalesAgentRepo, repos.SystemSettingRepo)
	handlerReg.AgentInvoice = handler.NewAgentInvoiceHandler(svcReg.AgentInvoice)
	handlerReg.AgentPortal = handler.NewAgentPortalHandler(repos.SalesAgentRepo, svcReg.AgentInvoice, hotspotSaleSvc, jwtSvc)
	handlerReg.CashManagement = handler.NewCashManagementHandler(svcReg.CashManagement)

	enforcer, err := casbinpkg.NewEnforcer(suite.RootDB)
	require.NoError(t, err)
	mwReg := middleware.NewRegistry(zap.NewNop(), jwtSvc, suite.RedisClient, enforcer, []string{"*"}, "")

	return router.New(handlerReg, mwReg)
}

// createAPIUserRoot inserts a user into the root (committed) DB with a unique BearerKey.
// Registers t.Cleanup to hard-delete the user after the test.
func createAPIUserRoot(t *testing.T, suite *TestSuite, role string) (email, password, id string) {
	t.Helper()
	id = uuid.New().String()
	password = "Password123!"
	email = fmt.Sprintf("load-%s@test.com", id[:8])

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	require.NoError(t, err)

	u := &model.User{
		ID:           id,
		FullName:     "Load Test User",
		Email:        email,
		PasswordHash: string(hash),
		BearerKey:    func() *string { s := uuid.New().String(); return &s }(),
		Role:         role,
		IsActive:     true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	err = suite.RootDB.WithContext(suite.Ctx).Create(u).Error
	require.NoError(t, err)
	t.Cleanup(func() { suite.RootDB.Unscoped().Delete(u) })
	return email, password, id
}
