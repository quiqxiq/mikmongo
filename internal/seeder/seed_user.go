package seeder

import (
	"context"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

func (s *Seeder) seedUsers(ctx context.Context) error {
	users := []struct {
		fullName string
		email    string
		phone    string
		password string
		role     string
	}{
		{"Super Admin", "superadmin@mikmongo.local", "+6280000000001", "admin123!", "superadmin"},
		{"Admin", "admin@mikmongo.local", "+6280000000002", "Admin123!", "admin"},
		{"Customer Service", "cs@mikmongo.local", "+6280000000003", "Cs123!", "cs"},
		{"Billing", "billing@mikmongo.local", "+6280000000004", "Billing123!", "billing"},
		{"Technician", "technician@mikmongo.local", "+6280000000005", "Tech123!", "technician"},
	}

	for _, u := range users {
		hash, err := bcrypt.GenerateFromPassword([]byte(u.password), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("bcrypt %s: %w", u.email, err)
		}
		_, err = s.db.ExecContext(ctx, `
			INSERT INTO users (full_name, email, phone, password_hash, role, is_active)
			VALUES ($1, $2, $3, $4, $5, true)
			ON CONFLICT (email) DO NOTHING
		`, u.fullName, u.email, u.phone, string(hash), u.role)
		if err != nil {
			return fmt.Errorf("insert user %s: %w", u.email, err)
		}
	}
	return nil
}
