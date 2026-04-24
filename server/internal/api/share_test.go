package api

// Handler tests for the `share` tag — POST /api/links and
// GET /api/links/{code}. Lives in package `api` (not `api_test`) so it
// can use setShareHTTP() to inject a fake store without a real SQLite.
// Shared test doubles live in share_fakes_test.go.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"

	"github.com/packalares/localmaps/server/internal/apierr"
	"github.com/packalares/localmaps/server/internal/shortlinks"
)

// newShareApp builds a bare Fiber app with only the share routes wired
// to our fakes. Skips the heavier buildApp() harness used in
// router_test.go — we don't need telemetry/metrics here.
func newShareApp(t *testing.T, sh *ShareHTTP) *fiber.App {
	t.Helper()
	restore := setShareHTTP(sh)
	t.Cleanup(restore)
	app := fiber.New()
	app.Post("/api/links", linksCreateHandler)
	app.Get("/api/links/:code", linksResolveHandler)
	return app
}

// --- POST /api/links -----------------------------------------------

func TestLinksCreate_HappyPath(t *testing.T) {
	store := &fakeStore{}
	settings := fakeSettings{origin: "http://localhost:8080", ttl: 365}
	app := newShareApp(t, &ShareHTTP{Store: store, Settings: settings})

	body, _ := json.Marshal(map[string]string{"url": "/#12/45.0/25.0"})
	req := httptest.NewRequest("POST", "/api/links", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusCreated, resp.StatusCode)

	var out map[string]any
	raw, _ := io.ReadAll(resp.Body)
	require.NoError(t, json.Unmarshal(raw, &out))
	require.Equal(t, "ABCDE12", out["code"])
	require.Equal(t, "/#12/45.0/25.0", out["url"])
	require.Equal(t, []string{"/#12/45.0/25.0"}, store.created)
}

func TestLinksCreate_AbsoluteSameOrigin(t *testing.T) {
	store := &fakeStore{}
	settings := fakeSettings{origin: "https://maps.example.com", ttl: 365}
	app := newShareApp(t, &ShareHTTP{Store: store, Settings: settings})

	body, _ := json.Marshal(map[string]string{
		"url": "https://maps.example.com/#12/45.0/25.0",
	})
	req := httptest.NewRequest("POST", "/api/links", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusCreated, resp.StatusCode)
}

func TestLinksCreate_RejectsOpenRedirect(t *testing.T) {
	settings := fakeSettings{origin: "https://maps.example.com", ttl: 365}
	app := newShareApp(t, &ShareHTTP{Store: &fakeStore{}, Settings: settings})

	bad := []string{
		"https://evil.example/phish",
		"//evil.example/phish",
		"javascript:alert(1)",
		"",
		"http://localhost:9999/#/x",
	}
	for _, u := range bad {
		body, _ := json.Marshal(map[string]string{"url": u})
		req := httptest.NewRequest("POST", "/api/links", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		require.NoErrorf(t, err, "url=%q", u)
		require.Equalf(t, fiber.StatusBadRequest, resp.StatusCode,
			"url=%q should be rejected", u)
	}
}

func TestLinksCreate_CollisionExhaustion(t *testing.T) {
	store := &fakeStore{
		createFn: func(context.Context, string) (shortlinks.ShortLink, error) {
			return shortlinks.ShortLink{}, shortlinks.ErrCodeCollision
		},
	}
	settings := fakeSettings{origin: "http://localhost:8080", ttl: 365}
	app := newShareApp(t, &ShareHTTP{Store: store, Settings: settings})

	body, _ := json.Marshal(map[string]string{"url": "/#x"})
	req := httptest.NewRequest("POST", "/api/links", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)

	raw, _ := io.ReadAll(resp.Body)
	var env apierr.ErrorResponse
	require.NoError(t, json.Unmarshal(raw, &env))
	require.Equal(t, apierr.CodeInternal, env.Error.Code)
	require.True(t, env.Error.Retryable, "collision errors must be retryable")
}

func TestLinksCreate_MalformedBody(t *testing.T) {
	settings := fakeSettings{origin: "http://localhost:8080"}
	app := newShareApp(t, &ShareHTTP{Store: &fakeStore{}, Settings: settings})

	req := httptest.NewRequest("POST", "/api/links", strings.NewReader("not-json"))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// --- GET /api/links/{code} -----------------------------------------

func TestLinksResolve_HappyRedirect(t *testing.T) {
	store := &fakeStore{
		resolveFn: func(_ context.Context, code string, _ int) (shortlinks.ShortLink, error) {
			require.Equal(t, "ABCDE12", code)
			return shortlinks.ShortLink{Code: code, URL: "/#12/45/25"}, nil
		},
	}
	settings := fakeSettings{origin: "http://localhost:8080", ttl: 365}
	app := newShareApp(t, &ShareHTTP{Store: store, Settings: settings})

	resp, err := app.Test(httptest.NewRequest("GET", "/api/links/ABCDE12", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusMovedPermanently, resp.StatusCode)
	require.Equal(t, "/#12/45/25", resp.Header.Get("Location"))
	require.Equal(t, []string{"ABCDE12"}, store.views)
}

func TestLinksResolve_NotFound(t *testing.T) {
	settings := fakeSettings{origin: "http://localhost:8080", ttl: 365}
	app := newShareApp(t, &ShareHTTP{Store: &fakeStore{}, Settings: settings})

	resp, err := app.Test(httptest.NewRequest("GET", "/api/links/NOTHERE", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}

func TestLinksResolve_ExpiredMapsTo404(t *testing.T) {
	store := &fakeStore{
		resolveFn: func(context.Context, string, int) (shortlinks.ShortLink, error) {
			return shortlinks.ShortLink{}, shortlinks.ErrExpired
		},
	}
	settings := fakeSettings{origin: "http://localhost:8080", ttl: 7}
	app := newShareApp(t, &ShareHTTP{Store: store, Settings: settings})

	resp, err := app.Test(httptest.NewRequest("GET", "/api/links/OLDONE_", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}

func TestLinksResolve_IncrementFailDoesNotBlockRedirect(t *testing.T) {
	store := &fakeStore{
		resolveFn: func(context.Context, string, int) (shortlinks.ShortLink, error) {
			return shortlinks.ShortLink{Code: "X", URL: "/foo"}, nil
		},
		incrementErr: errors.New("write fail"),
	}
	settings := fakeSettings{origin: "http://localhost:8080", ttl: 365}
	app := newShareApp(t, &ShareHTTP{Store: store, Settings: settings})

	resp, err := app.Test(httptest.NewRequest("GET", "/api/links/X", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusMovedPermanently, resp.StatusCode)
}

// --- stub fallthrough ----------------------------------------------

func TestLinksHandlers_StubWhenNotWired(t *testing.T) {
	restore := setShareHTTP(nil)
	t.Cleanup(restore)
	app := fiber.New()
	app.Post("/api/links", linksCreateHandler)
	app.Get("/api/links/:code", linksResolveHandler)

	for _, tc := range []struct{ method, path string }{
		{"POST", "/api/links"},
		{"GET", "/api/links/foo"},
	} {
		resp, err := app.Test(httptest.NewRequest(tc.method, tc.path, nil))
		require.NoError(t, err)
		require.Equalf(t, fiber.StatusNotImplemented, resp.StatusCode,
			"%s %s", tc.method, tc.path)
	}
}

// --- isSafeShareURL direct coverage --------------------------------

func TestIsSafeShareURL(t *testing.T) {
	origin := "http://localhost:8080"
	cases := []struct {
		name, target string
		want         bool
	}{
		{"empty", "", false},
		{"simple relative", "/foo", true},
		{"hash-only relative", "/#12/45/25", true},
		{"protocol relative rejected", "//evil/x", false},
		{"absolute same origin", "http://localhost:8080/x", true},
		{"absolute different host", "http://evil.example/x", false},
		{"absolute different scheme", "https://localhost:8080/x", false},
		{"crlf injection", "/foo\r\nX-Evil: 1", false},
		{"javascript scheme", "javascript:alert(1)", false},
		{"data scheme", "data:text/html,<script>", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			require.Equal(t, c.want, isSafeShareURL(c.target, origin))
		})
	}
}
