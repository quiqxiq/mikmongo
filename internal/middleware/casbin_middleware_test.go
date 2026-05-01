package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	casbincore "github.com/casbin/casbin/v3"
	"github.com/casbin/casbin/v3/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

const casbinModel = `
[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act

[role_definition]
g = _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = g(r.sub, p.sub) && keyMatch2(r.obj, p.obj) && regexMatch(r.act, p.act)
`

func newTestEnforcer(t *testing.T) *casbincore.Enforcer {
	t.Helper()
	m, err := model.NewModelFromString(casbinModel)
	if err != nil {
		t.Fatalf("failed to build model: %v", err)
	}
	e, err := casbincore.NewEnforcer(m)
	if err != nil {
		t.Fatalf("failed to create enforcer: %v", err)
	}

	_, _ = e.AddGroupingPolicy("superadmin", "admin")
	_, _ = e.AddGroupingPolicy("admin", "admin")
	_, _ = e.AddGroupingPolicy("technician", "staff")
	_, _ = e.AddGroupingPolicy("sales_agent", "staff")

	_, _ = e.AddPolicy("admin", "/api/v1/*", ".*")
	_, _ = e.AddPolicy("staff", "/api/v1/auth/*", "GET|POST")
	_, _ = e.AddPolicy("staff", "/api/v1/invoices", "GET|POST|PUT|DELETE")
	_, _ = e.AddPolicy("staff", "/api/v1/invoices/*", "GET|POST|PUT|DELETE")
	_, _ = e.AddPolicy("staff", "/api/v1/payments", "GET|POST|PUT")
	_, _ = e.AddPolicy("staff", "/api/v1/payments/*", "GET|POST|PUT")
	_, _ = e.AddPolicy("staff", "/api/v1/customers", "GET")
	_, _ = e.AddPolicy("staff", "/api/v1/customers/*", "GET")
	_, _ = e.AddPolicy("staff", "/api/v1/registrations", "GET|POST|PUT")
	_, _ = e.AddPolicy("staff", "/api/v1/registrations/*", "GET|POST|PUT")
	_, _ = e.AddPolicy("staff", "/api/v1/reports/*", "GET")

	return e
}

func buildCasbinRouter(e *casbincore.Enforcer, role string, setRole bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		if setRole {
			c.Set("role", role)
		}
		c.Next()
	})
	r.Use(CasbinMiddleware(e))
	r.Any("/*path", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	return r
}

func TestCasbin_AdminAllowed(t *testing.T) {
	e := newTestEnforcer(t)
	r := buildCasbinRouter(e, "admin", true)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/invoices", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCasbin_StaffAllowed_Invoice(t *testing.T) {
	e := newTestEnforcer(t)
	r := buildCasbinRouter(e, "technician", true)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/payments", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCasbin_StaffBlocked_Users(t *testing.T) {
	e := newTestEnforcer(t)
	r := buildCasbinRouter(e, "technician", true)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/users", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestCasbin_StaffBlocked_Routers(t *testing.T) {
	e := newTestEnforcer(t)
	r := buildCasbinRouter(e, "sales_agent", true)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/routers", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestCasbin_CustomerAllowed_Invoice(t *testing.T) {
	e := newTestEnforcer(t)
	r := buildCasbinRouter(e, "sales_agent", true)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/invoices", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCasbin_CustomerBlocked_Trigger(t *testing.T) {
	e := newTestEnforcer(t)
	r := buildCasbinRouter(e, "technician", true)

	// technician (staff) cannot DELETE customers — staff policy only allows GET on /api/v1/customers
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodDelete, "/api/v1/customers/some-id", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestCasbin_NoRole_Forbidden(t *testing.T) {
	e := newTestEnforcer(t)
	r := buildCasbinRouter(e, "", false)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/invoices", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}
