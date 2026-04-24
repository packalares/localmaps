package auth

import (
	"github.com/gofiber/fiber/v3"

	"github.com/packalares/localmaps/server/internal/apierr"
)

// managerKey is the fiber.Ctx.Locals key for the active Manager.
const managerKey = "authManager"

// WithManager stashes the Manager on every request so handlers can reach
// it via FromCtxManager. Install this BEFORE Optional/Require.
func WithManager(m *Manager) fiber.Handler {
	return func(c fiber.Ctx) error {
		c.Locals(managerKey, m)
		return c.Next()
	}
}

// FromCtxManager returns the Manager installed by WithManager, or nil.
func FromCtxManager(c fiber.Ctx) *Manager {
	if v := c.Locals(managerKey); v != nil {
		if m, ok := v.(*Manager); ok {
			return m
		}
	}
	return nil
}

// Optional reads the session cookie and attaches an Identity when the
// cookie resolves to a live session. Anonymous requests pass through.
func Optional(m *Manager) fiber.Handler {
	return func(c fiber.Ctx) error {
		if m != nil {
			if sid := c.Cookies(m.cookie.Name); sid != "" {
				if id, err := m.LookupSession(sid); err == nil {
					c.Locals(identityKey, id)
				}
			}
		}
		return c.Next()
	}
}

// Require enforces a live session and — when roles are provided — that
// the caller's role is in the set. Zero roles means "any authenticated
// user".
func Require(m *Manager, roles ...string) fiber.Handler {
	return func(c fiber.Ctx) error {
		if m == nil {
			return apierr.Write(c, apierr.CodeUnauthorized,
				"authentication not configured", false)
		}
		sid := c.Cookies(m.cookie.Name)
		id, err := m.LookupSession(sid)
		if err != nil {
			return apierr.Write(c, apierr.CodeUnauthorized,
				"authentication required", false)
		}
		if len(roles) > 0 && !hasRole(id, roles) {
			return apierr.Write(c, apierr.CodeForbidden,
				"insufficient privileges", false)
		}
		c.Locals(identityKey, id)
		return c.Next()
	}
}

// FromCtx returns the identity placed on the request by Require or
// Optional. Returns nil if none was set.
func FromCtx(c fiber.Ctx) *Identity {
	if v := c.Locals(identityKey); v != nil {
		if id, ok := v.(*Identity); ok {
			return id
		}
	}
	return nil
}

// SetSessionCookie writes the HttpOnly session cookie on the response.
func (m *Manager) SetSessionCookie(c fiber.Ctx, sid string) {
	c.Cookie(&fiber.Cookie{
		Name:     m.cookie.Name,
		Value:    sid,
		Path:     "/",
		HTTPOnly: true,
		Secure:   m.cookie.Secure,
		SameSite: "Lax",
		MaxAge:   m.cookie.TTLSeconds,
	})
}

// ClearSessionCookie sends a Max-Age=0 cookie to delete the browser copy.
func (m *Manager) ClearSessionCookie(c fiber.Ctx) {
	c.Cookie(&fiber.Cookie{
		Name:     m.cookie.Name,
		Value:    "",
		Path:     "/",
		HTTPOnly: true,
		Secure:   m.cookie.Secure,
		SameSite: "Lax",
		MaxAge:   -1,
	})
}

func hasRole(id *Identity, roles []string) bool {
	if id == nil {
		return false
	}
	for _, r := range roles {
		if id.Role == r {
			return true
		}
	}
	return false
}
