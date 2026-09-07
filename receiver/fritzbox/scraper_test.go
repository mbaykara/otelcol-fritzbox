package fritzbox

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/mbaykara/otelcol-fritzbox/receiver/fritzbox/internal/metadata"
	"github.com/mbaykara/otelcol-fritzbox/receiver/fritzbox/internal/tr064"
)

// fakeTR064 is a scripted TR-064 client. Responses are keyed by
// "serviceType#action"; missing keys return a SOAP-fault-like error.
// fetchBody is returned by FetchURL regardless of path when non-nil.
type fakeTR064 struct {
	services  []tr064.Service
	responses map[string]map[string]string
	calls     []string
	fetchBody []byte
}

func (f *fakeTR064) Services(_ context.Context) ([]tr064.Service, error) {
	return f.services, nil
}

func (f *fakeTR064) Call(_ context.Context, serviceType, _, action string) (map[string]string, error) {
	return f.respond(serviceType, action)
}

func (f *fakeTR064) CallWithArgs(_ context.Context, serviceType, _, action string, _ map[string]string) (map[string]string, error) {
	return f.respond(serviceType, action)
}

func (f *fakeTR064) FetchURL(_ context.Context, _ string) ([]byte, error) {
	if f.fetchBody == nil {
		return nil, errors.New("no fetch body configured")
	}
	return f.fetchBody, nil
}

func (f *fakeTR064) respond(serviceType, action string) (map[string]string, error) {
	key := serviceType + "#" + action
	f.calls = append(f.calls, key)
	if resp, ok := f.responses[key]; ok {
		return resp, nil
	}
	return nil, &tr064.Error{Code: 401, Description: "Invalid Action"}
}

// dslBoxServices models a DSL Fritz!Box (like the dev box).
var dslBoxServices = []tr064.Service{
	{Type: "urn:dslforum-org:service:DeviceInfo:1", ControlURL: "/upnp/control/deviceinfo"},
	{Type: "urn:dslforum-org:service:WANCommonInterfaceConfig:1", ControlURL: "/upnp/control/wancommonifconfig1"},
	{Type: "urn:dslforum-org:service:WANDSLInterfaceConfig:1", ControlURL: "/upnp/control/wandslifconfig1"},
	{Type: "urn:dslforum-org:service:WANIPConnection:1", ControlURL: "/upnp/control/wanipconn1"},
	{Type: "urn:dslforum-org:service:WLANConfiguration:1", ControlURL: "/upnp/control/wlanconfig1"},
	{Type: "urn:dslforum-org:service:WLANConfiguration:2", ControlURL: "/upnp/control/wlanconfig2"},
	{Type: "urn:dslforum-org:service:WLANConfiguration:3", ControlURL: "/upnp/control/wlanconfig3"},
	{Type: "urn:dslforum-org:service:Hosts:1", ControlURL: "/upnp/control/hosts"},
}

func dslBoxResponses() map[string]map[string]string {
	return map[string]map[string]string{
		"urn:dslforum-org:service:DeviceInfo:1#GetInfo": {
			"NewModelName":       "FRITZ!Box 7590",
			"NewSerialNumber":    "X123456789012",
			"NewSoftwareVersion": "154.08.25",
			"NewUpTime":          "86400",
		},
		"urn:dslforum-org:service:WANCommonInterfaceConfig:1#GetCommonLinkProperties": {
			"NewPhysicalLinkStatus":         "Up",
			"NewLayer1DownstreamMaxBitRate": "250000000",
			"NewLayer1UpstreamMaxBitRate":   "50000000",
		},
		"urn:dslforum-org:service:WANCommonInterfaceConfig:1#GetTotalBytesSent":       {"NewTotalBytesSent": "1000000"},
		"urn:dslforum-org:service:WANCommonInterfaceConfig:1#GetTotalBytesReceived":   {"NewTotalBytesReceived": "5000000"},
		"urn:dslforum-org:service:WANCommonInterfaceConfig:1#GetTotalPacketsSent":     {"NewTotalPacketsSent": "8000"},
		"urn:dslforum-org:service:WANCommonInterfaceConfig:1#GetTotalPacketsReceived": {"NewTotalPacketsReceived": "9000"},
		"urn:dslforum-org:service:WANIPConnection:1#GetStatusInfo": {
			"NewConnectionStatus": "Connected",
			"NewUptime":           "3600",
		},
		"urn:dslforum-org:service:WANDSLInterfaceConfig:1#GetInfo": {
			"NewDownstreamCurrRate":    "200000",
			"NewUpstreamCurrRate":      "40000",
			"NewDownstreamMaxRate":     "220000",
			"NewUpstreamMaxRate":       "45000",
			"NewDownstreamNoiseMargin": "120",
			"NewUpstreamNoiseMargin":   "95",
			"NewDownstreamAttenuation": "180",
			"NewUpstreamAttenuation":   "120",
		},
		"urn:dslforum-org:service:WANDSLInterfaceConfig:1#GetStatisticsTotal": {
			"NewFECErrors":           "10",
			"NewATUCFECErrors":       "20",
			"NewCRCErrors":           "30",
			"NewATUCCRCErrors":       "40",
			"NewHECErrors":           "5",
			"NewATUCHECErrors":       "6",
			"NewErroredSecs":         "100",
			"NewSeverelyErroredSecs": "7",
		},
		"urn:dslforum-org:service:WLANConfiguration:1#GetInfo": {
			"NewEnable": "1", "NewStatus": "Up", "NewChannel": "6", "NewSSID": "HomeNet",
		},
		"urn:dslforum-org:service:WLANConfiguration:1#GetTotalAssociations": {"NewTotalAssociations": "12"},
		"urn:dslforum-org:service:WLANConfiguration:1#GetStatistics": {
			"NewTotalPacketsSent": "111", "NewTotalPacketsReceived": "222",
		},
		"urn:dslforum-org:service:WLANConfiguration:2#GetInfo": {
			"NewEnable": "1", "NewStatus": "Up", "NewChannel": "36", "NewSSID": "HomeNet5",
		},
		"urn:dslforum-org:service:WLANConfiguration:2#GetTotalAssociations": {"NewTotalAssociations": "4"},
		"urn:dslforum-org:service:WLANConfiguration:2#GetStatistics": {
			"NewTotalPacketsSent": "333", "NewTotalPacketsReceived": "444",
		},
		"urn:dslforum-org:service:WLANConfiguration:3#GetInfo": {
			"NewEnable": "0", "NewStatus": "Disabled", "NewChannel": "0", "NewSSID": "Guest",
		},
		"urn:dslforum-org:service:WLANConfiguration:3#GetTotalAssociations": {"NewTotalAssociations": "0"},
		"urn:dslforum-org:service:WLANConfiguration:3#GetStatistics": {
			"NewTotalPacketsSent": "0", "NewTotalPacketsReceived": "0",
		},
		"urn:dslforum-org:service:Hosts:1#GetHostNumberOfEntries":   {"NewHostNumberOfEntries": "74"},
		"urn:dslforum-org:service:Hosts:1#X_AVM-DE_GetHostListPath": {"NewX_AVM-DE_HostListPath": "/devicehostlist.lua?sid=fake"},
	}
}

func newTestScraper(t *testing.T, fake *fakeTR064) *fritzboxScraper {
	t.Helper()
	cfg := defaultConfig()
	settings := receivertest.NewNopSettings(metadata.Type)
	s := newScraper(cfg, settings)
	s.client = fake
	return s
}

// metricPoints flattens all datapoints of a metric into (value, attrs) pairs.
type dataPoint struct {
	intVal    int64
	doubleVal float64
	isDouble  bool
	attrs     map[string]string
}

func collectMetrics(t *testing.T, m pmetric.Metrics) map[string][]dataPoint {
	t.Helper()
	out := map[string][]dataPoint{}
	for i := 0; i < m.ResourceMetrics().Len(); i++ {
		rm := m.ResourceMetrics().At(i)
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)
			for k := 0; k < sm.Metrics().Len(); k++ {
				metric := sm.Metrics().At(k)
				dps := extractDataPoints(t, metric)
				out[metric.Name()] = append(out[metric.Name()], dps...)
			}
		}
	}
	return out
}

func extractDataPoints(t *testing.T, metric pmetric.Metric) []dataPoint {
	t.Helper()
	var out []dataPoint
	add := func(dp pmetric.NumberDataPoint) {
		attrs := map[string]string{}
		dp.Attributes().Range(func(k string, v pcommon.Value) bool {
			attrs[k] = v.AsString()
			return true
		})
		switch dp.ValueType() {
		case pmetric.NumberDataPointValueTypeInt:
			out = append(out, dataPoint{intVal: dp.IntValue(), attrs: attrs})
		case pmetric.NumberDataPointValueTypeDouble:
			out = append(out, dataPoint{doubleVal: dp.DoubleValue(), isDouble: true, attrs: attrs})
		}
	}
	switch metric.Type() {
	case pmetric.MetricTypeGauge:
		for i := 0; i < metric.Gauge().DataPoints().Len(); i++ {
			add(metric.Gauge().DataPoints().At(i))
		}
	case pmetric.MetricTypeSum:
		for i := 0; i < metric.Sum().DataPoints().Len(); i++ {
			add(metric.Sum().DataPoints().At(i))
		}
	default:
		t.Fatalf("unexpected metric type %s for %s", metric.Type(), metric.Name())
	}
	return out
}

func findPoint(t *testing.T, points []dataPoint, attrs map[string]string) dataPoint {
	t.Helper()
	for _, p := range points {
		match := true
		for k, v := range attrs {
			if p.attrs[k] != v {
				match = false
				break
			}
		}
		if match {
			return p
		}
	}
	t.Fatalf("no datapoint with attrs %v in %+v", attrs, points)
	return dataPoint{}
}

func TestScrapeFullDSLBox(t *testing.T) {
	fake := &fakeTR064{services: dslBoxServices, responses: dslBoxResponses()}
	s := newTestScraper(t, fake)

	metrics, err := s.scrape(context.Background())
	if err != nil {
		t.Fatalf("scrape: %v", err)
	}
	got := collectMetrics(t, metrics)

	// Device uptime.
	if p := findPoint(t, got["fritzbox.device.uptime"], nil); p.intVal != 86400 {
		t.Errorf("device.uptime = %d, want 86400", p.intVal)
	}
	// WAN connection.
	if p := findPoint(t, got["fritzbox.wan.connection.status"], nil); p.intVal != 1 {
		t.Errorf("wan.connection.status = %d, want 1", p.intVal)
	}
	if p := findPoint(t, got["fritzbox.wan.connection.uptime"], nil); p.intVal != 3600 {
		t.Errorf("wan.connection.uptime = %d, want 3600", p.intVal)
	}
	// hw.network.io.
	if p := findPoint(t, got["hw.network.io"], map[string]string{"network.io.direction": "transmit", "hw.id": "fritzbox.wan"}); p.intVal != 1000000 {
		t.Errorf("hw.network.io transmit = %d, want 1000000", p.intVal)
	}
	if p := findPoint(t, got["hw.network.io"], map[string]string{"network.io.direction": "receive"}); p.intVal != 5000000 {
		t.Errorf("hw.network.io receive = %d, want 5000000", p.intVal)
	}
	// Bandwidth limit: 250000000 bit/s / 8 = 31250000 By/s.
	if p := findPoint(t, got["hw.network.bandwidth.limit"], map[string]string{"hw.id": "fritzbox.wan"}); p.intVal != 31250000 {
		t.Errorf("bandwidth.limit = %d, want 31250000", p.intVal)
	}
	// DSL: noise margin 120 -> 12.0 dB.
	if p := findPoint(t, got["fritzbox.dsl.noise_margin"], map[string]string{"network.io.direction": "receive"}); !p.isDouble || p.doubleVal != 12.0 {
		t.Errorf("dsl.noise_margin receive = %v, want 12.0", p.doubleVal)
	}
	// DSL errors: FEC transmit = ATUCFEC = 20.
	if p := findPoint(t, got["hw.errors"], map[string]string{"error.type": "fec", "network.io.direction": "transmit"}); p.intVal != 20 {
		t.Errorf("hw.errors fec transmit = %d, want 20", p.intVal)
	}
	// WLAN clients per radio.
	if p := findPoint(t, got["fritzbox.wlan.clients"], map[string]string{"hw.id": "fritzbox.wlan1", "ssid": "HomeNet"}); p.intVal != 12 {
		t.Errorf("wlan1 clients = %d, want 12", p.intVal)
	}
	if p := findPoint(t, got["fritzbox.wlan.clients"], map[string]string{"hw.id": "fritzbox.wlan2", "ssid": "HomeNet5"}); p.intVal != 4 {
		t.Errorf("wlan2 clients = %d, want 4", p.intVal)
	}
	// Disabled guest radio still reports up=0.
	if p := findPoint(t, got["hw.network.up"], map[string]string{"hw.id": "fritzbox.wlan3"}); p.intVal != 0 {
		t.Errorf("wlan3 up = %d, want 0", p.intVal)
	}
	// Hosts.
	if p := findPoint(t, got["fritzbox.hosts.total"], nil); p.intVal != 74 {
		t.Errorf("hosts.total = %d, want 74", p.intVal)
	}
	// Optional metrics must be absent by default.
	if _, ok := got["fritzbox.hosts.active"]; ok {
		t.Error("hosts.active must not be emitted when disabled")
	}
	if _, ok := got["fritzbox.wan.external_ip"]; ok {
		t.Error("wan.external_ip must not be emitted when disabled")
	}
	// Resource attributes.
	rm := metrics.ResourceMetrics().At(0)
	attrs := rm.Resource().Attributes()
	if v, _ := attrs.Get("hw.vendor"); v.AsString() != "AVM" {
		t.Errorf("resource hw.vendor = %q, want AVM", v.AsString())
	}
	if v, _ := attrs.Get("hw.model"); v.AsString() != "FRITZ!Box 7590" {
		t.Errorf("resource hw.model = %q", v.AsString())
	}
	if v, _ := attrs.Get("server.address"); v.AsString() != "fritz.box:49000" {
		t.Errorf("resource server.address = %q", v.AsString())
	}
}

func TestScrapeCableBoxSkipsDSL(t *testing.T) {
	// Cable box: no WANDSLInterfaceConfig service.
	services := make([]tr064.Service, 0, len(dslBoxServices))
	for _, svc := range dslBoxServices {
		if strings.Contains(svc.Type, "WANDSLInterfaceConfig") {
			continue
		}
		services = append(services, svc)
	}
	fake := &fakeTR064{services: services, responses: dslBoxResponses()}
	s := newTestScraper(t, fake)

	metrics, err := s.scrape(context.Background())
	if err != nil {
		t.Fatalf("scrape: %v", err)
	}
	got := collectMetrics(t, metrics)
	if _, ok := got["fritzbox.dsl.rate.current"]; ok {
		t.Error("DSL metrics must be absent on cable boxes")
	}
	if _, ok := got["hw.errors"]; ok {
		t.Error("hw.errors must be absent on cable boxes")
	}
	// WAN metrics still present.
	if p := findPoint(t, got["fritzbox.wan.connection.status"], nil); p.intVal != 1 {
		t.Errorf("wan.connection.status = %d, want 1", p.intVal)
	}
}

func TestScrapePPPFallback(t *testing.T) {
	// Replace WANIPConnection with WANPPPConnection.
	services := make([]tr064.Service, 0, len(dslBoxServices))
	for _, svc := range dslBoxServices {
		if strings.Contains(svc.Type, "WANIPConnection") {
			services = append(services, tr064.Service{
				Type: "urn:dslforum-org:service:WANPPPConnection:1", ControlURL: "/upnp/control/wanpppconn1",
			})
			continue
		}
		services = append(services, svc)
	}
	responses := dslBoxResponses()
	responses["urn:dslforum-org:service:WANPPPConnection:1#GetStatusInfo"] = map[string]string{
		"NewConnectionStatus": "Connected",
		"NewUptime":           "99",
	}
	fake := &fakeTR064{services: services, responses: responses}
	s := newTestScraper(t, fake)

	metrics, err := s.scrape(context.Background())
	if err != nil {
		t.Fatalf("scrape: %v", err)
	}
	got := collectMetrics(t, metrics)
	if p := findPoint(t, got["fritzbox.wan.connection.uptime"], nil); p.intVal != 99 {
		t.Errorf("wan.connection.uptime = %d, want 99 (PPP fallback)", p.intVal)
	}
}

func TestScrapeHostsActiveEnabled(t *testing.T) {
	responses := dslBoxResponses()
	responses["urn:dslforum-org:service:Hosts:1#GetHostNumberOfEntries"] = map[string]string{"NewHostNumberOfEntries": "3"}
	responses["urn:dslforum-org:service:Hosts:1#GetGenericHostEntry"] = map[string]string{"NewActive": "1"}
	fake := &fakeTR064{services: dslBoxServices, responses: responses}
	s := newTestScraper(t, fake)
	s.cfg.MetricsBuilderConfig.Metrics.FritzboxHostsActive.Enabled = true
	s.mb = metadata.NewMetricsBuilder(s.cfg.MetricsBuilderConfig, receivertest.NewNopSettings(metadata.Type))

	metrics, err := s.scrape(context.Background())
	if err != nil {
		t.Fatalf("scrape: %v", err)
	}
	got := collectMetrics(t, metrics)
	if p := findPoint(t, got["fritzbox.hosts.active"], nil); p.intVal != 3 {
		t.Errorf("hosts.active = %d, want 3", p.intVal)
	}
}

func TestScrapeDiscoveryFailure(t *testing.T) {
	fake := &fakeTR064{services: nil, responses: nil}
	s := newTestScraper(t, fake)
	s.client = &failingServicesClient{}

	_, err := s.scrape(context.Background())
	if err == nil {
		t.Fatal("expected discovery error")
	}
}

type failingServicesClient struct{ fakeTR064 }

func (f *failingServicesClient) Services(_ context.Context) ([]tr064.Service, error) {
	return nil, errors.New("connection refused")
}

func TestParseInt(t *testing.T) {
	if v, ok := parseInt("42"); !ok || v != 42 {
		t.Errorf("parseInt(42) = %d, %v", v, ok)
	}
	if _, ok := parseInt(""); ok {
		t.Error("parseInt(\"\") must fail")
	}
	if _, ok := parseInt("abc"); ok {
		t.Error("parseInt(abc) must fail")
	}
}

func TestHostOf(t *testing.T) {
	cases := map[string]string{
		"http://fritz.box:49000":      "fritz.box:49000",
		"https://192.168.178.1:49443": "192.168.178.1:49443",
		"http://fritz.box:49000/":     "fritz.box:49000",
	}
	for in, want := range cases {
		if got := hostOf(in); got != want {
			t.Errorf("hostOf(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestConfigValidate(t *testing.T) {
	valid := defaultConfig()
	if err := valid.Validate(); err != nil {
		t.Errorf("default config must be valid: %v", err)
	}

	bad := defaultConfig()
	bad.Endpoint = "not-a-url"
	if err := bad.Validate(); err == nil {
		t.Error("relative endpoint must fail")
	}

	bad = defaultConfig()
	bad.Username = "user"
	if err := bad.Validate(); err == nil {
		t.Error("username without password must fail")
	}

	bad = defaultConfig()
	bad.ScraperControllerSettings.Timeout = 0
	if err := bad.Validate(); err != nil {
		t.Errorf("zero timeout must be allowed: %v", err)
	}

	bad = defaultConfig()
	bad.ScraperControllerSettings.Timeout = bad.ScraperControllerSettings.CollectionInterval + 1
	if err := bad.Validate(); err == nil {
		t.Error("timeout > collection_interval must fail")
	}
}

func TestScrapeHostInfo(t *testing.T) {
	fixture, err := os.ReadFile("testdata/hostlist.xml")
	if err != nil {
		t.Fatal(err)
	}
	responses := dslBoxResponses()
	responses["urn:dslforum-org:service:Hosts:1#X_AVM-DE_GetHostListPath"] = map[string]string{
		"NewX_AVM-DE_HostListPath": "/devicehostlist.lua?sid=fake",
	}
	fake := &fakeTR064{services: dslBoxServices, responses: responses, fetchBody: fixture}
	s := newTestScraper(t, fake)

	metrics, err := s.scrape(context.Background())
	if err != nil {
		t.Fatalf("scrape: %v", err)
	}
	got := collectMetrics(t, metrics)
	points := got["fritzbox.hosts.info"]
	if len(points) != 3 {
		t.Fatalf("expected 3 host info series, got %d", len(points))
	}
	tv := findPoint(t, points, map[string]string{"hostname": "living-room-tv"})
	if tv.attrs["ip"] != "192.168.178.20" || tv.attrs["mac"] != "AA:BB:CC:DD:EE:FF" {
		t.Errorf("tv attrs wrong: %v", tv.attrs)
	}
	if tv.attrs["active"] != "1" || tv.attrs["guest"] != "0" || tv.attrs["friendly_name"] != "Living Room TV" {
		t.Errorf("tv attrs wrong: %v", tv.attrs)
	}
	// Empty hostname/ip must not break parsing.
	empty := findPoint(t, points, map[string]string{"mac": "77:88:99:AA:BB:CC"})
	if empty.attrs["hostname"] != "" || empty.attrs["ip"] != "" {
		t.Errorf("empty fields must be empty strings: %v", empty.attrs)
	}
}
