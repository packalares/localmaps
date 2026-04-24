package api

// Handlers for the `meta` tag in contracts/openapi.yaml. These are
// real Phase-1 implementations (liveness, readiness, version); the
// Prometheus `/metrics` handler is wired in router.go directly from
// the telemetry package.

import (
	"net"
	"net/url"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/packalares/localmaps/server/internal/config"
)

// GET /api/health — liveness.
func healthHandler(c fiber.Ctx) error {
	return c.JSON(fiber.Map{"ok": true})
}

// VersionInfo is the build metadata returned from /api/version. It's
// populated at link time via -ldflags by deploy/Dockerfile.*.
type VersionInfo struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	BuiltAt string `json:"builtAt"`
}

// versionHolder lets tests and main.go inject the build info once.
var (
	versionMu   sync.RWMutex
	currentInfo = VersionInfo{
		Version: "0.1.0",
		Commit:  "unknown",
		BuiltAt: "unknown",
	}
)

// SetVersion is called from cmd/localmaps/main.go with -ldflags values.
func SetVersion(v VersionInfo) {
	versionMu.Lock()
	defer versionMu.Unlock()
	currentInfo = v
}

// GET /api/version
func versionHandler(c fiber.Ctx) error {
	versionMu.RLock()
	defer versionMu.RUnlock()
	return c.JSON(currentInfo)
}

// readyDeps is the set of TCP endpoints the gateway dials to judge its
// readiness. Populated once at router build time from the boot config.
type readyDeps struct {
	redisURL     string
	protomapsURL string
	valhallaURL  string
	peliasURL    string
}

// newReadyHandler returns the /api/ready handler, which dials each
// backing service with a short timeout and reports the per-service
// status map defined in openapi.yaml (/api/ready response schema).
func newReadyHandler(boot *config.Boot) fiber.Handler {
	d := readyDeps{
		redisURL:     boot.RedisURL,
		protomapsURL: boot.ProtomapsURL,
		valhallaURL:  boot.ValhallaURL,
		peliasURL:    boot.PeliasURL,
	}
	return func(c fiber.Ctx) error {
		services := fiber.Map{
			"redis":    dialOK(d.redisURL),
			"pmtiles":  dialOK(d.protomapsURL),
			"valhalla": dialOK(d.valhallaURL),
			"pelias":   dialOK(d.peliasURL),
		}
		ok := true
		for _, v := range services {
			if v != true {
				ok = false
				break
			}
		}
		return c.JSON(fiber.Map{"ok": ok, "services": services})
	}
}

// dialOK TCP-dials the host:port of rawURL and returns true on success.
// redis://, http://, https:// are all understood; the port defaults
// to a scheme-appropriate one when absent.
func dialOK(rawURL string) bool {
	if rawURL == "" {
		return false
	}
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return false
	}
	host := u.Host
	// If no explicit port, pick one by scheme.
	if _, _, err := net.SplitHostPort(host); err != nil {
		switch u.Scheme {
		case "redis":
			host = net.JoinHostPort(host, "6379")
		case "https":
			host = net.JoinHostPort(host, "443")
		default:
			host = net.JoinHostPort(host, "80")
		}
	}
	conn, err := net.DialTimeout("tcp", host, 500*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}
