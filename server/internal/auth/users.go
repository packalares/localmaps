package auth

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// CreateUser inserts a new user row. `plainPassword` is bcrypted here
// so callers never pass the hash; roles must be one of the known values.
func (m *Manager) CreateUser(username, plainPassword, role string) (*User, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return nil, errors.New("username required")
	}
	if role != RoleAdmin && role != RoleViewer {
		return nil, fmt.Errorf("invalid role %q", role)
	}
	hash, err := HashPassword(plainPassword)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}
	ts := m.now().UTC().Format(time.RFC3339Nano)
	res, err := m.db.Exec(
		`INSERT INTO users (username, password_hash, role, created_at, disabled)
		 VALUES (?, ?, ?, ?, 0)`,
		username, hash, role, ts)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return m.GetUserByID(id)
}

// GetUserByID loads a single user; returns sql.ErrNoRows when absent.
func (m *Manager) GetUserByID(id int64) (*User, error) {
	var u User
	err := m.db.Get(&u, `SELECT id, username, password_hash, role, created_at,
		last_login_at, disabled FROM users WHERE id = ?`, id)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// GetUserByUsername is the login-lookup helper.
func (m *Manager) GetUserByUsername(username string) (*User, error) {
	var u User
	err := m.db.Get(&u, `SELECT id, username, password_hash, role, created_at,
		last_login_at, disabled FROM users WHERE username = ?`, username)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// ListUsers returns every user row ordered by id ascending.
func (m *Manager) ListUsers() ([]User, error) {
	var out []User
	err := m.db.Select(&out, `SELECT id, username, password_hash, role, created_at,
		last_login_at, disabled FROM users ORDER BY id ASC`)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	return out, nil
}

// DeleteUser removes a user (cascading their sessions via FK).
func (m *Manager) DeleteUser(id int64) error {
	_, err := m.db.Exec(`DELETE FROM users WHERE id = ?`, id)
	return err
}

// CountUsers returns the total row count; used on bootstrap.
func (m *Manager) CountUsers() (int, error) {
	var n int
	err := m.db.Get(&n, `SELECT COUNT(*) FROM users`)
	return n, err
}

// ChangePassword verifies oldPassword then writes a new hash. Returns
// ErrInvalidCredentials on mismatch.
func (m *Manager) ChangePassword(userID int64, oldPassword, newPassword string) error {
	u, err := m.GetUserByID(userID)
	if err != nil {
		return err
	}
	if err := VerifyPassword(u.PasswordHash, oldPassword); err != nil {
		return ErrInvalidCredentials
	}
	hash, err := HashPassword(newPassword)
	if err != nil {
		return err
	}
	_, err = m.db.Exec(`UPDATE users SET password_hash = ? WHERE id = ?`, hash, userID)
	return err
}

// ResetPassword replaces a user's password without verifying the old
// one. Used by admin endpoints and by the first-run bootstrap.
func (m *Manager) ResetPassword(userID int64, newPassword string) error {
	hash, err := HashPassword(newPassword)
	if err != nil {
		return err
	}
	_, err = m.db.Exec(`UPDATE users SET password_hash = ? WHERE id = ?`, hash, userID)
	return err
}
