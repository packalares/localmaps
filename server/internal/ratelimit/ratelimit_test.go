package ratelimit_test

import (
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"

	"github.com/packalares/localmaps/server/internal/ratelimit"
)

// fakeProvider returns settings from an in-memory map. It's guarded so
// tests can mutate the limit mid-run and verify the middleware picks it up.
type fakeProvider struct {
	mu  sync.Mutex
	val map[string]int
}

func (f *fakeProvider) GetInt(key string) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.val[key], nil
}

func (f *fakeProvider) set(key string, v int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.val[key] = v
}

func newApp(l *ratelimit.Limiter, key string) *fiber.App {
	app := fiber.New()
	// Fiber v3: Get(path, handler, middleware...). The middleware runs
	// before handler at request time.
	app.Get("/t", func(c fiber.Ctx) error {
		return c.SendString("ok")
	}, l.PerIP(key))
	return app
}

func TestPerIP_EnforcesLimit(t *testing.T) {
	p := &fakeProvider{val: map[string]int{"rl.test": 3}}
	l := ratelimit.New(p)
	app := newApp(l, "rl.test")

	for i := 0; i < 3; i++ {
		resp, err := app.Test(httptest.NewRequest("GET", "/t", nil))
		require.NoError(t, err)
		require.Equalf(t, fiber.StatusOK, resp.StatusCode, "hit %d", i)
	}
	// 4th hit must 429.
	resp, err := app.Test(httptest.NewRequest("GET", "/t", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusTooManyRequests, resp.StatusCode)
}

func TestPerIP_SettingsDriven(t *testing.T) {
	p := &fakeProvider{val: map[string]int{"rl.test": 1}}
	l := ratelimit.New(p)
	app := newApp(l, "rl.test")

	// First request ok; second is over the limit.
	resp, err := app.Test(httptest.NewRequest("GET", "/t", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest("GET", "/t", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusTooManyRequests, resp.StatusCode)

	// Raise the limit at runtime; next request should succeed without a restart.
	p.set("rl.test", 100)
	resp, err = app.Test(httptest.NewRequest("GET", "/t", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestPerIP_ZeroOrMissingIsFailOpen(t *testing.T) {
	// Missing key -> GetInt returns 0 -> treat as "no limit", allow through.
	p := &fakeProvider{val: map[string]int{}}
	l := ratelimit.New(p)
	app := newApp(l, "rl.absent")

	for i := 0; i < 5; i++ {
		resp, err := app.Test(httptest.NewRequest("GET", "/t", nil))
		require.NoError(t, err)
		require.Equal(t, fiber.StatusOK, resp.StatusCode)
	}
}
