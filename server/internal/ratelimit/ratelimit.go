// Package ratelimit provides per-IP rate-limiting middleware for Fiber.
// It uses github.com/go-chi/httprate's LocalCounter as the sliding-window
// counter primitive and reads its per-route limits from the config store,
// per docs/06-agent-rules.md R8 and docs/07-config-schema.md.
package ratelimit

import (
	"fmt"
	"sync"
	"time"

	"github.com/go-chi/httprate"
	"github.com/gofiber/fiber/v3"

	"github.com/packalares/localmaps/server/internal/apierr"
)

// Provider resolves a settings key to its current integer value at
// request time. This is a tiny interface so we don't couple ratelimit
// to the concrete config.Store type (which would create an import cycle
// for tests).
type Provider interface {
	GetInt(key string) (int, error)
}

// Limiter is a per-route rate limiter. Use Build() to get a Fiber
// middleware that enforces the given settings key with a 1-minute window.
type Limiter struct {
	mu       sync.Mutex
	counter  httprate.LimitCounter
	window   time.Duration
	provider Provider
	// cached per-settings-key Handler to avoid churn on hot paths.
	keyCache map[string]int
}

// New builds a Limiter backed by a shared sliding-window LocalCounter.
// window defaults to 1 minute — matches the `*PerMinutePerIP` keys in
// the schema.
func New(p Provider) *Limiter {
	c := httprate.NewLocalLimitCounter(time.Minute)
	return &Limiter{
		counter:  c,
		window:   time.Minute,
		provider: p,
		keyCache: make(map[string]int),
	}
}

// PerIP returns a Fiber middleware that enforces the limit stored at
// settingsKey (requests per window per IP). The limit is re-read every
// request so runtime settings updates take effect immediately.
func (l *Limiter) PerIP(settingsKey string) fiber.Handler {
	return func(c fiber.Ctx) error {
		limit, err := l.provider.GetInt(settingsKey)
		if err != nil || limit <= 0 {
			// Fail-open if config missing; log via the request logger if
			// the caller set one. (Observability in production layer.)
			return c.Next()
		}
		// Configure the shared counter to the current limit/window.
		l.mu.Lock()
		l.counter.Config(limit, l.window)
		l.mu.Unlock()

		key := fmt.Sprintf("%s|%s", settingsKey, c.IP())

		now := time.Now().UTC()
		currentWindow := now.Truncate(l.window)
		previousWindow := currentWindow.Add(-l.window)

		currCount, prevCount, err := l.counter.Get(key, currentWindow, previousWindow)
		if err != nil {
			return c.Next()
		}
		diff := now.Sub(currentWindow)
		rate := float64(prevCount)*
			(float64(l.window)-float64(diff))/float64(l.window) +
			float64(currCount)
		if rate >= float64(limit) {
			return apierr.Write(c, apierr.CodeRateLimited,
				"rate limit exceeded", true)
		}
		if err := l.counter.Increment(key, currentWindow); err != nil {
			return c.Next()
		}
		return c.Next()
	}
}
