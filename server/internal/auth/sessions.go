package auth

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Session is a row from the `sessions` table.
type Session struct {
	ID        string `db:"id"`
	UserID    int64  `db:"user_id"`
	CreatedAt string `db:"created_at"`
	ExpiresAt string `db:"expires_at"`
	UserAgent string `db:"user_agent"`
	IP        string `db:"ip"`
}

// Login validates credentials and creates a session. Returns the new
// session ID (cookie value) and the authenticated user.
func (m *Manager) Login(username, password, userAgent, ip string) (string, *User, error) {
	u, err := m.GetUserByUsername(username)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil, ErrInvalidCredentials
		}
		return "", nil, err
	}
	if u.Disabled {
		return "", nil, ErrUserDisabled
	}
	if err := VerifyPassword(u.PasswordHash, password); err != nil {
		return "", nil, ErrInvalidCredentials
	}
	sid, err := m.createSession(u.ID, userAgent, ip)
	if err != nil {
		return "", nil, err
	}
	// Best-effort last_login_at; don't fail the login if this write fails.
	_, _ = m.db.Exec(`UPDATE users SET last_login_at = ? WHERE id = ?`,
		m.now().UTC().Format(time.RFC3339Nano), u.ID)
	return sid, u, nil
}

// createSession inserts a new row and returns its id (cookie value).
func (m *Manager) createSession(userID int64, userAgent, ip string) (string, error) {
	sid, err := randomSessionID()
	if err != nil {
		return "", err
	}
	ttl := time.Duration(m.cookie.TTLSeconds) * time.Second
	if ttl <= 0 {
		ttl = 7 * 24 * time.Hour // defensive: one-week default
	}
	now := m.now().UTC()
	_, err = m.db.Exec(
		`INSERT INTO sessions (id, user_id, created_at, expires_at, user_agent, ip)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		sid, userID,
		now.Format(time.RFC3339Nano),
		now.Add(ttl).Format(time.RFC3339Nano),
		userAgent, ip,
	)
	if err != nil {
		return "", err
	}
	return sid, nil
}

// LookupSession resolves a cookie value to an authenticated identity.
// Returns ErrSessionExpired if the cookie doesn't match a live session.
func (m *Manager) LookupSession(sid string) (*Identity, error) {
	if sid == "" {
		return nil, ErrSessionExpired
	}
	var row struct {
		UserID    int64  `db:"user_id"`
		ExpiresAt string `db:"expires_at"`
		Username  string `db:"username"`
		Role      string `db:"role"`
		Disabled  bool   `db:"disabled"`
	}
	err := m.db.Get(&row,
		`SELECT s.user_id, s.expires_at, u.username, u.role, u.disabled
		 FROM sessions s JOIN users u ON u.id = s.user_id
		 WHERE s.id = ?`, sid)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrSessionExpired
		}
		return nil, err
	}
	if row.Disabled {
		return nil, ErrUserDisabled
	}
	expires, err := time.Parse(time.RFC3339Nano, row.ExpiresAt)
	if err != nil {
		return nil, fmt.Errorf("parse session expiry: %w", err)
	}
	if !m.now().UTC().Before(expires) {
		// Best-effort cleanup of the stale row.
		_, _ = m.db.Exec(`DELETE FROM sessions WHERE id = ?`, sid)
		return nil, ErrSessionExpired
	}
	return &Identity{
		UserID:    row.UserID,
		Username:  row.Username,
		Role:      row.Role,
		SessionID: sid,
	}, nil
}

// RevokeSession deletes a session row (logout).
func (m *Manager) RevokeSession(sid string) error {
	if sid == "" {
		return nil
	}
	_, err := m.db.Exec(`DELETE FROM sessions WHERE id = ?`, sid)
	return err
}

// PurgeExpired drops sessions whose expires_at is in the past. Best-effort
// cleanup; callers can wire this into a periodic tick.
func (m *Manager) PurgeExpired() error {
	_, err := m.db.Exec(`DELETE FROM sessions WHERE expires_at < ?`,
		m.now().UTC().Format(time.RFC3339Nano))
	return err
}
