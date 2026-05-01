package casbin

import (
	_ "embed"

	casbincore "github.com/casbin/casbin/v3"
	"github.com/casbin/casbin/v3/model"
	gormadapter "github.com/casbin/gorm-adapter/v3"
	"gorm.io/gorm"
)

//go:embed model.conf
var modelText string

// NewEnforcer creates a Casbin enforcer backed by the given *gorm.DB.
// It embeds model.conf and uses the existing casbin_rule table via gorm-adapter.
func NewEnforcer(db *gorm.DB) (*casbincore.Enforcer, error) {
	m, err := model.NewModelFromString(modelText)
	if err != nil {
		return nil, err
	}

	a, err := gormadapter.NewAdapterByDB(db)
	if err != nil {
		return nil, err
	}

	e, err := casbincore.NewEnforcer(m, a)
	if err != nil {
		return nil, err
	}

	seedDefaultPolicies(e)
	return e, nil
}

func seedDefaultPolicies(e *casbincore.Enforcer) {
	groupings := [][2]string{
		{"superadmin", "admin"},
		{"technician", "staff"},
		{"sales_agent", "staff"},
	}
	for _, g := range groupings {
		if has, _ := e.HasGroupingPolicy(g[0], g[1]); !has {
			_, _ = e.AddGroupingPolicy(g[0], g[1])
		}
	}

	policies := [][3]string{
		{"admin", "/api/v1/*", ".*"},
		{"staff", "/api/v1/auth/*", "GET|POST"},
		{"staff", "/api/v1/invoices", "GET|POST|PUT|DELETE"},
		{"staff", "/api/v1/invoices/*", "GET|POST|PUT|DELETE"},
		{"staff", "/api/v1/payments", "GET|POST|PUT"},
		{"staff", "/api/v1/payments/*", "GET|POST|PUT"},
		{"staff", "/api/v1/customers", "GET"},
		{"staff", "/api/v1/customers/*", "GET"},
		{"staff", "/api/v1/registrations", "GET|POST|PUT"},
		{"staff", "/api/v1/registrations/*", "GET|POST|PUT"},
		{"staff", "/api/v1/reports/*", "GET"},
	}
	for _, p := range policies {
		if has, _ := e.HasPolicy(p[0], p[1], p[2]); !has {
			_, _ = e.AddPolicy(p[0], p[1], p[2])
		}
	}
}
