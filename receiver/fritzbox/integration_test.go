package fritzbox

import (
	"context"
	"encoding/pem"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/mbaykara/otelcol-fritzbox/internal/fakebox"
	"github.com/mbaykara/otelcol-fritzbox/receiver/fritzbox/internal/metadata"
)

// startIntegrationReceiver runs the receiver built by the factory against
// box through the real scraper controller and returns the metrics sink.
func startIntegrationReceiver(t *testing.T, box *fakebox.Box, configure func(*Config)) *consumertest.MetricsSink {
	t.Helper()
	srv := httptest.NewServer(box)
	t.Cleanup(srv.Close)

	cfg := NewFactory().CreateDefaultConfig().(*Config)
	cfg.Endpoint = srv.URL
	cfg.Username = box.Username
	cfg.Password = configopaque.String(box.Password)
	cfg.ScraperControllerSettings.CollectionInterval = 200 * time.Millisecond
	cfg.ScraperControllerSettings.InitialDelay = 0
	cfg.ScraperControllerSettings.Timeout = 200 * time.Millisecond
	if configure != nil {
		configure(cfg)
	}
	require.NoError(t, cfg.Validate())

	sink := new(consumertest.MetricsSink)
	rcv, err := NewFactory().CreateMetrics(context.Background(), receivertest.NewNopSettings(metadata.Type), cfg, sink)
	require.NoError(t, err)
	require.NoError(t, rcv.Start(context.Background(), componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, rcv.Shutdown(context.Background())) })
	return sink
}

// waitForScrapes blocks until the sink has received at least n batches.
func waitForScrapes(t *testing.T, sink *consumertest.MetricsSink, n int) []pmetric.Metrics {
	t.Helper()
	require.Eventually(t, func() bool { return len(sink.AllMetrics()) >= n },
		10*time.Second, 10*time.Millisecond, "receiver did not export %d batches", n)
	return sink.AllMetrics()
}

func metricNames(m pmetric.Metrics) map[string]bool {
	names := map[string]bool{}
	rms := m.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		sms := rms.At(i).ScopeMetrics()
		for j := 0; j < sms.Len(); j++ {
			ms := sms.At(j).Metrics()
			for k := 0; k < ms.Len(); k++ {
				names[ms.At(k).Name()] = true
			}
		}
	}
	return names
}

func TestIntegrationDSLBox(t *testing.T) {
	box := fakebox.NewDSL()
	sink := startIntegrationReceiver(t, box, func(cfg *Config) {
		cfg.MetricsBuilderConfig.Metrics.FritzboxHostsInfo.Enabled = true
	})
	batches := waitForScrapes(t, sink, 2)

	names := metricNames(batches[len(batches)-1])
	for _, want := range []string{
		"fritzbox.device.uptime",
		"fritzbox.dsl.rate.current",
		"fritzbox.dsl.noise_margin",
		"fritzbox.hosts.total",
		"fritzbox.hosts.info",
		"fritzbox.wan.connection.status",
		"fritzbox.wlan.clients",
		"hw.network.io",
		"hw.network.up",
	} {
		assert.True(t, names[want], "metric %s missing from export", want)
	}
	assert.False(t, names["fritzbox.wan.external_ip"], "opt-in metric exported by default")

	res := batches[0].ResourceMetrics().At(0).Resource().Attributes()
	model, ok := res.Get("hw.model")
	if assert.True(t, ok, "resource attribute hw.model missing") {
		assert.Equal(t, "FRITZ!Box 7590", model.Str())
	}

	stats := box.Stats()
	assert.Positive(t, stats.Authorized, "no digest-authenticated calls")
	assert.Zero(t, stats.Rejected, "device rejected credentials")
	assert.Positive(t, stats.HostListFetches, "host list not fetched")
	// Challenges are cached: after the first round trip, calls authenticate
	// preemptively instead of paying one 401 per call.
	assert.Less(t, stats.Challenges, stats.Authorized/2, "digest challenge not reused")
}

// A faulting metric group must not suppress metrics from other groups once
// the controller has processed the partial scrape error.
func TestIntegrationPartialFailureKeepsOtherGroups(t *testing.T) {
	box := fakebox.NewDSL()
	box.Faults = map[string]int{
		"urn:dslforum-org:service:WANDSLInterfaceConfig:1#GetInfo": 820,
	}
	sink := startIntegrationReceiver(t, box, nil)
	batches := waitForScrapes(t, sink, 1)

	names := metricNames(batches[0])
	assert.False(t, names["fritzbox.dsl.rate.current"], "faulted group exported data")
	assert.True(t, names["fritzbox.hosts.total"], "unaffected group dropped")
	assert.True(t, names["hw.network.io"], "unaffected group dropped")
}

func TestIntegrationNonceRotation(t *testing.T) {
	box := fakebox.NewDSL()
	sink := startIntegrationReceiver(t, box, nil)
	waitForScrapes(t, sink, 1)

	box.RotateNonce()
	before := len(sink.AllMetrics())
	waitForScrapes(t, sink, before+2)

	names := metricNames(sink.AllMetrics()[len(sink.AllMetrics())-1])
	assert.True(t, names["fritzbox.dsl.rate.current"], "authenticated group lost after nonce rotation")
	assert.Zero(t, box.Stats().Rejected)
}

func TestIntegrationWithoutCredentials(t *testing.T) {
	box := fakebox.NewDSL()
	sink := startIntegrationReceiver(t, box, func(cfg *Config) {
		cfg.Username = ""
		cfg.Password = ""
	})
	batches := waitForScrapes(t, sink, 1)

	names := metricNames(batches[0])
	assert.True(t, names["fritzbox.hosts.total"], "unauthenticated group missing")
	assert.False(t, names["fritzbox.dsl.rate.current"], "authenticated group exported without credentials")
	assert.Zero(t, box.Stats().Authorized)
}

func TestIntegrationDeviceUnavailableAtStart(t *testing.T) {
	box := fakebox.NewDSL()
	box.SetDown(true)
	sink := startIntegrationReceiver(t, box, nil)

	// Discovery fails while the device is down; startup must still succeed
	// and no batch may claim data it could not collect.
	require.Eventually(t, func() bool { return box.Stats().DescriptionFetches >= 2 },
		10*time.Second, 10*time.Millisecond, "discovery not retried")
	for _, m := range sink.AllMetrics() {
		assert.Zero(t, m.DataPointCount(), "metrics exported while device was down")
	}

	box.SetDown(false)
	require.Eventually(t, func() bool {
		all := sink.AllMetrics()
		return len(all) > 0 && metricNames(all[len(all)-1])["fritzbox.hosts.total"]
	}, 10*time.Second, 10*time.Millisecond, "receiver did not recover after device came back")
}

// writeServerCA stores the test server's certificate as a PEM file, the way
// a user stores the certificate exported from the device.
func writeServerCA(t *testing.T, srv *httptest.Server) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fritzbox.pem")
	block := &pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw}
	require.NoError(t, os.WriteFile(path, pem.EncodeToMemory(block), 0o600))
	return path
}

func TestIntegrationHTTPSWithDeviceCertificate(t *testing.T) {
	box := fakebox.NewDSL()
	srv := httptest.NewTLSServer(box)
	t.Cleanup(srv.Close)

	cfg := NewFactory().CreateDefaultConfig().(*Config)
	cfg.Endpoint = srv.URL
	cfg.Username = box.Username
	cfg.Password = configopaque.String(box.Password)
	cfg.TLS.CAFile = writeServerCA(t, srv)
	cfg.ScraperControllerSettings.CollectionInterval = 200 * time.Millisecond
	cfg.ScraperControllerSettings.InitialDelay = 0
	cfg.ScraperControllerSettings.Timeout = 200 * time.Millisecond
	require.NoError(t, cfg.Validate())

	sink := new(consumertest.MetricsSink)
	rcv, err := NewFactory().CreateMetrics(context.Background(), receivertest.NewNopSettings(metadata.Type), cfg, sink)
	require.NoError(t, err)
	require.NoError(t, rcv.Start(context.Background(), componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, rcv.Shutdown(context.Background())) })

	batches := waitForScrapes(t, sink, 1)
	assert.True(t, metricNames(batches[0])["fritzbox.dsl.rate.current"], "authenticated group missing over https")
}

func TestIntegrationHTTPSRejectsUnknownCertificate(t *testing.T) {
	box := fakebox.NewDSL()
	srv := httptest.NewTLSServer(box)
	t.Cleanup(srv.Close)

	cfg := NewFactory().CreateDefaultConfig().(*Config)
	cfg.Endpoint = srv.URL
	cfg.ScraperControllerSettings.CollectionInterval = 200 * time.Millisecond
	cfg.ScraperControllerSettings.InitialDelay = 0
	cfg.ScraperControllerSettings.Timeout = 200 * time.Millisecond

	sink := new(consumertest.MetricsSink)
	rcv, err := NewFactory().CreateMetrics(context.Background(), receivertest.NewNopSettings(metadata.Type), cfg, sink)
	require.NoError(t, err)
	require.NoError(t, rcv.Start(context.Background(), componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, rcv.Shutdown(context.Background())) })

	// A failed scrape still delivers an empty batch, so two batches mean two
	// completed attempts.
	waitForScrapes(t, sink, 2)
	assert.Zero(t, box.Stats().DescriptionFetches, "request reached the device over an unverified TLS connection")
	for _, m := range sink.AllMetrics() {
		assert.Zero(t, m.DataPointCount())
	}
}
