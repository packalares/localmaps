package shortlinks

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

// Sentinel errors exposed to the HTTP layer. The handler maps these to
// the ErrorResponse/HTTP status pairs declared in contracts/openapi.yaml
// (share tag — 404 on missing; 404 on expired — no custom codes since
// the openapi ErrorCode enum doesn't expose a `LINK_EXPIRED` value).
var (
	// ErrNotFound — no row with that code exists.
	ErrNotFound = errors.New("shortlinks: not found")
	// ErrExpired — the row exists but was created longer ago than the
	// current `share.shortLinkTTLDays` setting. Treated as 404 by the
	// HTTP layer per openapi (no dedicated error code), but kept as a
	// distinct Go value so operators/tests can tell the two apart.
	ErrExpired = errors.New("shortlinks: expired")
	// ErrCodeCollision — the retry budget was exhausted without finding
	// a free code. In practice this is impossible under normal load; it
	// signals either a catastrophically small alphabet change or a DB
	// that has accumulated 62^7 rows. Retryable=true at the handler.
	ErrCodeCollision = errors.New("shortlinks: code collision retries exhausted")
)

// ShortLink mirrors the ShortLink schema in contracts/openapi.yaml. The
// JSON tags match the camelCase shape the UI validates against.
type ShortLink struct {
	Code       string     `db:"code"        json:"code"`
	URL        string     `db:"url"         json:"url"`
	CreatedAt  time.Time  `db:"created_at"  json:"createdAt"`
	LastHitAt  *time.Time `db:"last_hit_at" json:"lastHitAt,omitempty"`
	HitCount   int64      `db:"hit_count"   json:"hitCount"`
}

// maxCreateRetries is the collision-retry budget. At 62^7 possible codes
// collisions are effectively impossible, but we bound the loop anyway.
const maxCreateRetries = 5

// Store wraps a *sqlx.DB and exposes the CRUD surface the share handler
// needs. It is safe for concurrent use; SQLite serialises writes.
type Store struct {
	db        *sqlx.DB
	now       func() time.Time
	generator func() string
}

// Option mutates a Store at construction time. Used by tests to inject
// a deterministic clock and code generator.
type Option func(*Store)

// WithClock overrides the now() source (tests).
func WithClock(fn func() time.Time) Option {
	return func(s *Store) { s.now = fn }
}

// WithGenerator overrides the code generator (tests). The provided
// function is called once per retry attempt during Create.
func WithGenerator(fn func() string) Option {
	return func(s *Store) { s.generator = fn }
}

// New builds a Store over the supplied DB. Use config.Store.DB() in
// production wiring; tests can pass an in-memory sqlx.DB directly.
func New(db *sqlx.DB, opts ...Option) *Store {
	s := &Store{db: db, now: time.Now, generator: Generate}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Create inserts a new short-link row, retrying on the (astronomically
// rare) primary-key collision. Returns the persisted ShortLink — the
// caller builds `shortUrl` by joining it to `share.ogBaseURL`.
func (s *Store) Create(ctx context.Context, url string) (ShortLink, error) {
	now := s.now().UTC()
	createdAt := now.Format(time.RFC3339Nano)

	var lastErr error
	for attempt := 0; attempt < maxCreateRetries; attempt++ {
		code := s.generator()
		_, err := s.db.ExecContext(ctx,
			`INSERT INTO short_links (code, url, created_at, hit_count)
			 VALUES (?, ?, ?, 0)`,
			code, url, createdAt)
		if err == nil {
			return ShortLink{Code: code, URL: url, CreatedAt: now}, nil
		}
		// SQLite surfaces unique-constraint failures via ErrConstraint;
		// both modernc and mattn drivers embed the word "UNIQUE" in the
		// message. We retry on either signal.
		if isUniqueViolation(err) {
			lastErr = err
			continue
		}
		return ShortLink{}, fmt.Errorf("insert short_link: %w", err)
	}
	return ShortLink{}, fmt.Errorf("%w: last err=%v", ErrCodeCollision, lastErr)
}

// Resolve fetches the row, checks it against ttlDays (`<=0` disables
// expiry), and returns ErrNotFound / ErrExpired appropriately. Does not
// bump hit_count; callers do that via IncrementViews so the bookkeeping
// happens after the redirect has successfully been written.
func (s *Store) Resolve(ctx context.Context, code string, ttlDays int) (ShortLink, error) {
	var row shortLinkRow
	err := s.db.GetContext(ctx, &row,
		`SELECT code, url, created_at, last_hit_at, hit_count
		   FROM short_links WHERE code = ?`, code)
	if errors.Is(err, sql.ErrNoRows) {
		return ShortLink{}, ErrNotFound
	}
	if err != nil {
		return ShortLink{}, fmt.Errorf("select short_link: %w", err)
	}
	link, err := row.toShortLink()
	if err != nil {
		return ShortLink{}, err
	}
	if ttlDays > 0 {
		cutoff := s.now().UTC().Add(-time.Duration(ttlDays) * 24 * time.Hour)
		if link.CreatedAt.Before(cutoff) {
			return ShortLink{}, ErrExpired
		}
	}
	return link, nil
}

// IncrementViews bumps hit_count and stamps last_hit_at with now().
// Missing rows are a no-op (the caller already 404'd).
func (s *Store) IncrementViews(ctx context.Context, code string) error {
	now := s.now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.ExecContext(ctx,
		`UPDATE short_links
		    SET hit_count = hit_count + 1,
		        last_hit_at = ?
		  WHERE code = ?`, now, code)
	if err != nil {
		return fmt.Errorf("update hit_count: %w", err)
	}
	return nil
}

// Cleanup deletes every row whose created_at is older than ttlDays days
// from now. Returns the number of rows deleted. `ttlDays<=0` is a no-op.
func (s *Store) Cleanup(ctx context.Context, ttlDays int) (int64, error) {
	if ttlDays <= 0 {
		return 0, nil
	}
	cutoff := s.now().UTC().Add(-time.Duration(ttlDays) * 24 * time.Hour).
		Format(time.RFC3339Nano)
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM short_links WHERE created_at < ?`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("cleanup short_links: %w", err)
	}
	return res.RowsAffected()
}

// shortLinkRow is the on-disk projection of a short_links row. We parse
// the TEXT-encoded timestamps into time.Time ourselves because SQLite
// stores them as RFC3339 strings and the driver doesn't auto-convert.
type shortLinkRow struct {
	Code      string         `db:"code"`
	URL       string         `db:"url"`
	CreatedAt string         `db:"created_at"`
	LastHitAt sql.NullString `db:"last_hit_at"`
	HitCount  int64          `db:"hit_count"`
}

func (r shortLinkRow) toShortLink() (ShortLink, error) {
	created, err := time.Parse(time.RFC3339Nano, r.CreatedAt)
	if err != nil {
		return ShortLink{}, fmt.Errorf("parse created_at: %w", err)
	}
	link := ShortLink{
		Code:      r.Code,
		URL:       r.URL,
		CreatedAt: created.UTC(),
		HitCount:  r.HitCount,
	}
	if r.LastHitAt.Valid && r.LastHitAt.String != "" {
		t, err := time.Parse(time.RFC3339Nano, r.LastHitAt.String)
		if err != nil {
			return ShortLink{}, fmt.Errorf("parse last_hit_at: %w", err)
		}
		utc := t.UTC()
		link.LastHitAt = &utc
	}
	return link, nil
}

// isUniqueViolation returns true when err is a SQLite unique constraint
// failure. We use the driver-agnostic string probe so the package keeps
// working under both modernc.org/sqlite and mattn/go-sqlite3.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return containsAny(msg,
		"UNIQUE constraint failed",
		"constraint failed: UNIQUE",
		"SQLITE_CONSTRAINT_PRIMARYKEY",
		"SQLITE_CONSTRAINT_UNIQUE",
	)
}

func containsAny(haystack string, needles ...string) bool {
	for _, n := range needles {
		if indexOf(haystack, n) >= 0 {
			return true
		}
	}
	return false
}

// Tiny stdlib-free substring match so the package avoids importing
// strings just for this one probe — keeps the file under the 250-line
// cap without dragging in a larger dependency cascade.
func indexOf(h, n string) int {
	if len(n) == 0 {
		return 0
	}
	for i := 0; i+len(n) <= len(h); i++ {
		if h[i:i+len(n)] == n {
			return i
		}
	}
	return -1
}
