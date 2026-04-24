// Package config owns the SQLite-backed configuration database described
// in docs/04-data-model.md. It exposes a small Store API that the rest
// of the gateway uses to read and write runtime settings. All SQL uses
// named or positional parameters — never string concatenation of values.
package config

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"

	// Registers the "sqlite" database/sql driver (pure-Go, no CGO).
	_ "modernc.org/sqlite"
)

// ErrNotFound is returned when Get cannot find a key.
var ErrNotFound = errors.New("config: key not found")

const (
	// schemaVersionKey is the settings row that tracks migration state.
	schemaVersionKey = "schema.version"
	// systemUser labels default/seeded rows.
	systemUser = "system"
)

// Store is a thin wrapper around *sqlx.DB exposing the operations the
// gateway needs. It serialises writes that must be transactional.
type Store struct {
	db  *sqlx.DB
	mu  sync.Mutex
	now func() time.Time
}

// Open opens (or creates) the SQLite file at path, runs migrations, and
// seeds defaults. It is idempotent — calling Open on an existing DB
// leaves existing settings untouched. Pass ":memory:" for tests.
func Open(path string) (*Store, error) {
	dsn := path
	if path != ":memory:" {
		// _pragma flags are accepted by modernc.org/sqlite.
		dsn = fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)",
			filepath.ToSlash(path))
	}
	raw, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if err := raw.Ping(); err != nil {
		_ = raw.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	// In-memory DBs are per-connection; pin to one to survive the lifetime.
	if path == ":memory:" {
		raw.SetMaxOpenConns(1)
	}
	db := sqlx.NewDb(raw, "sqlite")
	s := &Store{db: db, now: time.Now}
	if err := s.runMigrations(); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := s.seedDefaults(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

// Close releases the underlying connection pool.
func (s *Store) Close() error { return s.db.Close() }

// DB exposes the underlying *sqlx.DB for packages that need to query
// other tables (regions, jobs, short_links, etc). Use named parameters.
func (s *Store) DB() *sqlx.DB { return s.db }

// runMigrations applies every DDL block in order. DDL is idempotent.
func (s *Store) runMigrations() error {
	for i, stmt := range migrations {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("migration %d: %w", i, err)
		}
	}
	return nil
}

// seedDefaults inserts defaults only on the first boot (tracked by
// settings row schema.version). It will also append newly-added
// defaults when the schema version advances.
func (s *Store) seedDefaults() error {
	ctx := context.Background()
	current, err := s.currentSchemaVersion(ctx)
	if err != nil {
		return err
	}
	return s.Transaction(func(tx *sqlx.Tx) error {
		for _, d := range Defaults() {
			v, err := encodeDefault(d.Value)
			if err != nil {
				return fmt.Errorf("encode default %q: %w", d.Key, err)
			}
			// INSERT OR IGNORE — existing values are preserved.
			_, err = tx.Exec(
				`INSERT OR IGNORE INTO settings (key, value, updated_at, updated_by)
				 VALUES (?, ?, ?, ?)`,
				d.Key, v, s.now().UTC().Format(time.RFC3339Nano), systemUser,
			)
			if err != nil {
				return fmt.Errorf("seed %q: %w", d.Key, err)
			}
		}
		// Always write the current schema version last.
		v, _ := encodeDefault(SchemaVersion)
		_, err := tx.Exec(
			`INSERT INTO settings (key, value, updated_at, updated_by)
			 VALUES (?, ?, ?, ?)
			 ON CONFLICT(key) DO UPDATE SET
			    value = excluded.value,
			    updated_at = excluded.updated_at,
			    updated_by = excluded.updated_by`,
			schemaVersionKey, v, s.now().UTC().Format(time.RFC3339Nano), systemUser,
		)
		if err != nil {
			return fmt.Errorf("write schema version: %w", err)
		}
		_ = current
		return nil
	})
}

// currentSchemaVersion reads the current schema.version, returning 0 if absent.
func (s *Store) currentSchemaVersion(ctx context.Context) (int, error) {
	var raw string
	err := s.db.GetContext(ctx, &raw,
		`SELECT value FROM settings WHERE key = ?`, schemaVersionKey)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	var v int
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return 0, fmt.Errorf("decode schema.version: %w", err)
	}
	return v, nil
}

// Get fetches a setting by key and JSON-decodes it into dest. Returns
// ErrNotFound when the key is absent.
func (s *Store) Get(key string, dest any) error {
	var raw string
	err := s.db.Get(&raw, `SELECT value FROM settings WHERE key = ?`, key)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(raw), dest)
}

// GetRaw returns the raw JSON string for a key, useful when the caller
// wants to forward it untouched to the client.
func (s *Store) GetRaw(key string) (string, error) {
	var raw string
	err := s.db.Get(&raw, `SELECT value FROM settings WHERE key = ?`, key)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	return raw, err
}

// Set JSON-encodes value and upserts it under key. user is recorded in
// updated_by — use "system" for programmatic writes.
func (s *Store) Set(key string, value any, user string) error {
	b, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("encode value: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err = s.db.Exec(
		`INSERT INTO settings (key, value, updated_at, updated_by)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(key) DO UPDATE SET
		     value = excluded.value,
		     updated_at = excluded.updated_at,
		     updated_by = excluded.updated_by`,
		key, string(b), s.now().UTC().Format(time.RFC3339Nano), user,
	)
	return err
}

// GetInt is a typed helper used by rate-limit middleware, returning the
// integer value at key or an error if it's absent or can't be decoded.
func (s *Store) GetInt(key string) (int, error) {
	var v int
	if err := s.Get(key, &v); err != nil {
		return 0, err
	}
	return v, nil
}

// GetString is a typed helper for callers that want a plain string.
func (s *Store) GetString(key string) (string, error) {
	var v string
	if err := s.Get(key, &v); err != nil {
		return "", err
	}
	return v, nil
}

// GetBool is a typed helper for callers that want a plain bool.
func (s *Store) GetBool(key string) (bool, error) {
	var v bool
	if err := s.Get(key, &v); err != nil {
		return false, err
	}
	return v, nil
}

// Delete removes a key. Absent keys are not an error.
func (s *Store) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`DELETE FROM settings WHERE key = ?`, key)
	return err
}

// Transaction runs fn inside a single DB transaction, committing on
// nil return and rolling back on any error.
func (s *Store) Transaction(fn func(tx *sqlx.Tx) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Beginx()
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}
