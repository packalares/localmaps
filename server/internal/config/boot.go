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
	RedisURL           string `env:"LOCALMAPS_REDIS_URL"           envDefault:"redis://redis:6379/0"`
	ProtomapsURL       string `env:"LOCALMAPS_PROTOMAPS_URL"       envDefault:"http://protomaps:8000"`
	ValhallaURL        string `env:"LOCALMAPS_VALHALLA_URL"        envDefault:"http://valhalla:8002"`
	PeliasURL          string `env:"LOCALMAPS_PELIAS_URL"          envDefault:"http://pelias-api:4000"`
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
