package settingsschema

// Annotation is a per-key refinement that overlays the inferred schema
// produced by BuildSchema. It is authoritative for enums, numeric
// ranges, units and descriptions — all of which mirror the tables in
// docs/07-config-schema.md. When a new default is added to
// server/internal/config/defaults.go, add its non-trivial constraints
// here.
type Annotation struct {
	Type        Type
	Description string
	Enum        []any
	Min         *float64
	Max         *float64
	Step        *float64
	Pattern     string
	Unit        string
	UIGroup     string
	ItemType    Type
	ReadOnly    bool
}

// floatP / stringEnum are small helpers to keep the map below readable.
func floatP(f float64) *float64 { return &f }
func stringEnum(vs ...string) []any {
	out := make([]any, len(vs))
	for i, v := range vs {
		out[i] = v
	}
	return out
}

// Annotations is the authoritative per-key metadata table. Every entry
// MUST match a key in config.Defaults(); TestValidateAnnotations fails
// if that invariant breaks.
var Annotations = map[string]Annotation{
	// --- map.* --------------------------------------------------------
	"map.defaultCenter": {
		Description: "Initial map centre and zoom for new sessions.",
	},
	"map.style": {
		Description: "Default map theme.",
		Enum:        stringEnum("light", "dark", "auto"),
	},
	"map.maxZoom": {Min: floatP(0), Max: floatP(19), Unit: "zoom"},
	"map.minZoom": {Min: floatP(0), Max: floatP(19), Unit: "zoom"},
	"map.language": {
		Description: "Label language. `default` uses the tile's native name.",
		Enum: stringEnum(
			"default", "en", "ro", "de", "fr", "es", "zh", "ja",
		),
	},
	"map.units": {
		Description: "Distance unit preference.",
		Enum:        stringEnum("metric", "imperial"),
	},
	"map.showBuildings3D": {Description: "Render 3D building extrusions."},
	"map.rotationEnabled": {Description: "Allow rotating the map with two-finger gesture."},
	"map.tiltEnabled":     {Description: "Allow tilting the map to enter 3D view."},
	"map.attribution":     {Description: "Attribution text rendered in the map corner."},

	// --- search.* -----------------------------------------------------
	"search.provider": {
		Description: "Geocoding backend.",
		Enum:        stringEnum("pelias"),
	},
	"search.resultLimit":              {Min: floatP(1), Max: floatP(50)},
	"search.biasToInstalledRegions":   {Description: "Prefer results inside installed regions."},
	"search.showHistory":              {Description: "Surface recent queries in the search panel."},
	"search.historyRetentionDays":     {Min: floatP(1), Max: floatP(3650), Unit: "days"},
	"search.autocompleteDebounceMs":   {Min: floatP(0), Max: floatP(2000), Unit: "ms"},
	"search.peliasElasticUrl":         {Description: "Pelias Elasticsearch endpoint (worker-only)."},
	"search.peliasLanguages":          {Description: "Languages to import into the Pelias index.", ItemType: TypeString},
	"search.peliasPolylinesEnabled":   {Description: "Enable Pelias polyline layer (heavier index)."},
	"search.peliasBuildTimeoutMinutes": {Min: floatP(1), Max: floatP(1440), Unit: "minutes"},
	"search.peliasImporterImage":      {Description: "Docker image used by the Pelias importer."},

	// --- routing.* ----------------------------------------------------
	"routing.provider": {
		Description: "Routing backend.",
		Enum:        stringEnum("valhalla"),
	},
	"routing.defaultMode": {
		Description: "Default travel mode.",
		Enum:        stringEnum("auto", "bicycle", "pedestrian", "truck"),
	},
	"routing.avoidHighways":        {},
	"routing.avoidTolls":           {},
	"routing.avoidFerries":         {},
	"routing.maxAlternatives":      {Min: floatP(0), Max: floatP(5)},
	"routing.truck.heightMeters":   {Min: floatP(1.5), Max: floatP(5), Step: floatP(0.1), Unit: "m"},
	"routing.truck.widthMeters":    {Min: floatP(1), Max: floatP(3.5), Step: floatP(0.1), Unit: "m"},
	"routing.truck.weightTons":     {Min: floatP(0.5), Max: floatP(50), Step: floatP(0.1), Unit: "t"},
	"routing.truck.lengthMeters":   {Min: floatP(2), Max: floatP(25), Step: floatP(0.1), Unit: "m"},
	"routing.valhallaConcurrency": {
		Description: "Build-time CPU concurrency. 0 uses runtime.NumCPU() capped to 8.",
		Min:         floatP(0), Max: floatP(64),
	},
	"routing.valhallaBuildTimeoutMinutes": {Min: floatP(1), Max: floatP(1440), Unit: "minutes"},
	"routing.valhallaExtraArgs":          {ItemType: TypeString, Description: "Extra CLI args passed to valhalla_build_tiles."},
	"routing.valhallaTileDirName":        {Description: "Subdirectory inside the region dir that holds the routing graph."},
	"routing.activeRegion": {
		Description: "Canonical region key whose Valhalla tiles serve routing requests. Empty falls back to the largest available tile archive.",
	},

	// --- tiles.* ------------------------------------------------------
	"tiles.source": {
		Description: "Vector tile source.",
		Enum:        stringEnum("protomaps"),
	},
	"tiles.cacheTTLSeconds":        {Min: floatP(0), Max: floatP(31536000), Unit: "s"},
	"tiles.diskCacheBytes":         {Min: floatP(0), Unit: "bytes"},
	"tiles.memoryCacheItems":       {Min: floatP(0), Max: floatP(1000000)},
	"tiles.planetilerJarURL":       {Description: "URL to the planetiler.jar release artefact."},
	"tiles.planetilerJarSha256":    {Description: "SHA-256 of the planetiler.jar. Empty = fail-closed in prod."},
	"tiles.planetilerMemoryMB":     {Min: floatP(1024), Max: floatP(65536), Unit: "MB"},
	"tiles.planetilerExtraArgs":    {ItemType: TypeString},
	"tiles.planetilerMaxDurationMinutes": {Min: floatP(1), Max: floatP(2880), Unit: "minutes"},
	"tiles.planetilerRetryCount":   {Min: floatP(0), Max: floatP(10)},

	// --- pois.* -------------------------------------------------------
	"pois.sources":             {ItemType: TypeString, Description: "POI source layers merged into the lookup index."},
	"pois.defaultRadiusMeters": {Min: floatP(1), Max: floatP(50000), Unit: "m"},
	"pois.categoryDefaults":    {ItemType: TypeString, Description: "Categories pre-selected in the POI filter panel."},

	// --- regions.* ----------------------------------------------------
	"regions.catalogURL": {Description: "Geofabrik catalogue JSON URL."},
	"regions.mirrorBase": {Description: "Base URL for Geofabrik region downloads."},
	"regions.defaultSchedule": {
		Description: "Default update cadence for new regions.",
		Enum:        stringEnum("never", "daily", "weekly", "monthly"),
	},
	"regions.updateCheckCron":        {Description: "Cron expression for the nightly update sweep."},
	"regions.retryAttempts":          {Min: floatP(0), Max: floatP(20)},
	"regions.retryBackoffSeconds":    {Min: floatP(0), Max: floatP(3600), Unit: "s"},
	"regions.downloadBandwidthBytes": {Min: floatP(0), Unit: "bytes/s"},
	"regions.keepSourcePbf":          {Description: "Keep the source pbf for incremental updates."},
	"regions.archivedRetentionDays":  {Min: floatP(0), Max: floatP(3650), Unit: "days"},
	"regions.maxConcurrentBuilds":    {Min: floatP(1), Max: floatP(16)},

	// --- share.* ------------------------------------------------------
	"share.shortLinkTTLDays":       {Min: floatP(1), Max: floatP(3650), Unit: "days"},
	"share.ogBaseURL":              {Description: "Public base URL used when rendering OG previews."},
	"share.ogRenderTimeoutSeconds": {Min: floatP(1), Max: floatP(120), Unit: "s"},
	"share.ogCacheTTLSeconds":      {Min: floatP(0), Max: floatP(31536000), Unit: "s"},
	"share.embedAllowedOrigins":    {ItemType: TypeString, Description: "CORS/iframe origin allow-list. `*` permits any origin."},
	"share.embedUIBaseURL":         {Description: "When set, /embed redirects to <baseURL>/embed?..."},
	"share.qrCodeSizePx":           {Min: floatP(64), Max: floatP(2048), Unit: "px"},

	// --- rateLimit.* --------------------------------------------------
	"rateLimit.tilesPerMinutePerIP":     {Min: floatP(0), Unit: "req/min"},
	"rateLimit.geocodePerMinutePerIP":   {Min: floatP(0), Unit: "req/min"},
	"rateLimit.routePerMinutePerIP":     {Min: floatP(0), Unit: "req/min"},
	"rateLimit.regionsAdminPerMinute":   {Min: floatP(0), Unit: "req/min"},
	"rateLimit.ogPreviewPerMinutePerIP": {Min: floatP(0), Unit: "req/min"},
	"rateLimit.bypassAuthenticated":     {Description: "Skip rate limits for authenticated users."},

	// --- auth.* -------------------------------------------------------
	// Native session-cookie authentication. Users live in the `users`
	// SQL table (bcrypt-hashed passwords); these are just tunables.
	"auth.publicReadOnly":               {Description: "Allow anonymous GETs on map/search/route."},
	"auth.sessionTTLHours":              {Min: floatP(1), Max: floatP(8760), Unit: "hours", Description: "Lifetime of a login session cookie."},
	"auth.cookieName":                   {Description: "Name of the session cookie set on the browser."},
	"auth.cookieSecure":                 {Description: "Set the `Secure` attribute on the session cookie. Disable only for plain-HTTP dev."},
	"auth.passwordMinLength":            {Min: floatP(4), Max: floatP(256), Description: "Minimum length for a new user password."},
	"auth.rateLimit.loginPerMinutePerIP": {Min: floatP(0), Unit: "req/min", Description: "Per-IP login attempt rate cap."},

	// --- security.* ---------------------------------------------------
	"security.allowedEgressHosts": {ItemType: TypeString, Description: "Outbound hosts the worker may fetch artefacts from."},

	// --- telemetry.* --------------------------------------------------
	"telemetry.enabled":    {Description: "Emit telemetry events to the configured endpoint."},
	"telemetry.endpoint":   {Description: "OTLP or statsd-compatible endpoint URL."},
	"telemetry.sampleRate": {Min: floatP(0), Max: floatP(1), Step: floatP(0.01)},

	// --- ui.* ---------------------------------------------------------
	"ui.appName":                 {Description: "Product name shown in the UI chrome."},
	"ui.brandColor":              {Description: "Primary accent colour (hex).", Pattern: "^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$"},
	"ui.favicon":                 {Description: "Absolute or relative URL of the favicon."},
	"ui.showAdminLink":           {Description: "Show the Admin entry in the navigation."},
	"ui.sidebarCollapsedDefault": {Description: "Collapse the sidebar by default."},
}
