package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/otelcol"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"

	"github.com/mbaykara/otelcol-fritzbox/internal/fakebox"
)

// otlpSink is an OTLP/HTTP metrics endpoint that records received metric
// names.
type otlpSink struct {
	mu    sync.Mutex
	names map[string]int
}

func (s *otlpSink) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	req := pmetricotlp.NewExportRequest()
	if err := req.UnmarshalProto(body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	rms := req.Metrics().ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		sms := rms.At(i).ScopeMetrics()
		for j := 0; j < sms.Len(); j++ {
			ms := sms.At(j).Metrics()
			for k := 0; k < ms.Len(); k++ {
				s.names[ms.At(k).Name()]++
			}
		}
	}
	s.mu.Unlock()

	resp, err := pmetricotlp.NewExportResponse().MarshalProto()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/x-protobuf")
	_, _ = w.Write(resp)
}

func (s *otlpSink) has(name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.names[name] > 0
}

// TestCollectorEndToEnd runs the distribution against a fake Fritz!Box and
// checks that metrics reach an OTLP/HTTP backend.
func TestCollectorEndToEnd(t *testing.T) {
	box := fakebox.NewDSL()
	device := httptest.NewServer(box)
	t.Cleanup(device.Close)
	sink := &otlpSink{names: map[string]int{}}
	backend := httptest.NewServer(sink)
	t.Cleanup(backend.Close)

	healthAddr := freeAddr(t)
	cfg := fmt.Sprintf(`
extensions:
  health_check:
    endpoint: %s
receivers:
  fritzbox:
    endpoint: %s
    username: %s
    password: %s
    collection_interval: 200ms
    initial_delay: 0s
    timeout: 200ms
processors:
  memory_limiter:
    check_interval: 1s
    limit_mib: 200
  batch:
    timeout: 100ms
exporters:
  otlp_http:
    endpoint: %s
    compression: none
    retry_on_failure:
      enabled: false
service:
  extensions: [health_check]
  telemetry:
    logs:
      level: warn
    metrics:
      level: none
  pipelines:
    metrics:
      receivers: [fritzbox]
      processors: [memory_limiter, batch]
      exporters: [otlp_http]
`, healthAddr, device.URL, box.Username, box.Password, backend.URL)
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(cfg), 0o600))

	set := settings()
	set.ConfigProviderSettings.ResolverSettings.URIs = []string{"file:" + path}
	col, err := otelcol.NewCollector(set)
	require.NoError(t, err)

	done := make(chan error, 1)
	go func() { done <- col.Run(context.Background()) }()
	t.Cleanup(func() {
		col.Shutdown()
		select {
		case err := <-done:
			assert.NoError(t, err)
		case <-time.After(10 * time.Second):
			t.Error("collector did not shut down")
		}
	})

	require.Eventually(t, func() bool {
		return sink.has("fritzbox.dsl.rate.current") && sink.has("hw.network.io") && sink.has("fritzbox.hosts.total")
	}, 20*time.Second, 50*time.Millisecond, "metrics did not reach the OTLP backend")
	assert.Positive(t, box.Stats().Authorized)

	require.Eventually(t, func() bool {
		resp, err := http.Get("http://" + healthAddr + "/")
		if err != nil {
			return false
		}
		_ = resp.Body.Close()
		return resp.StatusCode == http.StatusOK
	}, 10*time.Second, 50*time.Millisecond, "health_check did not report healthy")
}

// freeAddr returns a loopback address with a currently unused port.
func freeAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := l.Addr().String()
	require.NoError(t, l.Close())
	return addr
}

// TestExampleConfigsValidate checks that every example collector config is
// accepted by this distribution's component set.
func TestExampleConfigsValidate(t *testing.T) {
	t.Setenv("FRITZBOX_USERNAME", "user")
	t.Setenv("FRITZBOX_PASSWORD", "password")
	t.Setenv("OTLP_ENDPOINT", "https://otlp.example.com/otlp")
	t.Setenv("OTLP_USERNAME", "123456")
	t.Setenv("OTLP_PASSWORD", "token")

	paths, err := filepath.Glob("../../example/collector-config*.yaml")
	require.NoError(t, err)
	require.NotEmpty(t, paths)
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			cmd := otelcol.NewCommand(settings())
			cmd.SetArgs([]string{"validate", "--config=file:" + path})
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			require.NoError(t, cmd.Execute())
		})
	}
}
