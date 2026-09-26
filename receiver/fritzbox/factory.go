package fritzbox

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/mbaykara/otelcol-fritzbox/receiver/fritzbox/internal/metadata"
	"github.com/mbaykara/otelcol-fritzbox/receiver/fritzbox/internal/tr064"
)

// NewFactory creates a factory for the fritzbox receiver.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		func() component.Config { return defaultConfig() },
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability),
	)
}

func createMetricsReceiver(
	_ context.Context,
	settings receiver.Settings,
	baseCfg component.Config,
	nextConsumer consumer.Metrics,
) (receiver.Metrics, error) {
	cfg, ok := baseCfg.(*Config)
	if !ok {
		return nil, fmt.Errorf("fritzbox: invalid config type %T", baseCfg)
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("fritzbox: %w", err)
	}

	s := newScraper(cfg, settings)
	sc, err := scraper.NewMetrics(s.scrape,
		scraper.WithStart(s.start),
		scraper.WithShutdown(s.shutdown),
	)
	if err != nil {
		return nil, fmt.Errorf("fritzbox: creating scraper: %w", err)
	}

	return scraperhelper.NewMetricsController(
		&cfg.ScraperControllerSettings,
		settings,
		nextConsumer,
		scraperhelper.AddMetricsScraper(metadata.Type, sc),
	)
}

// tr064Client is the subset of the TR-064 client used by the scraper.
// It is an interface to allow fakes in tests.
type tr064Client interface {
	Services(ctx context.Context) ([]tr064.Service, error)
	Call(ctx context.Context, serviceType, controlURL, action string) (map[string]string, error)
	CallWithArgs(ctx context.Context, serviceType, controlURL, action string, args map[string]string) (map[string]string, error)
	FetchURL(ctx context.Context, path string) ([]byte, error)
}

// newTR064Client builds the real TR-064 client from config. TLS settings
// apply only to https endpoints; Validate rejects them for http.
func newTR064Client(ctx context.Context, cfg *Config) (tr064Client, error) {
	transport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return nil, errors.New("fritzbox: unexpected default HTTP transport type")
	}
	transport = transport.Clone()
	if strings.HasPrefix(cfg.Endpoint, "https://") {
		tlsCfg, err := cfg.TLS.LoadTLSConfig(ctx)
		if err != nil {
			return nil, fmt.Errorf("fritzbox: %w", err)
		}
		transport.TLSClientConfig = tlsCfg
	}
	httpClient := &http.Client{Timeout: cfg.ScraperControllerSettings.Timeout, Transport: transport}
	if httpClient.Timeout <= 0 {
		httpClient.Timeout = 10 * time.Second
	}
	client, err := tr064.NewClient(cfg.Endpoint, cfg.Username, string(cfg.Password), httpClient)
	if err != nil {
		return nil, fmt.Errorf("fritzbox: %w", err)
	}
	return &closeTrackingClient{Client: client, httpClient: httpClient}, nil
}

// closeTrackingClient wraps the TR-064 client and closes idle HTTP
// connections on shutdown so no dial goroutines outlive the receiver.
type closeTrackingClient struct {
	*tr064.Client
	httpClient *http.Client
}

// CloseIdleConnections releases idle keep-alive connections.
func (c *closeTrackingClient) CloseIdleConnections() {
	c.httpClient.CloseIdleConnections()
}

var errNoWANConnectionService = errors.New("no WANIPConnection or WANPPPConnection service found")
