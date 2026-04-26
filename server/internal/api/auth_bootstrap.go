package api

// auth_bootstrap.go — factories, first-boot admin creation, and router
// shim variables for the auth endpoints. Keeps auth.go focused on the
// pure HTTP-handler code path.

import (
	"os"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/jmoiron/sqlx"

	"github.com/packalares/localmaps/server/internal/auth"
	"github.com/packalares/localmaps/server/internal/config"
)

// BuildManager reads the cookie settings from the store and returns a
// session manager wired to the given DB. Default values match docs/07.
func BuildManager(store *config.Store, db *sqlx.DB) *auth.Manager {
	cfg := auth.CookieConfig{
		Name:       "localmaps_session",
		Secure:     true,
		TTLSeconds: 168 * 3600,
	}
	if store != nil {
		if s, err := store.GetString("auth.cookieName"); err == nil && s != "" {
			cfg.Name = s
		}
		if b, err := store.GetBool("auth.cookieSecure"); err == nil {
			cfg.Secure = b
		}
		if h, err := store.GetInt("auth.sessionTTLHours"); err == nil && h > 0 {
			cfg.TTLSeconds = h * 3600
		}
	}
	return auth.NewManager(db, cfg)
}

// BootstrapAdmin creates the initial admin if the users table is empty.
//
// First-run flow:
//   - LOCALMAPS_ADMIN_NAME / LOCALMAPS_ADMIN_PASSWORD env vars present
//     (set by the Olares install form): create the user with those
//     credentials, return ("", nil) so the caller doesn't log a password
//     the operator already knows.
//   - env vars absent (dev / docker-compose with no install form):
//     generate a random 18-char password, return it so the caller can
//     print it to stdout once.
//
// Later runs: users table is non-empty → no-op, returns ("", nil).
func BootstrapAdmin(m *auth.Manager) (string, error) {
	n, err := m.CountUsers()
	if err != nil {
		return "", err
	}
	if n > 0 {
		return "", nil
	}
	name := strings.TrimSpace(os.Getenv("LOCALMAPS_ADMIN_NAME"))
	if name == "" {
		name = "admin"
	}
	pw := os.Getenv("LOCALMAPS_ADMIN_PASSWORD")
	announce := false
	if pw == "" {
		var err error
		pw, err = auth.RandomPassword(18)
		if err != nil {
			return "", err
		}
		announce = true
	}
	if _, err := m.CreateUser(name, pw, auth.RoleAdmin); err != nil {
		return "", err
	}
	if announce {
		return pw, nil
	}
	return "", nil
}

// --- Registration shims wired by router.go -------------------------
var pkgAuth *authHandlers

func setAuthHandlers(h *authHandlers) { pkgAuth = h }

func authLoginHandler(c fiber.Ctx) error {
	if pkgAuth == nil {
		return notImplemented(c)
	}
	return pkgAuth.login(c)
}
func authLogoutHandler(c fiber.Ctx) error {
	if pkgAuth == nil {
		return notImplemented(c)
	}
	return pkgAuth.logout(c)
}
func authMeHandler(c fiber.Ctx) error {
	if pkgAuth == nil {
		return notImplemented(c)
	}
	return pkgAuth.me(c)
}
func authChangePasswordHandler(c fiber.Ctx) error {
	if pkgAuth == nil {
		return notImplemented(c)
	}
	return pkgAuth.changePassword(c)
}
func authListUsersHandler(c fiber.Ctx) error {
	if pkgAuth == nil {
		return notImplemented(c)
	}
	return pkgAuth.listUsers(c)
}
func authCreateUserHandler(c fiber.Ctx) error {
	if pkgAuth == nil {
		return notImplemented(c)
	}
	return pkgAuth.createUser(c)
}
func authDeleteUserHandler(c fiber.Ctx) error {
	if pkgAuth == nil {
		return notImplemented(c)
	}
	return pkgAuth.deleteUser(c)
}
