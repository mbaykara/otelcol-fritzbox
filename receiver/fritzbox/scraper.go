package fritzbox

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/mbaykara/fritzotel-receiver/receiver/fritzbox/internal/metadata"
	"github.com/mbaykara/fritzotel-receiver/receiver/fritzbox/internal/tr064"
)

// TR-064 service URN prefixes, without the version suffix.
const (
	serviceDeviceInfo            = "urn:dslforum-org:service:DeviceInfo"
	serviceWANCommonInterfaceCfg = "urn:dslforum-org:service:WANCommonInterfaceConfig"
	serviceWANDSLInterfaceCfg    = "urn:dslforum-org:service:WANDSLInterfaceConfig"
	serviceWANIPConnection       = "urn:dslforum-org:service:WANIPConnection"
	serviceWANPPPConnection      = "urn:dslforum-org:service:WANPPPConnection"
	serviceWLANConfiguration     = "urn:dslforum-org:service:WLANConfiguration"
	serviceHosts                 = "urn:dslforum-org:service:Hosts"
)

// Synthetic hardware IDs required by the hw.network.* semantic conventions.
// The Fritz!Box exposes no native hardware identifiers.
const (
	hwIDWAN = "fritzbox.wan"
)

// fritzboxScraper implements the metrics collection against one Fritz!Box device.
type fritzboxScraper struct {
	cfg      *Config
	settings receiver.Settings
	logger   *zap.Logger
	mb       *metadata.MetricsBuilder

	client tr064Client

	mu                 sync.Mutex
	services           []tr064.Service
	warned             map[string]struct{}
	resourceAttrsCache pcommon.Resource
}

func newScraper(cfg *Config, settings receiver.Settings) *fritzboxScraper {
	return &fritzboxScraper{
		cfg:      cfg,
		settings: settings,
		logger:   settings.Logger,
		mb:       metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, settings),
		warned:   map[string]struct{}{},
	}
}

// start initializes the TR-064 client and discovers device services.
func (s *fritzboxScraper) start(ctx context.Context, _ component.Host) error {
	if s.client == nil {
		client, err := newTR064Client(s.cfg)
		if err != nil {
			return err
		}
		s.client = client
	}
	// Discovery failure must not abort startup: the device may be temporarily
	// unreachable. The first scrape retries discovery.
	if err := s.discover(ctx); err != nil {
		s.logger.Warn("fritzbox: initial service discovery failed, will retry on first scrape", zap.Error(err))
	}
	return nil
}

func (s *fritzboxScraper) shutdown(_ context.Context) error {
	if closer, ok := s.client.(interface{ CloseIdleConnections() }); ok {
		closer.CloseIdleConnections()
	}
	return nil
}

// discover fetches the service list once and caches it.
func (s *fritzboxScraper) discover(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.services != nil {
		return nil
	}
	services, err := s.client.Services(ctx)
	if err != nil {
		return err
	}
	s.services = services
	s.logger.Info("fritzbox: discovered TR-064 services", zap.Int("count", len(services)))
	return nil
}

// findService returns the control URL for the first service whose type has
// the given prefix (ignoring version suffix), or false if the device does
// not offer it.
func (s *fritzboxScraper) findService(prefix string) (tr064.Service, bool) {
	for _, svc := range s.services {
		if len(svc.Type) > len(prefix) && svc.Type[:len(prefix)] == prefix {
			return svc, true
		}
	}
	return tr064.Service{}, false
}

// findServices returns all services whose type has the given prefix,
// ordered as discovered (WLANConfiguration instances come in radio order).
func (s *fritzboxScraper) findServices(prefix string) []tr064.Service {
	var out []tr064.Service
	for _, svc := range s.services {
		if len(svc.Type) > len(prefix) && svc.Type[:len(prefix)] == prefix {
			out = append(out, svc)
		}
	}
	return out
}

// scrape collects all metric groups and emits them.
func (s *fritzboxScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	if err := s.discover(ctx); err != nil {
		return pmetric.NewMetrics(), fmt.Errorf("fritzbox: service discovery: %w", err)
	}

	now := pcommon.NewTimestampFromTime(time.Now())
	var errs []error

	s.scrapeDeviceInfo(ctx, now, &errs)
	s.scrapeWAN(ctx, now, &errs)
	s.scrapeDSL(ctx, now, &errs)
	s.scrapeWLAN(ctx, now, &errs)
	s.scrapeHosts(ctx, now, &errs)

	resourceOpts := s.resourceOptions(ctx)
	return s.mb.Emit(resourceOpts...), errors.Join(errs...)
}

// resourceOptions fetches static device information once and caches it for
// the resource attributes.
func (s *fritzboxScraper) resourceOptions(ctx context.Context) []metadata.ResourceMetricsOption {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.resourceAttrsCache != (pcommon.Resource{}) {
		return []metadata.ResourceMetricsOption{metadata.WithResource(s.resourceAttrsCache)}
	}
	info, err := s.callGroup(ctx, "device-info", serviceDeviceInfo, "GetInfo")
	if err != nil {
		return nil
	}
	s.resourceAttrsCache = buildResource(info, s.cfg.Endpoint)
	return []metadata.ResourceMetricsOption{metadata.WithResource(s.resourceAttrsCache)}
}

// buildResource assembles the resource attributes from DeviceInfo.GetInfo
// and the configured endpoint.
func buildResource(info map[string]string, endpoint string) pcommon.Resource {
	res := pcommon.NewResource()
	attrs := res.Attributes()
	attrs.PutStr("hw.vendor", "AVM")
	if v := info["NewModelName"]; v != "" {
		attrs.PutStr("hw.model", v)
	}
	if v := info["NewSerialNumber"]; v != "" {
		attrs.PutStr("hw.serial_number", v)
	}
	if v := info["NewSoftwareVersion"]; v != "" {
		attrs.PutStr("fritzbox.device.software_version", v)
	}
	if host := hostOf(endpoint); host != "" {
		attrs.PutStr("server.address", host)
	}
	return res
}

func (s *fritzboxScraper) scrapeDeviceInfo(ctx context.Context, now pcommon.Timestamp, errs *[]error) {
	info, err := s.callGroup(ctx, "device", serviceDeviceInfo, "GetInfo")
	if err != nil {
		*errs = append(*errs, err)
		return
	}
	if v, ok := parseInt(info["NewUpTime"]); ok {
		s.mb.RecordFritzboxDeviceUptimeDataPoint(now, v)
	}
}

func (s *fritzboxScraper) scrapeWAN(ctx context.Context, now pcommon.Timestamp, errs *[]error) {
	// Link properties and traffic counters.
	if props, err := s.callGroup(ctx, "wan-link", serviceWANCommonInterfaceCfg, "GetCommonLinkProperties"); err != nil {
		*errs = append(*errs, err)
	} else {
		// Layer1 rates are bit/s; hw.network.bandwidth.limit is By/s.
		if down, ok := parseInt(props["NewLayer1DownstreamMaxBitRate"]); ok {
			s.mb.RecordHwNetworkBandwidthLimitDataPoint(now, down/8, hwIDWAN)
		}
		linkUp := int64(0)
		if props["NewPhysicalLinkStatus"] == "Up" {
			linkUp = 1
		}
		s.mb.RecordHwNetworkUpDataPoint(now, linkUp, hwIDWAN, "wan")
	}

	if bytesSent, bytesRecv, err := s.callWANCounterPair(ctx, "GetTotalBytesSent", "GetTotalBytesReceived"); err != nil {
		*errs = append(*errs, err)
	} else {
		s.mb.RecordHwNetworkIoDataPoint(now, bytesSent, hwIDWAN, "wan", metadata.AttributeNetworkIoDirectionTransmit)
		s.mb.RecordHwNetworkIoDataPoint(now, bytesRecv, hwIDWAN, "wan", metadata.AttributeNetworkIoDirectionReceive)
	}

	if sent, recv, err := s.callWANCounterPair(ctx, "GetTotalPacketsSent", "GetTotalPacketsReceived"); err != nil {
		*errs = append(*errs, err)
	} else {
		s.mb.RecordHwNetworkPacketsDataPoint(now, sent, hwIDWAN, "wan", metadata.AttributeNetworkIoDirectionTransmit)
		s.mb.RecordHwNetworkPacketsDataPoint(now, recv, hwIDWAN, "wan", metadata.AttributeNetworkIoDirectionReceive)
	}

	// Bandwidth utilization is an AVM extension, reported in bps as strings.
	if util, err := s.callGroup(ctx, "wan-utilization", serviceWANCommonInterfaceCfg, "X_AVM-DE_GetCommonLinkProperties"); err == nil {
		s.recordUtilization(now, util)
	}
	// Connection status/uptime come from the active WAN connection service.
	s.scrapeWANConnection(ctx, now, errs)
}

// recordUtilization converts the AVM utilization values to fractions.
func (s *fritzboxScraper) recordUtilization(now pcommon.Timestamp, util map[string]string) {
	down, downOK := parseInt(util["NewX_AVM-DE_DownstreamCurrentUtilization"])
	up, upOK := parseInt(util["NewX_AVM-DE_UpstreamCurrentUtilization"])
	if !downOK && !upOK {
		return
	}
	props, err := s.callGroupCtx(context.Background(), serviceWANCommonInterfaceCfg, "GetCommonLinkProperties")
	if err != nil {
		return
	}
	maxDown, dOK := parseInt(props["NewLayer1DownstreamMaxBitRate"])
	maxUp, uOK := parseInt(props["NewLayer1UpstreamMaxBitRate"])
	if downOK && dOK && maxDown > 0 {
		s.mb.RecordHwNetworkBandwidthUtilizationDataPoint(now, float64(down)/float64(maxDown), hwIDWAN, metadata.AttributeNetworkIoDirectionReceive)
	}
	if upOK && uOK && maxUp > 0 {
		s.mb.RecordHwNetworkBandwidthUtilizationDataPoint(now, float64(up)/float64(maxUp), hwIDWAN, metadata.AttributeNetworkIoDirectionTransmit)
	}
}

// callWANCounterPair calls two counter actions on WANCommonInterfaceConfig
// and returns their values.
func (s *fritzboxScraper) callWANCounterPair(ctx context.Context, sentAction, recvAction string) (int64, int64, error) {
	sentResp, err := s.callGroup(ctx, "wan-traffic", serviceWANCommonInterfaceCfg, sentAction)
	if err != nil {
		return 0, 0, err
	}
	recvResp, err := s.callGroup(ctx, "wan-traffic", serviceWANCommonInterfaceCfg, recvAction)
	if err != nil {
		return 0, 0, err
	}
	sent, ok1 := parseInt(firstValue(sentResp))
	recv, ok2 := parseInt(firstValue(recvResp))
	if !ok1 || !ok2 {
		return 0, 0, errors.New("fritzbox: unparsable WAN counter values")
	}
	return sent, recv, nil
}

// scrapeWANConnection handles WANIPConnection with fallback to WANPPPConnection.
func (s *fritzboxScraper) scrapeWANConnection(ctx context.Context, now pcommon.Timestamp, errs *[]error) {
	svc, ok := s.findService(serviceWANIPConnection)
	if !ok {
		svc, ok = s.findService(serviceWANPPPConnection)
	}
	if !ok {
		s.warnOnce("wan-connection", errNoWANConnectionService.Error())
		return
	}

	status, err := s.call(ctx, svc, "GetStatusInfo")
	if err != nil {
		// Faulting advertised action or missing service: group disabled,
		// not a scrape failure. All call paths here are best-effort.
		var tr064Err *tr064.Error
		if errors.As(err, &tr064Err) {
			s.warnOnce("wan-connection-fault", fmt.Sprintf("GetStatusInfo on %s faults (UPnP %d), WAN connection metrics disabled", svc.Type, tr064Err.Code))
			return
		}
		*errs = append(*errs, fmt.Errorf("wan connection status: %w", err))
		return
	}
	connected := int64(0)
	if status["NewConnectionStatus"] == "Connected" {
		connected = 1
	}
	s.mb.RecordFritzboxWanConnectionStatusDataPoint(now, connected)
	if v, ok := parseInt(status["NewUptime"]); ok {
		s.mb.RecordFritzboxWanConnectionUptimeDataPoint(now, v)
	}

	if s.cfg.MetricsBuilderConfig.Metrics.FritzboxWanExternalIP.Enabled {
		if ext, err := s.call(ctx, svc, "GetExternalIPAddress"); err == nil {
			if ip := ext["NewExternalIPAddress"]; ip != "" {
				s.mb.RecordFritzboxWanExternalIPDataPoint(now, 1, ip)
			}
		}
	}
}

func (s *fritzboxScraper) scrapeDSL(ctx context.Context, now pcommon.Timestamp, errs *[]error) {
	info, err := s.callGroup(ctx, "dsl", serviceWANDSLInterfaceCfg, "GetInfo")
	if err != nil {
		if !isServiceMissing(err) {
			*errs = append(*errs, err)
		}
		return
	}
	// TR-064 reports DSL rates in kbit/s; the metrics use bit/s (UCUM).
	if v, ok := parseInt(info["NewDownstreamCurrRate"]); ok {
		s.mb.RecordFritzboxDslRateCurrentDataPoint(now, v*1000, metadata.AttributeNetworkIoDirectionReceive)
	}
	if v, ok := parseInt(info["NewUpstreamCurrRate"]); ok {
		s.mb.RecordFritzboxDslRateCurrentDataPoint(now, v*1000, metadata.AttributeNetworkIoDirectionTransmit)
	}
	if v, ok := parseInt(info["NewDownstreamMaxRate"]); ok {
		s.mb.RecordFritzboxDslRateMaxDataPoint(now, v*1000, metadata.AttributeNetworkIoDirectionReceive)
	}
	if v, ok := parseInt(info["NewUpstreamMaxRate"]); ok {
		s.mb.RecordFritzboxDslRateMaxDataPoint(now, v*1000, metadata.AttributeNetworkIoDirectionTransmit)
	}
	// Noise margin and attenuation are reported in 0.1 dB units.
	if v, ok := parseInt(info["NewDownstreamNoiseMargin"]); ok {
		s.mb.RecordFritzboxDslNoiseMarginDataPoint(now, float64(v)/10, metadata.AttributeNetworkIoDirectionReceive)
	}
	if v, ok := parseInt(info["NewUpstreamNoiseMargin"]); ok {
		s.mb.RecordFritzboxDslNoiseMarginDataPoint(now, float64(v)/10, metadata.AttributeNetworkIoDirectionTransmit)
	}
	if v, ok := parseInt(info["NewDownstreamAttenuation"]); ok {
		s.mb.RecordFritzboxDslAttenuationDataPoint(now, float64(v)/10, metadata.AttributeNetworkIoDirectionReceive)
	}
	if v, ok := parseInt(info["NewUpstreamAttenuation"]); ok {
		s.mb.RecordFritzboxDslAttenuationDataPoint(now, float64(v)/10, metadata.AttributeNetworkIoDirectionTransmit)
	}

	stats, err := s.callGroup(ctx, "dsl", serviceWANDSLInterfaceCfg, "GetStatisticsTotal")
	if err != nil {
		*errs = append(*errs, err)
		return
	}
	// DSL error counters.
	errorCounters := []struct {
		key       string
		errorType metadata.AttributeErrorType
		direction metadata.AttributeNetworkIoDirection
	}{
		{"NewFECErrors", metadata.AttributeErrorTypeFec, metadata.AttributeNetworkIoDirectionReceive},
		{"NewATUCFECErrors", metadata.AttributeErrorTypeFec, metadata.AttributeNetworkIoDirectionTransmit},
		{"NewCRCErrors", metadata.AttributeErrorTypeCrc, metadata.AttributeNetworkIoDirectionReceive},
		{"NewATUCCRCErrors", metadata.AttributeErrorTypeCrc, metadata.AttributeNetworkIoDirectionTransmit},
		{"NewHECErrors", metadata.AttributeErrorTypeHec, metadata.AttributeNetworkIoDirectionReceive},
		{"NewATUCHECErrors", metadata.AttributeErrorTypeHec, metadata.AttributeNetworkIoDirectionTransmit},
	}
	for _, ec := range errorCounters {
		if v, ok := parseInt(stats[ec.key]); ok {
			s.mb.RecordHwErrorsDataPoint(now, v, hwIDWAN, ec.errorType, ec.direction)
		}
	}
	if v, ok := parseInt(stats["NewErroredSecs"]); ok {
		s.mb.RecordFritzboxDslErrorSecondsDataPoint(now, v, metadata.AttributeSeverityErrored)
	}
	if v, ok := parseInt(stats["NewSeverelyErroredSecs"]); ok {
		s.mb.RecordFritzboxDslErrorSecondsDataPoint(now, v, metadata.AttributeSeveritySeverelyErrored)
	}
}

func (s *fritzboxScraper) scrapeWLAN(ctx context.Context, now pcommon.Timestamp, errs *[]error) {
	radios := s.findServices(serviceWLANConfiguration)
	for i, radio := range radios {
		hwID := fmt.Sprintf("fritzbox.wlan%d", i+1)
		info, err := s.call(ctx, radio, "GetInfo")
		if err != nil {
			s.warnOnce("wlan-"+hwID, fmt.Sprintf("WLAN radio %s unavailable: %v", hwID, err))
			continue
		}
		ssid := info["NewSSID"]

		enabled := int64(0)
		if info["NewEnable"] == "1" && info["NewStatus"] == "Up" {
			enabled = 1
		}
		s.mb.RecordHwNetworkUpDataPoint(now, enabled, hwID, ssid)

		if v, ok := parseInt(info["NewChannel"]); ok {
			s.mb.RecordFritzboxWlanChannelDataPoint(now, v, hwID, ssid)
		}

		assoc, err := s.call(ctx, radio, "GetTotalAssociations")
		if err != nil {
			s.warnOnce("wlan-assoc-"+hwID, fmt.Sprintf("WLAN associations %s: %v", hwID, err))
			continue
		}
		if v, ok := parseInt(assoc["NewTotalAssociations"]); ok {
			s.mb.RecordFritzboxWlanClientsDataPoint(now, v, hwID, ssid)
		}

		stats, err := s.call(ctx, radio, "GetStatistics")
		if err != nil {
			continue
		}
		if v, ok := parseInt(stats["NewTotalPacketsSent"]); ok {
			s.mb.RecordHwNetworkPacketsDataPoint(now, v, hwID, ssid, metadata.AttributeNetworkIoDirectionTransmit)
		}
		if v, ok := parseInt(stats["NewTotalPacketsReceived"]); ok {
			s.mb.RecordHwNetworkPacketsDataPoint(now, v, hwID, ssid, metadata.AttributeNetworkIoDirectionReceive)
		}
	}
}

func (s *fritzboxScraper) scrapeHosts(ctx context.Context, now pcommon.Timestamp, errs *[]error) {
	resp, err := s.callGroup(ctx, "hosts", serviceHosts, "GetHostNumberOfEntries")
	if err != nil {
		*errs = append(*errs, err)
		return
	}
	total, ok := parseInt(resp["NewHostNumberOfEntries"])
	if !ok {
		return
	}
	s.mb.RecordFritzboxHostsTotalDataPoint(now, total)

	if !s.cfg.MetricsBuilderConfig.Metrics.FritzboxHostsActive.Enabled {
		return
	}
	svc, ok := s.findService(serviceHosts)
	if !ok {
		return
	}
	var active int64
	for i := int64(0); i < total; i++ {
		entry, err := s.callWithArgs(ctx, svc, "GetGenericHostEntry", map[string]string{"NewIndex": strconv.FormatInt(i, 10)})
		if err != nil {
			s.warnOnce("hosts-entry", fmt.Sprintf("host entry %d failed: %v", i, err))
			continue
		}
		if entry["NewActive"] == "1" {
			active++
		}
	}
	s.mb.RecordFritzboxHostsActiveDataPoint(now, active)
}

// callGroup resolves a service by prefix and calls an action on it. Missing
// services and actions the device does not actually implement (it may still
// advertise them in SCPD, then answer with SOAP faults) produce a warn-once
// log and a typed error so the group is skipped without failing the scrape.
func (s *fritzboxScraper) callGroup(ctx context.Context, group, servicePrefix, action string) (map[string]string, error) {
	svc, ok := s.findService(servicePrefix)
	if !ok {
		s.warnOnce(group, fmt.Sprintf("service %s not offered by device, metric group %q disabled", servicePrefix, group))
		return nil, errServiceMissing{servicePrefix}
	}
	resp, err := s.call(ctx, svc, action)
	if err != nil {
		var tr064Err *tr064.Error
		// 401: authentication required for this action - skip group.
		// 5xx/SOAP fault on an advertised action: the box doesn't actually
		// implement it (observed for WANIPConnection on this box) - skip group.
		if errors.As(err, &tr064Err) && tr064Err.Code == http.StatusUnauthorized {
			s.warnOnce(group+"-auth", fmt.Sprintf("action %s on %s requires authentication, metric group %q skipped", action, servicePrefix, group))
			return nil, err
		}
		if errors.As(err, &tr064Err) && tr064Err.Code >= 500 {
			s.warnOnce(group+"-fault", fmt.Sprintf("action %s on %s answers with SOAP fault %d, metric group %q disabled", action, servicePrefix, tr064Err.Code, group))
			return nil, errServiceUnavailable{servicePrefix, action}
		}
		return nil, fmt.Errorf("fritzbox: %s.%s: %w", servicePrefix, action, err)
	}
	return resp, nil
}

// callGroupCtx is callGroup without warn bookkeeping, for internal reuse.
func (s *fritzboxScraper) callGroupCtx(ctx context.Context, servicePrefix, action string) (map[string]string, error) {
	svc, ok := s.findService(servicePrefix)
	if !ok {
		return nil, errServiceMissing{servicePrefix}
	}
	return s.call(ctx, svc, action)
}

// call invokes an action on a resolved service.
func (s *fritzboxScraper) call(ctx context.Context, svc tr064.Service, action string) (map[string]string, error) {
	return s.client.Call(ctx, svc.Type, svc.ControlURL, action)
}

// callWithArgs invokes an action with input arguments.
func (s *fritzboxScraper) callWithArgs(ctx context.Context, svc tr064.Service, action string, args map[string]string) (map[string]string, error) {
	return s.client.CallWithArgs(ctx, svc.Type, svc.ControlURL, action, args)
}

// warnOnce logs a warning once per key for the lifetime of the scraper.
func (s *fritzboxScraper) warnOnce(key, msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, seen := s.warned[key]; seen {
		return
	}
	s.warned[key] = struct{}{}
	s.logger.Warn("fritzbox: "+msg, zap.String("group", key))
}

// errServiceMissing marks a service that the device does not offer.
type errServiceMissing struct{ service string }

func (e errServiceMissing) Error() string { return "fritzbox: service not offered: " + e.service }

// errServiceUnavailable marks an advertised service whose actions fault.
type errServiceUnavailable struct{ service, action string }

func (e errServiceUnavailable) Error() string {
	return "fritzbox: service action unavailable: " + e.service + "." + e.action
}

func isServiceMissing(err error) bool {
	var e errServiceMissing
	return errors.As(err, &e)
}

// parseInt parses a TR-064 numeric argument. TR-064 sends all values as
// strings; empty or invalid values yield ok=false.
func parseInt(s string) (int64, bool) {
	if s == "" {
		return 0, false
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

// firstValue returns the first value of a single-argument SOAP response.
func firstValue(m map[string]string) string {
	for _, v := range m {
		return v
	}
	return ""
}

// hostOf extracts the host part of a URL string, without failing hard.
func hostOf(endpoint string) string {
	// Avoid importing net/url for this cosmetic attribute: strip scheme and
	// path manually, tolerate malformed input.
	rest := endpoint
	for _, prefix := range []string{"http://", "https://"} {
		if len(rest) > len(prefix) && rest[:len(prefix)] == prefix {
			rest = rest[len(prefix):]
			break
		}
	}
	for i, c := range rest {
		if c == '/' {
			return rest[:i]
		}
	}
	return rest
}
