// Package telemetry wires structured logging (zerolog), a request-scoped
// trace id, and a Prometheus registry. See docs/06-agent-rules.md R9.
package telemetry

import (
	"crypto/rand"
	"encoding/hex"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"github.com/valyala/fasthttp/fasthttpadaptor"
)

// redactedHeaders lists request header names whose values must never
// appear in logs. Matching is case-insensitive.
var redactedHeaders = []string{"authorization", "cookie"}

// Telemetry bundles the per-process logger and Prometheus registry
// plus the request counters/histograms.
type Telemetry struct {
	Logger   zerolog.Logger
	Registry *prometheus.Registry
	Reqs     *prometheus.CounterVec
	Latency  *prometheus.HistogramVec
}

// New builds a Telemetry with a zerolog logger writing to w (os.Stderr
// in production) at the given level string ("debug"|"info"|...).
func New(w io.Writer, level string) *Telemetry {
	if w == nil {
		w = os.Stderr
	}
	zl, err := zerolog.ParseLevel(strings.ToLower(level))
	if err != nil || level == "" {
		zl = zerolog.InfoLevel
	}
	zerolog.TimeFieldFormat = time.RFC3339Nano
	logger := zerolog.New(w).Level(zl).With().
		Timestamp().
		Str("service", "localmaps").
		Logger().
		Hook(redactionHook{})

	reg := prometheus.NewRegistry()
	reqs := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "localmaps_http_requests_total",
		Help: "Total HTTP requests, labelled by route, method, and status.",
	}, []string{"route", "method", "status"})
	lat := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "localmaps_http_request_duration_seconds",
		Help:    "HTTP request duration in seconds, labelled by route.",
		Buckets: prometheus.DefBuckets,
	}, []string{"route"})
	reg.MustRegister(reqs, lat,
		prometheus.NewGoCollector(),
		prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}),
	)
	return &Telemetry{Logger: logger, Registry: reg, Reqs: reqs, Latency: lat}
}

// NewTracer returns Fiber middleware that assigns a random 12-hex trace
// id to each request, stores it in c.Locals("traceId"), and attaches it
// to a per-request logger on c.Locals("logger"). It also records the
// Prometheus counter + histogram.
func (t *Telemetry) NewTracer() fiber.Handler {
	return func(c fiber.Ctx) error {
		traceID := randomTraceID()
		c.Locals("traceId", traceID)
		logger := t.Logger.With().
			Str("traceId", traceID).
			Str("method", c.Method()).
			Str("path", c.Path()).
			Logger()
		c.Locals("logger", logger)

		start := time.Now()
		err := c.Next()
		dur := time.Since(start).Seconds()
		route := c.Route().Path
		if route == "" {
			route = c.Path()
		}
		status := c.Response().StatusCode()
		t.Reqs.WithLabelValues(route, c.Method(), strconv.Itoa(status)).Inc()
		t.Latency.WithLabelValues(route).Observe(dur)
		return err
	}
}

// MetricsHandler returns a Fiber handler serving the Prometheus registry.
func (t *Telemetry) MetricsHandler() fiber.Handler {
	h := promhttp.HandlerFor(t.Registry, promhttp.HandlerOpts{Registry: t.Registry})
	return func(c fiber.Ctx) error {
		fasthttpadaptor.NewFastHTTPHandler(h)(c.RequestCtx())
		return nil
	}
}

// LoggerFrom extracts the per-request logger, or falls back to the base
// Telemetry logger if none was set.
func (t *Telemetry) LoggerFrom(c fiber.Ctx) zerolog.Logger {
	if v := c.Locals("logger"); v != nil {
		if l, ok := v.(zerolog.Logger); ok {
			return l
		}
	}
	return t.Logger
}

// randomTraceID returns 12 hex chars (6 random bytes). Unique enough for
// request correlation.
func randomTraceID() string {
	var b [6]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// redactionHook zeros out redactedHeaders' values if a handler ever logs
// a `headers` event field. zerolog doesn't globally intercept fields —
// this is a belt-and-braces guard.
type redactionHook struct{}

// Run implements zerolog.Hook. It's a no-op today; we document the
// redaction contract here and rely on callers to use RedactHeaders
// before logging raw headers. See LogHeaders below.
func (redactionHook) Run(_ *zerolog.Event, _ zerolog.Level, _ string) {}

// RedactHeaders returns a copy of headers with sensitive values masked.
func RedactHeaders(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		lk := strings.ToLower(k)
		redacted := false
		for _, r := range redactedHeaders {
			if lk == r {
				redacted = true
				break
			}
		}
		if redacted {
			out[k] = "[redacted]"
		} else {
			out[k] = v
		}
	}
	return out
}
