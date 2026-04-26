package config

import "encoding/json"

// SchemaVersion is the current settings schema version. Bump when new
// defaults are seeded or deprecated keys are removed.
const SchemaVersion = 1

// Default is a seed row for the settings table. Value is serialised as
// JSON; consumers decode into whatever shape they expect.
type Default struct {
	Key   string
	Value any
}

// Defaults lists every runtime setting from docs/07-config-schema.md.
// Keep this in sync with that doc. Values here are the authoritative
// defaults for first boot.
func Defaults() []Default {
	return []Default{
		// --- map.* ----------------------------------------------------
		{"map.defaultCenter", map[string]any{"lat": 0, "lon": 0, "zoom": 2}},
		{"map.style", "light"},
		{"map.maxZoom", 14},
		{"map.minZoom", 0},
		{"map.language", "default"},
		{"map.units", "metric"},
		{"map.showBuildings3D", true},
		{"map.rotationEnabled", true},
		{"map.tiltEnabled", true},
		{"map.attribution", "© OpenStreetMap contributors, Overture Maps"},

		// --- search.* -------------------------------------------------
		{"search.provider", "pelias"},
		{"search.resultLimit", 10},
		{"search.biasToInstalledRegions", true},
		{"search.showHistory", true},
		{"search.historyRetentionDays", 90},
		{"search.autocompleteDebounceMs", 150},
		{"search.peliasElasticUrl", "http://127.0.0.1:9200"},
		{"search.peliasLanguages", []string{"en"}},
		{"search.peliasPolylinesEnabled", false},
		{"search.peliasBuildTimeoutMinutes", 120},
		{"search.peliasImporterImage", "pelias/openstreetmap:6.4.0"},

		// --- routing.* ------------------------------------------------
		{"routing.provider", "valhalla"},
		{"routing.defaultMode", "auto"},
		{"routing.avoidHighways", false},
		{"routing.avoidTolls", false},
		{"routing.avoidFerries", false},
		{"routing.maxAlternatives", 2},
		{"routing.truck.heightMeters", 3.5},
		{"routing.truck.widthMeters", 2.6},
		{"routing.truck.weightTons", 7.5},
		{"routing.truck.lengthMeters", 10.0},
		{"routing.valhallaConcurrency", 0},
		{"routing.valhallaBuildTimeoutMinutes", 60},
		{"routing.valhallaExtraArgs", []string{}},
		{"routing.valhallaTileDirName", "valhalla_tiles"},
		{"routing.activeRegion", ""},

		// --- tiles.* --------------------------------------------------
		{"tiles.source", "protomaps"},
		{"tiles.cacheTTLSeconds", 86400},
		{"tiles.diskCacheBytes", int64(5368709120)},
		{"tiles.memoryCacheItems", 1024},
		{"tiles.planetilerJarURL", "https://github.com/onthegomap/planetiler/releases/download/v0.8.2/planetiler.jar"},
		{"tiles.planetilerJarSha256", ""},
		{"tiles.planetilerMemoryMB", 4096},
		{"tiles.planetilerExtraArgs", []string{}},
		{"tiles.planetilerMaxDurationMinutes", 240},
		{"tiles.planetilerRetryCount", 3},

		// --- pois.* ---------------------------------------------------
		{"pois.sources", []string{"overture", "osm"}},
		{"pois.defaultRadiusMeters", 500},
		{"pois.categoryDefaults", []string{}},

		// --- regions.* ------------------------------------------------
		{"regions.catalogURL", "https://download.geofabrik.de/index-v1.json"},
		{"regions.mirrorBase", "https://download.geofabrik.de"},
		{"regions.defaultSchedule", "monthly"},
		{"regions.updateCheckCron", "0 3 * * *"},
		{"regions.retryAttempts", 3},
		{"regions.retryBackoffSeconds", 60},
		{"regions.downloadBandwidthBytes", 0},
		{"regions.keepSourcePbf", true},
		{"regions.archivedRetentionDays", 30},
		{"regions.maxConcurrentBuilds", 1},
		{"regions.deletePurgesPelias", true},

		// --- share.* --------------------------------------------------
		{"share.shortLinkTTLDays", 365},
		{"share.ogBaseURL", "http://localhost:8080"},
		{"share.ogRenderTimeoutSeconds", 10},
		{"share.ogCacheTTLSeconds", 604800},
		{"share.embedAllowedOrigins", []string{"*"}},
		{"share.embedUIBaseURL", ""},
		{"share.qrCodeSizePx", 256},

		// --- rateLimit.* ----------------------------------------------
		{"rateLimit.tilesPerMinutePerIP", 600},
		{"rateLimit.geocodePerMinutePerIP", 120},
		{"rateLimit.routePerMinutePerIP", 60},
		{"rateLimit.regionsAdminPerMinute", 30},
		{"rateLimit.ogPreviewPerMinutePerIP", 10},
		{"rateLimit.bypassAuthenticated", true},

		// --- auth.* ---------------------------------------------------
		// Native session-cookie auth. auth.mode / auth.basicUsers were
		// removed in favour of the real `users` table; see
		// docs/07-config-schema.md and docs/08-security.md.
		{"auth.publicReadOnly", true},
		{"auth.sessionTTLHours", 168},
		{"auth.cookieName", "localmaps_session"},
		{"auth.cookieSecure", true},
		{"auth.passwordMinLength", 10},
		{"auth.rateLimit.loginPerMinutePerIP", 10},

		// --- security.* -----------------------------------------------
		{"security.allowedEgressHosts", []string{
			"download.geofabrik.de",
			"github.com",
			"objects.githubusercontent.com",
			"download.kiwix.org",
		}},

		// --- telemetry.* ----------------------------------------------
		{"telemetry.enabled", false},
		{"telemetry.endpoint", ""},
		{"telemetry.sampleRate", 0.1},

		// --- ui.* -----------------------------------------------------
		{"ui.appName", "LocalMaps"},
		{"ui.brandColor", "#0ea5e9"},
		{"ui.favicon", "/favicon.ico"},
		{"ui.showAdminLink", true},
		{"ui.sidebarCollapsedDefault", false},
	}
}

// encodeDefault JSON-encodes a default value. Exposed for tests.
func encodeDefault(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
