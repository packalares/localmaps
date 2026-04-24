package auth

import "github.com/jmoiron/sqlx"

// TestDB returns the Manager's DB handle. Exported for _test files only;
// production code uses the higher-level Manager methods.
func TestDB(m *Manager) *sqlx.DB { return m.db }
