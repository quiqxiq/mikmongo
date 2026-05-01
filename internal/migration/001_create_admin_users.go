package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigrationContext(up001, down001)
}

// up001 creates the users table for ISP system administrators and operators.
func up001(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.Exec(`
	CREATE TABLE IF NOT EXISTS users (
		id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		full_name     VARCHAR(100) NOT NULL,
		email         VARCHAR(100) UNIQUE NOT NULL,
		phone         VARCHAR(20),
		password_hash VARCHAR(255) NOT NULL,
		role          VARCHAR(20)  NOT NULL
		              CHECK (role IN ('superadmin', 'admin', 'technician', 'sales_agent', 'customer'))
		              DEFAULT 'customer',
		is_active     BOOLEAN    DEFAULT true,
		last_login    TIMESTAMPTZ,
		last_ip       VARCHAR(45),
		bearer_key    VARCHAR(255) UNIQUE,
		created_at    TIMESTAMPTZ  DEFAULT CURRENT_TIMESTAMP NOT NULL,
		updated_at    TIMESTAMPTZ  DEFAULT CURRENT_TIMESTAMP NOT NULL,
		deleted_at    TIMESTAMPTZ
	);

	CREATE INDEX IF NOT EXISTS idx_users_email     ON users(email);
	CREATE INDEX IF NOT EXISTS idx_users_role      ON users(role);
	CREATE INDEX IF NOT EXISTS idx_users_is_active ON users(is_active);
	CREATE INDEX IF NOT EXISTS idx_users_deleted   ON users(deleted_at);

	COMMENT ON TABLE  users              IS 'Admin dan operator sistem ISP';
	COMMENT ON COLUMN users.password_hash IS 'bcrypt hash password';
	COMMENT ON COLUMN users.bearer_key    IS 'API key untuk akses programatik / integrasi eksternal';
	COMMENT ON COLUMN users.role          IS 'superadmin=full access, admin=manajemen, technician=lapangan, sales_agent=agen penjualan, customer=pelanggan';
	`)
	return err
}

func down001(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.Exec(`DROP TABLE IF EXISTS users CASCADE;`)
	return err
}
