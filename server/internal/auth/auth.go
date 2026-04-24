// Package auth implements native session-cookie authentication for the
// LocalMaps gateway.
//
// The request cycle:
//  1. POST /api/auth/login with {username,password} verifies the bcrypt
//     hash against the `users` table and, on success, writes a row into
//     `sessions` and sets an HTTP-only cookie.
//  2. Subsequent requests carry the cookie; the Require middleware reads
//     it, looks up the session, checks expiry + user.disabled, and
//     attaches the Identity to the fiber ctx.
//  3. POST /api/auth/logout deletes the session row.
//
// Roles: `admin` (full access) or `viewer` (read-only admin pages).
package auth

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"time"

	"github.com/jmoiron/sqlx"
	"golang.org/x/crypto/bcrypt"
)

// Roles.
const (
	RoleAdmin  = "admin"
	RoleViewer = "viewer"
)

// BcryptCost is the target cost for password hashing (per docs/08-security.md).
const BcryptCost = 12

// identityKey is the fiber.Ctx.Locals key under which *Identity is stored.
const identityKey = "identity"

// sessionIDBytes is the number of random bytes behind a session cookie.
const sessionIDBytes = 32

// Identity is the authenticated principal for the current request.
type Identity struct {
	UserID    int64  `json:"id"`
	Username  string `json:"username"`
	Role      string `json:"role"`
	SessionID string `json:"-"`
}

// IsAdmin reports whether the identity is allowed to hit admin-only endpoints.
func (i *Identity) IsAdmin() bool { return i != nil && i.Role == RoleAdmin }

// User is a persisted row from the `users` table.
type User struct {
	ID           int64          `db:"id" json:"id"`
	Username     string         `db:"username" json:"username"`
	PasswordHash string         `db:"password_hash" json:"-"`
	Role         string         `db:"role" json:"role"`
	CreatedAt    string         `db:"created_at" json:"createdAt"`
	LastLoginAt  sql.NullString `db:"last_login_at" json:"lastLoginAt,omitempty"`
	Disabled     bool           `db:"disabled" json:"disabled"`
}

// CookieConfig is the subset of settings we read at cookie-set time.
type CookieConfig struct {
	Name       string
	Secure     bool
	TTLSeconds int
}

// Manager is the session/user service. It owns the DB and the cookie
// tuneables and is the single home for the hashing helpers used by
// handlers + bootstrap.
type Manager struct {
	db     *sqlx.DB
	cookie CookieConfig
	now    func() time.Time
}

// NewManager wires a Manager against the given database and cookie config.
func NewManager(db *sqlx.DB, c CookieConfig) *Manager {
	if c.Name == "" {
		c.Name = "localmaps_session"
	}
	return &Manager{db: db, cookie: c, now: time.Now}
}

// Cookie returns the cookie name (used by router for clearing).
func (m *Manager) Cookie() CookieConfig { return m.cookie }

// HashPassword produces a bcrypt hash with the canonical cost.
func HashPassword(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), BcryptCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// VerifyPassword returns nil on match, an error otherwise.
func VerifyPassword(hash, plain string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain))
}

// RandomPassword returns a printable password of the requested byte length
// (base64 raw, stripped to alnum-ish).
func RandomPassword(nBytes int) (string, error) {
	if nBytes < 8 {
		nBytes = 8
	}
	buf := make([]byte, nBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// randomSessionID returns a URL-safe random session id.
func randomSessionID() (string, error) {
	buf := make([]byte, sessionIDBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// ErrInvalidCredentials is returned by Login on wrong user/pass.
var ErrInvalidCredentials = errors.New("invalid credentials")

// ErrUserDisabled is returned by Login when the account is disabled.
var ErrUserDisabled = errors.New("user disabled")

// ErrSessionExpired is returned by LookupSession when the row is gone or past expiry.
var ErrSessionExpired = errors.New("session expired")
