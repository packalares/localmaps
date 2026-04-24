package api

// Test-only fakes shared by share_test.go. Lives in package `api`
// (not `api_test`) so it can touch the unexported shareHTTP plumbing
// from share.go; kept in a separate file to hold share_test.go under
// the 250-line per-file cap.

import (
	"context"

	"github.com/packalares/localmaps/server/internal/shortlinks"
)

// fakeSettings implements shareSettings for tests. The errOrg / errTTL
// knobs let tests force a config read-miss path without touching a
// real SQLite.
type fakeSettings struct {
	origin string
	ttl    int
	errOrg error
	errTTL error
}

func (f fakeSettings) GetString(key string) (string, error) {
	if key == "share.ogBaseURL" {
		return f.origin, f.errOrg
	}
	return "", nil
}

func (f fakeSettings) GetInt(key string) (int, error) {
	if key == "share.shortLinkTTLDays" {
		return f.ttl, f.errTTL
	}
	return 0, nil
}

// fakeStore implements linksStore. Callers wire createFn / resolveFn
// to drive the scenarios they care about; un-set fields default to
// the unsurprising behaviour (Create succeeds with a deterministic
// code, Resolve fails with ErrNotFound).
type fakeStore struct {
	created   []string
	createFn  func(ctx context.Context, rawURL string) (shortlinks.ShortLink, error)
	resolveFn func(ctx context.Context, code string, ttl int) (shortlinks.ShortLink, error)
	views     []string
	// incrementErr is returned when non-nil to exercise the fire-and-
	// forget bookkeeping path (which must still redirect).
	incrementErr error
}

func (f *fakeStore) Create(ctx context.Context, rawURL string) (shortlinks.ShortLink, error) {
	f.created = append(f.created, rawURL)
	if f.createFn != nil {
		return f.createFn(ctx, rawURL)
	}
	return shortlinks.ShortLink{Code: "ABCDE12", URL: rawURL}, nil
}

func (f *fakeStore) Resolve(ctx context.Context, code string, ttl int) (shortlinks.ShortLink, error) {
	if f.resolveFn != nil {
		return f.resolveFn(ctx, code, ttl)
	}
	return shortlinks.ShortLink{}, shortlinks.ErrNotFound
}

func (f *fakeStore) IncrementViews(_ context.Context, code string) error {
	f.views = append(f.views, code)
	return f.incrementErr
}
