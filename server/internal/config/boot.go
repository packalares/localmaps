package config

import (
	"fmt"

	"github.com/caarlos0/env/v11"
)

// Boot holds the env-driven boot-time configuration from
// docs/07-config-schema.md. Everything else lives in SQLite. Adding a
// new field here requires a matching row in that doc.
type Boot struct {
	DataDir            string `env:"LOCALMAPS_DATA_DIR"            envDefault:"/data"`
	ListenAddr         string `env:"LOCALMAPS_LISTEN_ADDR"         envDefault:":8080"`
	// Sidecars run in the same k8s pod and bind to 127.0.0.1 only — the
	// legacy docker-compose service names (`redis`, `protomaps`, `valhalla`,
	// `pelias-api`, `pelias-es`) don't resolve here. Defaults match the
	// chart's configmap so dev / `helm template` runs work without env.
	RedisURL           string `env:"LOCALMAPS_REDIS_URL"           envDefault:"redis://127.0.0.1:6379/0"`
	ProtomapsURL       string `env:"LOCALMAPS_PROTOMAPS_URL"       envDefault:"http://127.0.0.1:8000"`
	ValhallaURL        string `env:"LOCALMAPS_VALHALLA_URL"        envDefault:"http://127.0.0.1:8002"`
	PeliasURL          string `env:"LOCALMAPS_PELIAS_URL"          envDefault:"http://127.0.0.1:4000"`
	PeliasESURL        string `env:"LOCALMAPS_PELIAS_ES_URL"       envDefault:"http://127.0.0.1:9200"`
	Mode               string `env:"LOCALMAPS_MODE"                envDefault:"gateway"`
	LogLevel           string `env:"LOCALMAPS_LOG_LEVEL"           envDefault:"info"`
	TrustProxyHeaders  bool   `env:"LOCALMAPS_TRUST_PROXY_HEADERS" envDefault:"false"`
}

// LoadBoot reads Boot from the process environment.
func LoadBoot() (*Boot, error) {
	var b Boot
	if err := env.Parse(&b); err != nil {
		return nil, fmt.Errorf("parse boot env: %w", err)
	}
	return &b, nil
}

// IsWorker reports whether the process should run in worker mode.
func (b *Boot) IsWorker() bool { return b.Mode == "worker" }
