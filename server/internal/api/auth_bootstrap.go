package api

// auth_bootstrap.go — factories, first-boot admin creation, and router
// shim variables for the auth endpoints. Keeps auth.go focused on the
// pure HTTP-handler code path.

import (
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
// The generated password is returned so the caller can print it once.
func BootstrapAdmin(m *auth.Manager) (string, error) {
	n, err := m.CountUsers()
	if err != nil {
		return "", err
	}
	if n > 0 {
		return "", nil
	}
	pw, err := auth.RandomPassword(18)
	if err != nil {
		return "", err
	}
	if _, err := m.CreateUser("admin", pw, auth.RoleAdmin); err != nil {
		return "", err
	}
	return pw, nil
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
