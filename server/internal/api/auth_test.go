package api_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"

	"github.com/packalares/localmaps/server/internal/api"
	"github.com/packalares/localmaps/server/internal/auth"
	"github.com/packalares/localmaps/server/internal/config"
	"github.com/packalares/localmaps/server/internal/ratelimit"
	"github.com/packalares/localmaps/server/internal/telemetry"
	"github.com/packalares/localmaps/server/internal/ws"
)

// buildAuthApp spins up a Fiber app with a live session manager + one
// admin and one viewer user. The returned sessions are ready to attach
// to outgoing requests as cookies.
func buildAuthApp(t *testing.T) (*fiber.App, *auth.Manager, string, string) {
	t.Helper()
	store, err := config.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	tel := telemetry.New(io.Discard, "info")
	hub := ws.NewHub()
	t.Cleanup(hub.Close)

	mgr := api.BuildManager(store, store.DB())
	_, err = mgr.CreateUser("admin", "passphrase-11", auth.RoleAdmin)
	require.NoError(t, err)
	_, err = mgr.CreateUser("viewer", "passphrase-22", auth.RoleViewer)
	require.NoError(t, err)
	adminSID, _, err := mgr.Login("admin", "passphrase-11", "", "")
	require.NoError(t, err)
	viewerSID, _, err := mgr.Login("viewer", "passphrase-22", "", "")
	require.NoError(t, err)

	app := fiber.New()
	api.Register(app, api.Deps{
		Boot:      &config.Boot{},
		Store:     store,
		Telemetry: tel,
		Hub:       hub,
		Limiter:   ratelimit.New(store),
		Auth:      mgr,
	})
	return app, mgr, adminSID, viewerSID
}

// do sends a JSON request optionally attaching a session cookie.
func do(t *testing.T, app *fiber.App, sid, method, path string, body any) (*http.Response, map[string]any) {
	t.Helper()
	var raw []byte
	if body != nil {
		b, err := json.Marshal(body)
		require.NoError(t, err)
		raw = b
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(raw))
	if raw != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if sid != "" {
		req.AddCookie(&http.Cookie{Name: "localmaps_session", Value: sid})
	}
	resp, err := app.Test(req)
	require.NoError(t, err)
	rb, _ := io.ReadAll(resp.Body)
	var out map[string]any
	if len(rb) > 0 {
		_ = json.Unmarshal(rb, &out)
	}
	return resp, out
}

func TestAuth_Login_Happy(t *testing.T) {
	app, _, _, _ := buildAuthApp(t)
	resp, body := do(t, app, "", "POST", "/api/auth/login", map[string]any{
		"username": "admin", "password": "passphrase-11",
	})
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	user, ok := body["user"].(map[string]any)
	require.True(t, ok, "body: %v", body)
	require.Equal(t, "admin", user["username"])
	require.Equal(t, "admin", user["role"])

	// Cookie is set.
	var sid string
	for _, ck := range resp.Cookies() {
		if ck.Name == "localmaps_session" {
			sid = ck.Value
		}
	}
	require.NotEmpty(t, sid, "expected session cookie on login")
}

func TestAuth_Login_WrongPassword_401(t *testing.T) {
	app, _, _, _ := buildAuthApp(t)
	resp, body := do(t, app, "", "POST", "/api/auth/login", map[string]any{
		"username": "admin", "password": "WRONG",
	})
	require.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
	errObj := body["error"].(map[string]any)
	require.Equal(t, "UNAUTHORIZED", errObj["code"])
}

func TestAuth_Me_NoCookie_401(t *testing.T) {
	app, _, _, _ := buildAuthApp(t)
	resp, _ := do(t, app, "", "GET", "/api/auth/me", nil)
	require.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestAuth_Me_WithCookie_Returns200(t *testing.T) {
	app, _, adminSID, _ := buildAuthApp(t)
	resp, body := do(t, app, adminSID, "GET", "/api/auth/me", nil)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	user := body["user"].(map[string]any)
	require.Equal(t, "admin", user["username"])
}

func TestAuth_Logout_RevokesSession(t *testing.T) {
	app, _, adminSID, _ := buildAuthApp(t)
	resp, _ := do(t, app, adminSID, "POST", "/api/auth/logout", nil)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	// Same session must now fail /me.
	resp, _ = do(t, app, adminSID, "GET", "/api/auth/me", nil)
	require.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestAuth_ExpiredSession_Returns401(t *testing.T) {
	app, mgr, adminSID, _ := buildAuthApp(t)
	// Rewind expiry directly.
	past := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339Nano)
	_, err := auth.TestDB(mgr).Exec(`UPDATE sessions SET expires_at = ? WHERE id = ?`,
		past, adminSID)
	require.NoError(t, err)

	resp, _ := do(t, app, adminSID, "GET", "/api/auth/me", nil)
	require.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestAuth_ChangePassword_Flow(t *testing.T) {
	app, _, adminSID, _ := buildAuthApp(t)
	// Wrong current password → 401.
	resp, _ := do(t, app, adminSID, "POST", "/api/auth/change-password",
		map[string]any{"oldPassword": "nope", "newPassword": "new-passphrase-33"})
	require.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)

	// Happy: set new password.
	resp, _ = do(t, app, adminSID, "POST", "/api/auth/change-password",
		map[string]any{"oldPassword": "passphrase-11", "newPassword": "new-passphrase-33"})
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	// Login with old password must now fail.
	resp, _ = do(t, app, "", "POST", "/api/auth/login", map[string]any{
		"username": "admin", "password": "passphrase-11",
	})
	require.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestAuth_ListUsers_AdminOnly(t *testing.T) {
	app, _, adminSID, viewerSID := buildAuthApp(t)

	// Viewer → 403.
	resp, _ := do(t, app, viewerSID, "GET", "/api/auth/users", nil)
	require.Equal(t, fiber.StatusForbidden, resp.StatusCode)

	// Admin → 200 with both users visible.
	resp, body := do(t, app, adminSID, "GET", "/api/auth/users", nil)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	users := body["users"].([]any)
	require.Len(t, users, 2)
}

func TestAuth_CreateUser_AdminOnly(t *testing.T) {
	app, _, adminSID, viewerSID := buildAuthApp(t)
	// Viewer cannot create.
	resp, _ := do(t, app, viewerSID, "POST", "/api/auth/users",
		map[string]any{"username": "n", "password": "p-10-chars-ok", "role": "viewer"})
	require.Equal(t, fiber.StatusForbidden, resp.StatusCode)

	// Admin can.
	resp, body := do(t, app, adminSID, "POST", "/api/auth/users",
		map[string]any{"username": "new-user", "password": "p-10-chars-ok", "role": "viewer"})
	require.Equal(t, fiber.StatusCreated, resp.StatusCode)
	user := body["user"].(map[string]any)
	require.Equal(t, "new-user", user["username"])
}

func TestAuth_DeleteUser_AdminOnly(t *testing.T) {
	app, mgr, adminSID, viewerSID := buildAuthApp(t)
	u, err := mgr.CreateUser("tmp", "tmppass1234", auth.RoleViewer)
	require.NoError(t, err)

	// Viewer cannot delete.
	resp, _ := do(t, app, viewerSID, "DELETE",
		"/api/auth/users/"+itoa(u.ID), nil)
	require.Equal(t, fiber.StatusForbidden, resp.StatusCode)

	resp, _ = do(t, app, adminSID, "DELETE",
		"/api/auth/users/"+itoa(u.ID), nil)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
}

// itoa keeps the test body small and independent from strconv import spread.
func itoa(n int64) string {
	return fmtID(n)
}

func fmtID(n int64) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
