package fritzbox

import (
	"errors"
	"fmt"
	"net/url"
	"time"

	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/mbaykara/otelcol-fritzbox/receiver/fritzbox/internal/metadata"
)

// Config represents the receiver config settings in the Collector config.yaml.
type Config struct {
	// Endpoint is the base URL of the Fritz!Box TR-064 API,
	// e.g. "http://fritz.box:49000".
	Endpoint string `mapstructure:"endpoint"`
	// Username for TR-064 digest authentication. Optional: actions that do
	// not require authentication (e.g. host counts) are still collected.
	Username string `mapstructure:"username"`
	// Password for TR-064 digest authentication.
	Password string `mapstructure:"password"`

	// ScraperControllerSettings configures collection interval and timeout.
	ScraperControllerSettings scraperhelper.ControllerConfig `mapstructure:",squash"`
	// MetricsBuilderConfig allows enabling/disabling individual metrics.
	MetricsBuilderConfig metadata.MetricsBuilderConfig `mapstructure:",squash"`
}

// Validate checks if the receiver configuration is valid.
func (cfg *Config) Validate() error {
	u, err := url.Parse(cfg.Endpoint)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("endpoint %q must be an absolute URL", cfg.Endpoint)
	}
	if (cfg.Username == "") != (cfg.Password == "") {
		return errors.New("username and password must be both set or both empty")
	}
	if cfg.ScraperControllerSettings.CollectionInterval <= 0 {
		return errors.New("collection_interval must be positive")
	}
	if cfg.ScraperControllerSettings.Timeout < 0 {
		return errors.New("timeout must not be negative")
	}
	if cfg.ScraperControllerSettings.Timeout > cfg.ScraperControllerSettings.CollectionInterval {
		return fmt.Errorf("timeout (%s) must not exceed collection_interval (%s)",
			cfg.ScraperControllerSettings.Timeout, cfg.ScraperControllerSettings.CollectionInterval)
	}
	return nil
}

func defaultConfig() *Config {
	controllerCfg := scraperhelper.NewDefaultControllerConfig()
	controllerCfg.CollectionInterval = 30 * time.Second
	controllerCfg.Timeout = 10 * time.Second
	return &Config{
		Endpoint:                  "http://fritz.box:49000",
		ScraperControllerSettings: controllerCfg,
		MetricsBuilderConfig:      metadata.DefaultMetricsBuilderConfig(),
	}
}
