package seeder

import (
	"context"
	"fmt"
)

// seedCasbin inserts default RBAC rules into casbin_rule via raw SQL.
// Mirrors seedDefaultPolicies() in internal/casbin/enforcer.go.
// Idempotent: each row is only inserted if not already present (WHERE NOT EXISTS).
func (s *Seeder) seedCasbin(ctx context.Context) error {
	// Role groupings: (ptype='g', v0=role, v1=parentRole)
	groupings := [][2]string{
		{"superadmin", "admin"},
		{"technician", "staff"},
		{"sales_agent", "staff"},
	}
	for _, g := range groupings {
		_, err := s.db.ExecContext(ctx, `
			INSERT INTO casbin_rule (ptype, v0, v1)
			SELECT 'g', $1::text, $2::text
			WHERE NOT EXISTS (
				SELECT 1 FROM casbin_rule WHERE ptype = 'g' AND v0 = $1::text AND v1 = $2::text
			)
		`, g[0], g[1])
		if err != nil {
			return fmt.Errorf("casbin grouping %v: %w", g, err)
		}
	}

	// Policies: (ptype='p', v0=role, v1=path, v2=action)
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
		_, err := s.db.ExecContext(ctx, `
			INSERT INTO casbin_rule (ptype, v0, v1, v2)
			SELECT 'p', $1::text, $2::text, $3::text
			WHERE NOT EXISTS (
				SELECT 1 FROM casbin_rule WHERE ptype = 'p' AND v0 = $1::text AND v1 = $2::text AND v2 = $3::text
			)
		`, p[0], p[1], p[2])
		if err != nil {
			return fmt.Errorf("casbin policy %v: %w", p, err)
		}
	}
	return nil
}
