package api_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"

	"github.com/packalares/localmaps/server/internal/api"
	"github.com/packalares/localmaps/server/internal/auth"
	"github.com/packalares/localmaps/server/internal/config"
	"github.com/packalares/localmaps/server/internal/ratelimit"
	"github.com/packalares/localmaps/server/internal/telemetry"
	"github.com/packalares/localmaps/server/internal/ws"
)

// buildSettingsApp builds a Fiber app wired with a real (in-memory)
// config store. It returns the app, the store, and a freshly-minted
// session cookie for an admin user so tests can assert admin-only
// behaviour.
func buildSettingsApp(t *testing.T) (*fiber.App, *config.Store, string) {
	t.Helper()
	store, err := config.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	tel := telemetry.New(io.Discard, "info")
	hub := ws.NewHub()
	t.Cleanup(hub.Close)

	// Build a manager + seed an admin user + create a live session.
	mgr := api.BuildManager(store, store.DB())
	_, err = mgr.CreateUser("alice", "passphrase-11", auth.RoleAdmin)
	require.NoError(t, err)
	sid, _, err := mgr.Login("alice", "passphrase-11", "", "")
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
	return app, store, sid
}

// doAdmin sends an authenticated admin request and returns the fiber
// response. The helper attaches the session cookie so auth.Require is happy.
func doAdmin(t *testing.T, app *fiber.App, sid, method, path string, body any) (int, map[string]any) {
	t.Helper()
	var raw []byte
	if body != nil {
		b, err := json.Marshal(body)
		require.NoError(t, err)
		raw = b
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(raw))
	req.AddCookie(&http.Cookie{Name: "localmaps_session", Value: sid})
	if raw != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	rb, _ := io.ReadAll(resp.Body)
	var out map[string]any
	if len(rb) > 0 {
		_ = json.Unmarshal(rb, &out)
	}
	return resp.StatusCode, out
}

func TestSettings_Schema_ReturnsNodeList(t *testing.T) {
	app, _, _ := buildSettingsApp(t)
	resp, err := app.Test(httptest.NewRequest("GET", "/api/settings/schema", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	b, _ := io.ReadAll(resp.Body)
	var body map[string]any
	require.NoError(t, json.Unmarshal(b, &body))
	nodes, ok := body["nodes"].([]any)
	require.True(t, ok)
	require.Greater(t, len(nodes), 20, "expected many schema nodes")

	// Pick one and assert shape.
	var mapStyle map[string]any
	for _, n := range nodes {
		if m := n.(map[string]any); m["key"] == "map.style" {
			mapStyle = m
			break
		}
	}
	require.NotNil(t, mapStyle)
	require.Equal(t, "enum", mapStyle["type"])
	require.ElementsMatch(t, []any{"light", "dark", "auto"}, mapStyle["enumValues"])
	require.Equal(t, "map", mapStyle["uiGroup"])
}

func TestSettings_Get_ReturnsTree(t *testing.T) {
	app, _, sid := buildSettingsApp(t)
	status, body := doAdmin(t, app, sid, "GET", "/api/settings", nil)
	require.Equal(t, fiber.StatusOK, status)

	mapSec, ok := body["map"].(map[string]any)
	require.True(t, ok, "expected map group, got %v", body)
	require.Equal(t, "light", mapSec["style"])

	_, hasSchemaVersion := body["schema"]
	require.False(t, hasSchemaVersion, "schema.version must not be exposed")
}

func TestSettings_Patch_Valid(t *testing.T) {
	app, store, sid := buildSettingsApp(t)
	status, body := doAdmin(t, app, sid, "PATCH", "/api/settings",
		map[string]any{"map.style": "dark", "search.resultLimit": 25})
	require.Equal(t, fiber.StatusOK, status)

	mapSec := body["map"].(map[string]any)
	require.Equal(t, "dark", mapSec["style"])

	// Persisted to the store.
	s, err := store.GetString("map.style")
	require.NoError(t, err)
	require.Equal(t, "dark", s)
	n, err := store.GetInt("search.resultLimit")
	require.NoError(t, err)
	require.Equal(t, 25, n)
}

func TestSettings_Patch_AcceptsNestedTree(t *testing.T) {
	app, store, sid := buildSettingsApp(t)
	status, _ := doAdmin(t, app, sid, "PATCH", "/api/settings", map[string]any{
		"routing": map[string]any{"avoidTolls": true},
	})
	require.Equal(t, fiber.StatusOK, status)
	b, err := store.GetBool("routing.avoidTolls")
	require.NoError(t, err)
	require.True(t, b)
}

func TestSettings_Patch_RejectsUnknownKey(t *testing.T) {
	app, _, sid := buildSettingsApp(t)
	status, body := doAdmin(t, app, sid, "PATCH", "/api/settings",
		map[string]any{"map.nonExistent": "x"})
	require.Equal(t, fiber.StatusBadRequest, status)
	require.Contains(t, body["error"].(map[string]any)["message"],
		"unknown setting")
}

func TestSettings_Patch_RejectsOutOfRange(t *testing.T) {
	app, _, sid := buildSettingsApp(t)
	status, body := doAdmin(t, app, sid, "PATCH", "/api/settings",
		map[string]any{"map.maxZoom": 99})
	require.Equal(t, fiber.StatusBadRequest, status)
	msg := body["error"].(map[string]any)["message"].(string)
	require.Contains(t, msg, "map.maxZoom")
}

func TestSettings_Patch_RejectsTypeMismatch(t *testing.T) {
	app, _, sid := buildSettingsApp(t)
	status, _ := doAdmin(t, app, sid, "PATCH", "/api/settings",
		map[string]any{"map.showBuildings3D": "yes"})
	require.Equal(t, fiber.StatusBadRequest, status)
}

func TestSettings_Patch_RejectsInvalidEnum(t *testing.T) {
	app, _, sid := buildSettingsApp(t)
	status, _ := doAdmin(t, app, sid, "PATCH", "/api/settings",
		map[string]any{"map.style": "sepia"})
	require.Equal(t, fiber.StatusBadRequest, status)
}

func TestSettings_Patch_TransactionRollback(t *testing.T) {
	// Mix a valid + invalid key. The valid one must NOT be written.
	app, store, sid := buildSettingsApp(t)
	status, _ := doAdmin(t, app, sid, "PATCH", "/api/settings", map[string]any{
		"map.style":     "dark",
		"map.maxZoom":   99, // out of range
	})
	require.Equal(t, fiber.StatusBadRequest, status)

	// map.style must remain at its default.
	s, err := store.GetString("map.style")
	require.NoError(t, err)
	require.Equal(t, "light", s)
}

func TestSettings_Patch_EmptyBodyRejected(t *testing.T) {
	app, _, sid := buildSettingsApp(t)
	status, _ := doAdmin(t, app, sid, "PATCH", "/api/settings", nil)
	require.Equal(t, fiber.StatusBadRequest, status)
}

func TestSettings_Patch_AuthRequired(t *testing.T) {
	app, _, _ := buildSettingsApp(t)
	body, _ := json.Marshal(map[string]any{"map.style": "dark"})
	req := httptest.NewRequest("PATCH", "/api/settings", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}
