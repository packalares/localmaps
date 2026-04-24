package config

// Migrations are idempotent DDL blocks. Each one creates a table or index
// from docs/04-data-model.md with the exact columns listed there. We use
// CREATE ... IF NOT EXISTS throughout so re-running is safe.
var migrations = []string{
	// settings ---------------------------------------------------------
	`CREATE TABLE IF NOT EXISTS settings (
		key         TEXT PRIMARY KEY,
		value       TEXT NOT NULL,
		updated_at  TEXT NOT NULL,
		updated_by  TEXT
	)`,

	// regions ----------------------------------------------------------
	`CREATE TABLE IF NOT EXISTS regions (
		name                 TEXT PRIMARY KEY,
		display_name         TEXT NOT NULL,
		parent               TEXT,
		source_url           TEXT NOT NULL,
		source_pbf_sha256    TEXT,
		source_pbf_bytes     INTEGER,
		bbox                 TEXT,
		state                TEXT NOT NULL,
		state_detail         TEXT,
		last_error           TEXT,
		installed_at         TEXT,
		last_updated_at      TEXT,
		next_update_at       TEXT,
		schedule             TEXT,
		disk_bytes           INTEGER,
		active_job_id        TEXT
	)`,
	`CREATE INDEX IF NOT EXISTS regions_state_idx ON regions(state)`,

	// jobs -------------------------------------------------------------
	`CREATE TABLE IF NOT EXISTS jobs (
		id             TEXT PRIMARY KEY,
		kind           TEXT NOT NULL,
		region         TEXT,
		state          TEXT NOT NULL,
		progress       REAL,
		message        TEXT,
		started_at     TEXT,
		finished_at    TEXT,
		error          TEXT,
		created_by     TEXT,
		parent_job_id  TEXT
	)`,
	`CREATE INDEX IF NOT EXISTS jobs_state_idx ON jobs(state)`,
	`CREATE INDEX IF NOT EXISTS jobs_region_idx ON jobs(region)`,

	// short_links ------------------------------------------------------
	`CREATE TABLE IF NOT EXISTS short_links (
		code         TEXT PRIMARY KEY,
		url          TEXT NOT NULL,
		created_at   TEXT NOT NULL,
		last_hit_at  TEXT,
		hit_count    INTEGER DEFAULT 0
	)`,

	// saved_places ----------------------------------------------------
	`CREATE TABLE IF NOT EXISTS saved_places (
		id         TEXT PRIMARY KEY,
		user_id    TEXT NOT NULL,
		lat        REAL NOT NULL,
		lon        REAL NOT NULL,
		label      TEXT NOT NULL,
		list_name  TEXT,
		notes      TEXT,
		created_at TEXT NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS saved_places_user_idx ON saved_places(user_id)`,

	// search_history --------------------------------------------------
	`CREATE TABLE IF NOT EXISTS search_history (
		id         TEXT PRIMARY KEY,
		user_id    TEXT NOT NULL,
		query      TEXT NOT NULL,
		resolved   TEXT,
		created_at TEXT NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS search_history_user_idx ON search_history(user_id, created_at DESC)`,

	// route_cache -----------------------------------------------------
	`CREATE TABLE IF NOT EXISTS route_cache (
		id         TEXT PRIMARY KEY,
		request    TEXT NOT NULL,
		response   TEXT NOT NULL,
		created_at TEXT NOT NULL,
		expires_at TEXT NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS route_cache_expiry_idx ON route_cache(expires_at)`,

	// users -----------------------------------------------------------
	// Native authentication: usernames + bcrypt hashes + role. The
	// first admin is bootstrapped in-process on first boot with a
	// printed-once random password (see auth.BootstrapAdmin).
	`CREATE TABLE IF NOT EXISTS users (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		username      TEXT NOT NULL UNIQUE,
		password_hash TEXT NOT NULL,
		role          TEXT NOT NULL DEFAULT 'admin',
		created_at    TEXT NOT NULL,
		last_login_at TEXT,
		disabled      INTEGER NOT NULL DEFAULT 0
	)`,
	`CREATE INDEX IF NOT EXISTS users_username_idx ON users(username)`,

	// sessions --------------------------------------------------------
	// Session cookie tokens. `id` is the random cookie value (32-byte
	// base64). Deleting a row revokes that session.
	`CREATE TABLE IF NOT EXISTS sessions (
		id         TEXT PRIMARY KEY,
		user_id    INTEGER NOT NULL,
		created_at TEXT NOT NULL,
		expires_at TEXT NOT NULL,
		user_agent TEXT,
		ip         TEXT,
		FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
	)`,
	`CREATE INDEX IF NOT EXISTS sessions_user_idx ON sessions(user_id)`,
	`CREATE INDEX IF NOT EXISTS sessions_expiry_idx ON sessions(expires_at)`,
}
