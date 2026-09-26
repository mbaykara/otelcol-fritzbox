// Package fakebox is an in-memory TR-064 device for tests. It serves the
// device description document, SOAP actions behind HTTP digest
// authentication, and the AVM host list, so the receiver and the collector
// distribution can be exercised end to end without a real Fritz!Box.
//
// Configure a Box before serving it; the exported fields must not be
// mutated while requests are in flight. All data is synthetic.
package fakebox

import (
	"bytes"
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

// Realm is the digest realm announced by the fake device.
const Realm = "F!Box SOAP-Auth"

// hostListSID is the session id embedded in the host list path.
const hostListSID = "0123456789abcdef"

// Service is a TR-064 service offered by the fake device.
type Service struct {
	Type       string
	ControlURL string
}

// Stats counts requests handled by the fake device.
type Stats struct {
	// SOAPCalls is the number of SOAP requests received, including
	// unauthenticated attempts.
	SOAPCalls int
	// Challenges is the number of 401 digest challenges sent.
	Challenges int
	// Authorized is the number of SOAP requests with valid digest credentials.
	Authorized int
	// Rejected is the number of SOAP requests with invalid digest credentials.
	Rejected int
	// HostListFetches is the number of successful host list downloads.
	HostListFetches int
	// DescriptionFetches is the number of tr64desc.xml requests, including
	// those answered while the device was down.
	DescriptionFetches int
}

// Box is a fake Fritz!Box. It implements http.Handler.
type Box struct {
	// Username and Password are the digest credentials. When Username is
	// empty, every action is served without authentication.
	Username string
	Password string
	// Services is the service list served in tr64desc.xml.
	Services []Service
	// Responses maps "serviceType#action" to the action's out arguments.
	// Actions without a response answer with UPnP fault 401 (Invalid Action).
	Responses map[string]map[string]string
	// Faults maps "serviceType#action" to a UPnP error code to answer with.
	Faults map[string]int
	// NoAuth lists "serviceType#action" keys served without authentication.
	NoAuth map[string]bool
	// HostList is the XML document served at the host list path.
	HostList []byte

	mu     sync.Mutex
	down   bool
	nonce  string
	lastNC uint64
	stats  Stats
}

// NewDSL returns a fake DSL Fritz!Box 7590 with three WLAN radios, WAN over
// WANIPConnection, and two known hosts. The credentials are
// "tester"/"secret".
func NewDSL() *Box {
	return &Box{
		Username: "tester",
		Password: "secret",
		Services: []Service{
			{Type: "urn:dslforum-org:service:DeviceInfo:1", ControlURL: "/upnp/control/deviceinfo"},
			{Type: "urn:dslforum-org:service:WANCommonInterfaceConfig:1", ControlURL: "/upnp/control/wancommonifconfig1"},
			{Type: "urn:dslforum-org:service:WANDSLInterfaceConfig:1", ControlURL: "/upnp/control/wandslifconfig1"},
			{Type: "urn:dslforum-org:service:WANIPConnection:1", ControlURL: "/upnp/control/wanipconnection1"},
			{Type: "urn:dslforum-org:service:WLANConfiguration:1", ControlURL: "/upnp/control/wlanconfig1"},
			{Type: "urn:dslforum-org:service:WLANConfiguration:2", ControlURL: "/upnp/control/wlanconfig2"},
			{Type: "urn:dslforum-org:service:WLANConfiguration:3", ControlURL: "/upnp/control/wlanconfig3"},
			{Type: "urn:dslforum-org:service:Hosts:1", ControlURL: "/upnp/control/hosts"},
		},
		Responses: map[string]map[string]string{
			"urn:dslforum-org:service:DeviceInfo:1#GetInfo": {
				"NewModelName":       "FRITZ!Box 7590",
				"NewSerialNumber":    "X000000000000",
				"NewSoftwareVersion": "154.08.25",
				"NewUpTime":          "86400",
			},
			"urn:dslforum-org:service:WANCommonInterfaceConfig:1#GetCommonLinkProperties": {
				"NewWANAccessType":              "DSL",
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
			"urn:dslforum-org:service:WANIPConnection:1#GetExternalIPAddress": {"NewExternalIPAddress": "203.0.113.10"},
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
				"NewEnable": "1", "NewStatus": "Up", "NewChannel": "6", "NewSSID": "ExampleNet",
			},
			"urn:dslforum-org:service:WLANConfiguration:1#GetTotalAssociations": {"NewTotalAssociations": "12"},
			"urn:dslforum-org:service:WLANConfiguration:1#GetStatistics": {
				"NewTotalPacketsSent": "111", "NewTotalPacketsReceived": "222",
			},
			"urn:dslforum-org:service:WLANConfiguration:2#GetInfo": {
				"NewEnable": "1", "NewStatus": "Up", "NewChannel": "36", "NewSSID": "ExampleNet5",
			},
			"urn:dslforum-org:service:WLANConfiguration:2#GetTotalAssociations": {"NewTotalAssociations": "4"},
			"urn:dslforum-org:service:WLANConfiguration:2#GetStatistics": {
				"NewTotalPacketsSent": "333", "NewTotalPacketsReceived": "444",
			},
			"urn:dslforum-org:service:WLANConfiguration:3#GetInfo": {
				"NewEnable": "0", "NewStatus": "Disabled", "NewChannel": "0", "NewSSID": "ExampleGuest",
			},
			"urn:dslforum-org:service:WLANConfiguration:3#GetTotalAssociations": {"NewTotalAssociations": "0"},
			"urn:dslforum-org:service:WLANConfiguration:3#GetStatistics": {
				"NewTotalPacketsSent": "0", "NewTotalPacketsReceived": "0",
			},
			"urn:dslforum-org:service:Hosts:1#GetHostNumberOfEntries":   {"NewHostNumberOfEntries": "2"},
			"urn:dslforum-org:service:Hosts:1#X_AVM-DE_GetHostListPath": {"NewX_AVM-DE_HostListPath": "/devicehostlist.lua?sid=" + hostListSID},
		},
		NoAuth: map[string]bool{
			"urn:dslforum-org:service:Hosts:1#GetHostNumberOfEntries": true,
		},
		HostList: []byte(`<?xml version="1.0" encoding="UTF-8"?>
<List>
<Item><Index>1</Index><IPAddress>192.0.2.20</IPAddress><MACAddress>02:00:00:00:00:01</MACAddress><Active>1</Active><HostName>example-laptop</HostName><InterfaceType>802.11</InterfaceType><X_AVM-DE_Guest>0</X_AVM-DE_Guest><X_AVM-DE_FriendlyName>Example Laptop</X_AVM-DE_FriendlyName></Item>
<Item><Index>2</Index><IPAddress>192.0.2.21</IPAddress><MACAddress>02:00:00:00:00:02</MACAddress><Active>0</Active><HostName>example-printer</HostName><InterfaceType>Ethernet</InterfaceType><X_AVM-DE_Guest>0</X_AVM-DE_Guest><X_AVM-DE_FriendlyName>Example Printer</X_AVM-DE_FriendlyName></Item>
</List>
`),
	}
}

// Stats returns a snapshot of the request counters.
func (b *Box) Stats() Stats {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.stats
}

// SetDown makes the device answer every request with 503 Service
// Unavailable while down is true, simulating a rebooting router.
func (b *Box) SetDown(down bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.down = down
}

// RotateNonce invalidates the current digest nonce, forcing clients to
// answer a fresh challenge on their next request.
func (b *Box) RotateNonce() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.nonce = ""
	b.lastNC = 0
}

// ServeHTTP implements http.Handler.
func (b *Box) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	b.mu.Lock()
	down := b.down
	if r.URL.Path == "/tr64desc.xml" {
		b.stats.DescriptionFetches++
	}
	b.mu.Unlock()
	if down {
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
		return
	}
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/tr64desc.xml":
		b.serveDescription(w)
	case r.Method == http.MethodGet && r.URL.Path == "/devicehostlist.lua":
		b.serveHostList(w, r)
	case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/upnp/control/"):
		b.serveSOAP(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (b *Box) serveDescription(w http.ResponseWriter) {
	// The first service is attached to the root device, the rest to a
	// nested sub-device, mirroring the layout of real devices.
	var buf bytes.Buffer
	buf.WriteString(`<?xml version="1.0"?>` + "\n")
	buf.WriteString(`<root xmlns="urn:dslforum-org:device-1-0"><device><deviceType>urn:dslforum-org:device:InternetGatewayDevice:1</deviceType><serviceList>`)
	for i, svc := range b.Services {
		if i == 1 {
			buf.WriteString(`</serviceList><deviceList><device><deviceType>urn:dslforum-org:device:LANDevice:1</deviceType><serviceList>`)
		}
		buf.WriteString(`<service><serviceType>`)
		xmlEscape(&buf, svc.Type)
		buf.WriteString(`</serviceType><controlURL>`)
		xmlEscape(&buf, svc.ControlURL)
		buf.WriteString(`</controlURL></service>`)
	}
	if len(b.Services) > 1 {
		buf.WriteString(`</serviceList></device></deviceList></device></root>`)
	} else {
		buf.WriteString(`</serviceList></device></root>`)
	}
	w.Header().Set("Content-Type", "text/xml")
	_, _ = w.Write(buf.Bytes())
}

func (b *Box) serveHostList(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("sid") != hostListSID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	b.mu.Lock()
	b.stats.HostListFetches++
	b.mu.Unlock()
	w.Header().Set("Content-Type", "text/xml")
	_, _ = w.Write(b.HostList)
}

func (b *Box) serveSOAP(w http.ResponseWriter, r *http.Request) {
	b.mu.Lock()
	b.stats.SOAPCalls++
	b.mu.Unlock()

	serviceType, action, ok := strings.Cut(r.Header.Get("SOAPAction"), "#")
	if !ok {
		http.Error(w, "missing SOAPAction header", http.StatusBadRequest)
		return
	}
	svc, found := b.service(serviceType)
	if !found || svc.ControlURL != r.URL.Path {
		http.NotFound(w, r)
		return
	}
	if _, err := io.Copy(io.Discard, io.LimitReader(r.Body, 1<<20)); err != nil {
		http.Error(w, "reading body", http.StatusBadRequest)
		return
	}

	key := serviceType + "#" + action
	if b.Username != "" && !b.NoAuth[key] && !b.authorize(w, r) {
		return
	}

	if code, faulted := b.Faults[key]; faulted {
		writeFault(w, code, "Action Failed")
		return
	}
	out, found := b.Responses[key]
	if !found {
		writeFault(w, 401, "Invalid Action")
		return
	}
	writeResponse(w, serviceType, action, out)
}

func (b *Box) service(serviceType string) (Service, bool) {
	for _, svc := range b.Services {
		if svc.Type == serviceType {
			return svc, true
		}
	}
	return Service{}, false
}

// authorize validates the digest Authorization header. It enforces a
// strictly increasing nonce count, as real devices do. On failure it writes
// a 401 challenge and returns false.
func (b *Box) authorize(w http.ResponseWriter, r *http.Request) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.nonce == "" {
		b.nonce = newNonce()
		b.lastNC = 0
	}
	header := r.Header.Get("Authorization")
	if header == "" {
		b.challenge(w)
		return false
	}
	params, ok := parseDigest(header)
	if !ok || params["nonce"] != b.nonce || params["realm"] != Realm || params["uri"] != r.URL.RequestURI() {
		// A stale nonce is not a credential failure: re-challenge.
		b.challenge(w)
		return false
	}
	nc, err := strconv.ParseUint(params["nc"], 16, 64)
	if err != nil || nc <= b.lastNC || params["qop"] != "auth" {
		b.challenge(w)
		return false
	}
	ha1 := md5Hex(b.Username + ":" + Realm + ":" + b.Password)
	ha2 := md5Hex(r.Method + ":" + params["uri"])
	want := md5Hex(ha1 + ":" + b.nonce + ":" + params["nc"] + ":" + params["cnonce"] + ":auth:" + ha2)
	if params["username"] != b.Username || params["response"] != want {
		b.stats.Rejected++
		b.challenge(w)
		return false
	}
	b.lastNC = nc
	b.stats.Authorized++
	return true
}

// challenge writes a 401 digest challenge. b.mu must be held.
func (b *Box) challenge(w http.ResponseWriter) {
	b.stats.Challenges++
	w.Header().Set("WWW-Authenticate",
		fmt.Sprintf(`Digest realm="%s", nonce="%s", algorithm=MD5, qop="auth"`, Realm, b.nonce))
	w.WriteHeader(http.StatusUnauthorized)
}

func writeResponse(w http.ResponseWriter, serviceType, action string, out map[string]string) {
	var buf bytes.Buffer
	buf.WriteString(`<?xml version="1.0"?>` + "\n")
	buf.WriteString(`<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/"><s:Body>`)
	fmt.Fprintf(&buf, `<u:%sResponse xmlns:u="`, action)
	xmlEscape(&buf, serviceType)
	buf.WriteString(`">`)
	for name, value := range out {
		fmt.Fprintf(&buf, "<%s>", name)
		xmlEscape(&buf, value)
		fmt.Fprintf(&buf, "</%s>", name)
	}
	fmt.Fprintf(&buf, `</u:%sResponse></s:Body></s:Envelope>`, action)
	w.Header().Set("Content-Type", `text/xml; charset="utf-8"`)
	_, _ = w.Write(buf.Bytes())
}

func writeFault(w http.ResponseWriter, code int, description string) {
	var buf bytes.Buffer
	buf.WriteString(`<?xml version="1.0"?>` + "\n")
	buf.WriteString(`<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/"><s:Body><s:Fault><faultcode>s:Client</faultcode><faultstring>UPnPError</faultstring><detail><UPnPError xmlns="urn:dslforum-org:control-1-0">`)
	fmt.Fprintf(&buf, "<errorCode>%d</errorCode><errorDescription>", code)
	xmlEscape(&buf, description)
	buf.WriteString(`</errorDescription></UPnPError></detail></s:Fault></s:Body></s:Envelope>`)
	w.Header().Set("Content-Type", `text/xml; charset="utf-8"`)
	w.WriteHeader(http.StatusInternalServerError)
	_, _ = w.Write(buf.Bytes())
}

func parseDigest(header string) (map[string]string, bool) {
	rest, ok := strings.CutPrefix(header, "Digest ")
	if !ok {
		return nil, false
	}
	params := map[string]string{}
	for part := range strings.SplitSeq(rest, ",") {
		k, v, found := strings.Cut(strings.TrimSpace(part), "=")
		if !found {
			continue
		}
		params[strings.ToLower(k)] = strings.Trim(v, `"`)
	}
	return params, true
}

func xmlEscape(buf *bytes.Buffer, s string) {
	_ = xml.EscapeText(buf, []byte(s))
}

func md5Hex(s string) string {
	sum := md5.Sum([]byte(s)) //nolint:gosec // MD5 is mandated by TR-064 digest authentication.
	return hex.EncodeToString(sum[:])
}

func newNonce() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return strings.ToUpper(hex.EncodeToString(b))
}
