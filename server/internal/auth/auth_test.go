package auth_test

import (
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"

	"github.com/packalares/localmaps/server/internal/auth"
)

// newMgr opens an in-memory SQLite, applies the two auth DDL blocks,
// and returns a Manager bound to it.
func newMgr(t *testing.T) *auth.Manager {
	t.Helper()
	raw, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	raw.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = raw.Close() })
	db := sqlx.NewDb(raw, "sqlite")

	_, err = db.Exec(`CREATE TABLE users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT NOT NULL UNIQUE,
		password_hash TEXT NOT NULL,
		role TEXT NOT NULL DEFAULT 'admin',
		created_at TEXT NOT NULL,
		last_login_at TEXT,
		disabled INTEGER NOT NULL DEFAULT 0
	)`)
	require.NoError(t, err)
	_, err = db.Exec(`CREATE TABLE sessions (
		id TEXT PRIMARY KEY,
		user_id INTEGER NOT NULL,
		created_at TEXT NOT NULL,
		expires_at TEXT NOT NULL,
		user_agent TEXT,
		ip TEXT
	)`)
	require.NoError(t, err)
	return auth.NewManager(db, auth.CookieConfig{
		Name:       "localmaps_session",
		Secure:     false,
		TTLSeconds: 3600,
	})
}

func TestHashAndVerifyPassword(t *testing.T) {
	h, err := auth.HashPassword("hunter22xx")
	require.NoError(t, err)
	require.NotEqual(t, "hunter22xx", h)
	require.NoError(t, auth.VerifyPassword(h, "hunter22xx"))
	require.Error(t, auth.VerifyPassword(h, "wrong"))
}

func TestCreateUser_AndLogin_HappyPath(t *testing.T) {
	m := newMgr(t)
	u, err := m.CreateUser("alice", "supersecret1", auth.RoleAdmin)
	require.NoError(t, err)
	require.Equal(t, "alice", u.Username)
	require.Equal(t, auth.RoleAdmin, u.Role)

	sid, logged, err := m.Login("alice", "supersecret1", "ua", "1.2.3.4")
	require.NoError(t, err)
	require.NotEmpty(t, sid)
	require.Equal(t, u.ID, logged.ID)

	id, err := m.LookupSession(sid)
	require.NoError(t, err)
	require.Equal(t, "alice", id.Username)
	require.Equal(t, auth.RoleAdmin, id.Role)
}

func TestLogin_WrongPassword_Rejects(t *testing.T) {
	m := newMgr(t)
	_, err := m.CreateUser("bob", "correct-horse-1", auth.RoleAdmin)
	require.NoError(t, err)

	_, _, err = m.Login("bob", "WRONG", "", "")
	require.ErrorIs(t, err, auth.ErrInvalidCredentials)
}

func TestLookupSession_Expired_Returns401(t *testing.T) {
	m := newMgr(t)
	_, err := m.CreateUser("eve", "passphrase-12", auth.RoleAdmin)
	require.NoError(t, err)
	sid, _, err := m.Login("eve", "passphrase-12", "", "")
	require.NoError(t, err)

	// Rewind expiry to the past via SQL.
	db := extractDB(t, m)
	past := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339Nano)
	_, err = db.Exec(`UPDATE sessions SET expires_at = ? WHERE id = ?`, past, sid)
	require.NoError(t, err)

	_, err = m.LookupSession(sid)
	require.ErrorIs(t, err, auth.ErrSessionExpired)
}

func TestLogout_DeletesSession(t *testing.T) {
	m := newMgr(t)
	_, err := m.CreateUser("carol", "s3cret-value!", auth.RoleAdmin)
	require.NoError(t, err)
	sid, _, err := m.Login("carol", "s3cret-value!", "", "")
	require.NoError(t, err)

	require.NoError(t, m.RevokeSession(sid))
	_, err = m.LookupSession(sid)
	require.ErrorIs(t, err, auth.ErrSessionExpired)
}

func TestRequire_NoCookie_Returns401(t *testing.T) {
	m := newMgr(t)
	app := fiber.New()
	app.Use(auth.WithManager(m))
	app.Get("/admin", func(c fiber.Ctx) error { return c.SendString("ok") },
		auth.Require(m))

	resp, err := app.Test(httptest.NewRequest("GET", "/admin", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var env map[string]any
	require.NoError(t, json.Unmarshal(body, &env))
	errObj := env["error"].(map[string]any)
	require.Equal(t, "UNAUTHORIZED", errObj["code"])
}

func TestRequire_WithCookie_AttachesIdentity(t *testing.T) {
	m := newMgr(t)
	_, err := m.CreateUser("dave", "passphrase-99", auth.RoleAdmin)
	require.NoError(t, err)
	sid, _, err := m.Login("dave", "passphrase-99", "", "")
	require.NoError(t, err)

	app := fiber.New()
	app.Use(auth.WithManager(m))
	app.Get("/admin", func(c fiber.Ctx) error {
		id := auth.FromCtx(c)
		require.NotNil(t, id)
		return c.JSON(id)
	}, auth.Require(m))

	req := httptest.NewRequest("GET", "/admin", nil)
	req.AddCookie(&http.Cookie{Name: "localmaps_session", Value: sid})
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	require.Contains(t, string(body), "dave")
}

func TestRequire_ViewerBlockedFromAdmin(t *testing.T) {
	m := newMgr(t)
	_, err := m.CreateUser("ivan", "passphrase-77", auth.RoleViewer)
	require.NoError(t, err)
	sid, _, err := m.Login("ivan", "passphrase-77", "", "")
	require.NoError(t, err)

	app := fiber.New()
	app.Use(auth.WithManager(m))
	app.Get("/admin", func(c fiber.Ctx) error { return c.SendString("ok") },
		auth.Require(m, auth.RoleAdmin))

	req := httptest.NewRequest("GET", "/admin", nil)
	req.AddCookie(&http.Cookie{Name: "localmaps_session", Value: sid})
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusForbidden, resp.StatusCode)
}

func TestOptional_NoCookie_NoIdentity(t *testing.T) {
	m := newMgr(t)
	app := fiber.New()
	app.Use(auth.WithManager(m))
	app.Use(auth.Optional(m))
	app.Get("/pub", func(c fiber.Ctx) error {
		if auth.FromCtx(c) != nil {
			return c.Status(500).SendString("unexpected identity")
		}
		return c.SendString("ok")
	})
	resp, err := app.Test(httptest.NewRequest("GET", "/pub", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestChangePassword_Flow(t *testing.T) {
	m := newMgr(t)
	u, err := m.CreateUser("pat", "old-password-1", auth.RoleAdmin)
	require.NoError(t, err)
	require.NoError(t, m.ChangePassword(u.ID, "old-password-1", "new-password-2"))
	require.Error(t, m.ChangePassword(u.ID, "WRONG", "x"))
	_, _, err = m.Login("pat", "new-password-2", "", "")
	require.NoError(t, err)
}

// --- helpers --------------------------------------------------------

// extractDB returns the Manager's inner DB for direct SQL in tests.
func extractDB(t *testing.T, m *auth.Manager) *sqlx.DB {
	t.Helper()
	db := auth.TestDB(m)
	require.NotNil(t, db)
	return db
}
