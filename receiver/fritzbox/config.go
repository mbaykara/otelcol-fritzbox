package fritzbox

import (
	"errors"
	"fmt"
	"net/url"
	"time"

	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/mbaykara/otelcol-fritzbox/receiver/fritzbox/internal/metadata"
)

// Config represents the receiver config settings in the Collector config.yaml.
type Config struct {
	// Endpoint is the base URL of the Fritz!Box TR-064 API, either
	// "http://fritz.box:49000" or "https://fritz.box:49443".
	Endpoint string `mapstructure:"endpoint"`
	// Username for TR-064 digest authentication. Optional: actions that do
	// not require authentication (e.g. host counts) are still collected.
	Username string `mapstructure:"username"`
	// Password for TR-064 digest authentication. Redacted when the
	// configuration is printed.
	Password configopaque.String `mapstructure:"password"`
	// TLS configures certificate verification for an https endpoint. The
	// device uses a self-signed certificate: set ca_file to the certificate
	// exported from the device. Ignored settings on an http endpoint are
	// rejected by Validate.
	TLS configtls.ClientConfig `mapstructure:"tls"`

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
	switch u.Scheme {
	case "https":
		if err := cfg.TLS.Validate(); err != nil {
			return fmt.Errorf("tls: %w", err)
		}
	case "http":
		if cfg.TLS.CAFile != "" || cfg.TLS.CAPem != "" || cfg.TLS.ServerName != "" || cfg.TLS.InsecureSkipVerify {
			return fmt.Errorf("tls settings require an https endpoint, got %q", cfg.Endpoint)
		}
	default:
		return fmt.Errorf("endpoint %q must use http or https", cfg.Endpoint)
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
		TLS:                       configtls.NewDefaultClientConfig(),
		ScraperControllerSettings: controllerCfg,
		MetricsBuilderConfig:      metadata.NewDefaultMetricsBuilderConfig(),
	}
}
