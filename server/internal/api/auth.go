package api

// Handlers for the `auth` tag in contracts/openapi.yaml.
//
// /api/auth/login            — public, rate-limited; sets session cookie
// /api/auth/logout           — revokes the current session
// /api/auth/me               — returns current user (401 anon)
// /api/auth/change-password  — auth required
// /api/auth/users            — admin only (list + create)
// /api/auth/users/{id}       — admin only (delete)

import (
	"encoding/json"
	"errors"
	"strconv"

	"github.com/gofiber/fiber/v3"

	"github.com/packalares/localmaps/server/internal/apierr"
	"github.com/packalares/localmaps/server/internal/auth"
	"github.com/packalares/localmaps/server/internal/config"
)

// authHandlers bundles the manager + config needed by the endpoints.
type authHandlers struct {
	mgr   *auth.Manager
	store *config.Store
}

func newAuthHandlers(m *auth.Manager, s *config.Store) *authHandlers {
	return &authHandlers{mgr: m, store: s}
}

// passwordMinLength reads auth.passwordMinLength or falls back to 10.
func (h *authHandlers) passwordMinLength() int {
	if h.store == nil {
		return 10
	}
	n, err := h.store.GetInt("auth.passwordMinLength")
	if err != nil || n < 4 {
		return 10
	}
	return n
}

// loginBody is the JSON shape of POST /api/auth/login.
type loginBody struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// POST /api/auth/login
func (h *authHandlers) login(c fiber.Ctx) error {
	var b loginBody
	if err := json.Unmarshal(c.Body(), &b); err != nil {
		return apierr.Write(c, apierr.CodeBadRequest, "invalid JSON body", false)
	}
	if b.Username == "" || b.Password == "" {
		return apierr.Write(c, apierr.CodeBadRequest,
			"username and password required", false)
	}
	sid, user, err := h.mgr.Login(b.Username, b.Password,
		string(c.Request().Header.UserAgent()), c.IP())
	if err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) ||
			errors.Is(err, auth.ErrUserDisabled) {
			return apierr.Write(c, apierr.CodeUnauthorized,
				"invalid credentials", false)
		}
		return apierr.Write(c, apierr.CodeInternal,
			"login failed", true)
	}
	h.mgr.SetSessionCookie(c, sid)
	return c.JSON(fiber.Map{"user": fiber.Map{
		"id":       user.ID,
		"username": user.Username,
		"role":     user.Role,
	}})
}

// POST /api/auth/logout
func (h *authHandlers) logout(c fiber.Ctx) error {
	sid := c.Cookies(h.mgr.Cookie().Name)
	if sid != "" {
		_ = h.mgr.RevokeSession(sid)
	}
	h.mgr.ClearSessionCookie(c)
	return c.JSON(fiber.Map{"ok": true})
}

// GET /api/auth/me
func (h *authHandlers) me(c fiber.Ctx) error {
	id := auth.FromCtx(c)
	if id == nil {
		return apierr.Write(c, apierr.CodeUnauthorized,
			"authentication required", false)
	}
	return c.JSON(fiber.Map{
		"user": fiber.Map{
			"id":       id.UserID,
			"username": id.Username,
			"role":     id.Role,
		},
	})
}

// changePasswordBody is the body for POST /api/auth/change-password.
type changePasswordBody struct {
	OldPassword string `json:"oldPassword"`
	NewPassword string `json:"newPassword"`
}

// POST /api/auth/change-password
func (h *authHandlers) changePassword(c fiber.Ctx) error {
	id := auth.FromCtx(c)
	if id == nil {
		return apierr.Write(c, apierr.CodeUnauthorized,
			"authentication required", false)
	}
	var b changePasswordBody
	if err := json.Unmarshal(c.Body(), &b); err != nil {
		return apierr.Write(c, apierr.CodeBadRequest, "invalid JSON body", false)
	}
	if len(b.NewPassword) < h.passwordMinLength() {
		return apierr.Write(c, apierr.CodeBadRequest,
			"new password is too short", false)
	}
	if err := h.mgr.ChangePassword(id.UserID, b.OldPassword, b.NewPassword); err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) {
			return apierr.Write(c, apierr.CodeUnauthorized,
				"current password incorrect", false)
		}
		return apierr.Write(c, apierr.CodeInternal,
			"failed to change password", true)
	}
	return c.JSON(fiber.Map{"ok": true})
}

// newUserBody is the body for POST /api/auth/users.
type newUserBody struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

// GET /api/auth/users (admin)
func (h *authHandlers) listUsers(c fiber.Ctx) error {
	users, err := h.mgr.ListUsers()
	if err != nil {
		return apierr.Write(c, apierr.CodeInternal,
			"failed to list users", true)
	}
	out := make([]fiber.Map, 0, len(users))
	for _, u := range users {
		out = append(out, fiber.Map{
			"id":          u.ID,
			"username":    u.Username,
			"role":        u.Role,
			"createdAt":   u.CreatedAt,
			"lastLoginAt": u.LastLoginAt.String,
			"disabled":    u.Disabled,
		})
	}
	return c.JSON(fiber.Map{"users": out})
}

// POST /api/auth/users (admin)
func (h *authHandlers) createUser(c fiber.Ctx) error {
	var b newUserBody
	if err := json.Unmarshal(c.Body(), &b); err != nil {
		return apierr.Write(c, apierr.CodeBadRequest, "invalid JSON body", false)
	}
	if b.Username == "" {
		return apierr.Write(c, apierr.CodeBadRequest, "username required", false)
	}
	if len(b.Password) < h.passwordMinLength() {
		return apierr.Write(c, apierr.CodeBadRequest, "password too short", false)
	}
	role := b.Role
	if role == "" {
		role = auth.RoleAdmin
	}
	u, err := h.mgr.CreateUser(b.Username, b.Password, role)
	if err != nil {
		return apierr.Write(c, apierr.CodeBadRequest, err.Error(), false)
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"user": fiber.Map{
			"id":       u.ID,
			"username": u.Username,
			"role":     u.Role,
		},
	})
}

// DELETE /api/auth/users/{id} (admin)
func (h *authHandlers) deleteUser(c fiber.Ctx) error {
	idStr := c.Params("id")
	n, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return apierr.Write(c, apierr.CodeBadRequest, "invalid user id", false)
	}
	if err := h.mgr.DeleteUser(n); err != nil {
		return apierr.Write(c, apierr.CodeInternal, "failed to delete user", true)
	}
	return c.JSON(fiber.Map{"ok": true})
}


