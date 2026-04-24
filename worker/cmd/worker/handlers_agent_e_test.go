// Tests for sqlxSettings — the worker-side shim that reads JSON-encoded
// values from the shared config.db settings table. The server's config
// package writes values in JSON, so every getter here must decode
// strings, ints, bools, and []string entries the way the gateway writes
// them.
package main

import (
	"database/sql"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

// seedSettings creates a tiny settings table and inserts one row per
// (key, json-value) pair. Mirrors server/internal/config/store.go.
func seedSettings(t *testing.T, rows map[string]string) *sqlxSettings {
	t.Helper()
	raw, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = raw.Close() })
	_, err = raw.Exec(`CREATE TABLE settings (key TEXT PRIMARY KEY, value TEXT NOT NULL)`)
	require.NoError(t, err)
	for k, v := range rows {
		_, err := raw.Exec(`INSERT INTO settings(key, value) VALUES (?, ?)`, k, v)
		require.NoError(t, err)
	}
	return &sqlxSettings{db: sqlx.NewDb(raw, "sqlite")}
}

func TestSqlxSettings_GetString(t *testing.T) {
	s := seedSettings(t, map[string]string{
		"tiles.planetilerJarURL": `"https://example/planetiler.jar"`,
	})
	got, err := s.GetString("tiles.planetilerJarURL")
	require.NoError(t, err)
	require.Equal(t, "https://example/planetiler.jar", got)
}

func TestSqlxSettings_GetInt(t *testing.T) {
	s := seedSettings(t, map[string]string{
		"tiles.planetilerMemoryMB": `4096`,
	})
	got, err := s.GetInt("tiles.planetilerMemoryMB")
	require.NoError(t, err)
	require.Equal(t, 4096, got)
}

func TestSqlxSettings_GetBool(t *testing.T) {
	s := seedSettings(t, map[string]string{
		"search.peliasPolylinesEnabled": `true`,
	})
	got, err := s.GetBool("search.peliasPolylinesEnabled")
	require.NoError(t, err)
	require.True(t, got)
}

func TestSqlxSettings_GetStringSlice(t *testing.T) {
	s := seedSettings(t, map[string]string{
		"search.peliasLanguages": `["en","fr"]`,
	})
	got, err := s.GetStringSlice("search.peliasLanguages")
	require.NoError(t, err)
	require.Equal(t, []string{"en", "fr"}, got)
}

// TestSqlxSettings_MissingKey asserts every getter returns a non-nil
// error when the key is absent so fallback helpers in
// handlers_agent_fg.go take the default path.
func TestSqlxSettings_MissingKey(t *testing.T) {
	s := seedSettings(t, map[string]string{})
	_, err := s.GetString("nope")
	require.Error(t, err)
	_, err = s.GetInt("nope")
	require.Error(t, err)
	_, err = s.GetBool("nope")
	require.Error(t, err)
	_, err = s.GetStringSlice("nope")
	require.Error(t, err)
}

// TestSqlxSettings_NilReceiver covers the degraded path (DB didn't
// open at boot): every getter must return a sentinel error rather
// than panic. settingsOrDefault / settingsIntOrDefault then fall back
// to documented defaults.
func TestSqlxSettings_NilReceiver(t *testing.T) {
	var s *sqlxSettings
	_, err := s.GetString("x")
	require.Error(t, err)
	_, err = s.GetInt("x")
	require.Error(t, err)
	_, err = s.GetBool("x")
	require.Error(t, err)
	_, err = s.GetStringSlice("x")
	require.Error(t, err)
}
