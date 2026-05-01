//go:build integration

package integration

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRBAC_Admin_CanAccessInvoices(t *testing.T) {
	suite := SetupSuite(t)
	defer suite.Cleanup(t)
	r := buildTestRouter(t, suite)

	email, password, _ := createAPIUser(t, suite, "admin")
	token := loginAs(t, r, email, password)

	w := makeRequest(t, r, http.MethodGet, "/api/v1/invoices", token, nil)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRBAC_Admin_CanAccessUsers(t *testing.T) {
	suite := SetupSuite(t)
	defer suite.Cleanup(t)
	r := buildTestRouter(t, suite)

	email, password, _ := createAPIUser(t, suite, "admin")
	token := loginAs(t, r, email, password)

	w := makeRequest(t, r, http.MethodGet, "/api/v1/users", token, nil)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRBAC_Admin_CanAccessRouters(t *testing.T) {
	suite := SetupSuite(t)
	defer suite.Cleanup(t)
	r := buildTestRouter(t, suite)

	email, password, _ := createAPIUser(t, suite, "admin")
	token := loginAs(t, r, email, password)

	w := makeRequest(t, r, http.MethodGet, "/api/v1/routers", token, nil)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRBAC_Staff_CanGetInvoices(t *testing.T) {
	suite := SetupSuite(t)
	defer suite.Cleanup(t)
	r := buildTestRouter(t, suite)

	email, password, _ := createAPIUser(t, suite, "technician")
	token := loginAs(t, r, email, password)

	w := makeRequest(t, r, http.MethodGet, "/api/v1/invoices", token, nil)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRBAC_Staff_CannotAccessUsers(t *testing.T) {
	suite := SetupSuite(t)
	defer suite.Cleanup(t)
	r := buildTestRouter(t, suite)

	email, password, _ := createAPIUser(t, suite, "technician")
	token := loginAs(t, r, email, password)

	w := makeRequest(t, r, http.MethodGet, "/api/v1/users", token, nil)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestRBAC_Staff_CannotAccessRouters(t *testing.T) {
	suite := SetupSuite(t)
	defer suite.Cleanup(t)
	r := buildTestRouter(t, suite)

	email, password, _ := createAPIUser(t, suite, "sales_agent")
	token := loginAs(t, r, email, password)

	w := makeRequest(t, r, http.MethodGet, "/api/v1/routers", token, nil)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestRBAC_NoToken_Returns401(t *testing.T) {
	suite := SetupSuite(t)
	defer suite.Cleanup(t)
	r := buildTestRouter(t, suite)

	w := makeRequest(t, r, http.MethodGet, "/api/v1/invoices", "", nil)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRBAC_Staff_CanConfirmPayment(t *testing.T) {
	suite := SetupSuite(t)
	defer suite.Cleanup(t)
	r := buildTestRouter(t, suite)

	email, password, _ := createAPIUser(t, suite, "sales_agent")
	token := loginAs(t, r, email, password)

	// sales_agent (staff) can POST to /api/v1/payments/* — even if the payment doesn't exist
	// we expect 404 (not found), not 403 (forbidden), proving RBAC allows the request through.
	w := makeRequest(t, r, http.MethodPost, "/api/v1/payments/00000000-0000-0000-0000-000000000001/confirm", token, nil)
	require.NotEqual(t, http.StatusForbidden, w.Code, "sales_agent staff should be allowed to POST payments/*")
}
