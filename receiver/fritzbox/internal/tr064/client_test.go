package tr064

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const testDeviceDescription = `<?xml version="1.0"?>
<root xmlns="urn:dslforum-org:device-1-0">
<device>
<deviceType>urn:dslforum-org:device:InternetGatewayDevice:1</deviceType>
<serviceList>
<service>
<serviceType>urn:dslforum-org:service:DeviceInfo:1</serviceType>
<controlURL>/upnp/control/deviceinfo</controlURL>
</service>
<service>
<serviceType>urn:dslforum-org:service:Hosts:1</serviceType>
<controlURL>/upnp/control/hosts</controlURL>
</service>
</serviceList>
<deviceList>
<device>
<deviceType>urn:dslforum-org:device:WANDevice:1</deviceType>
<serviceList>
<service>
<serviceType>urn:dslforum-org:service:WANCommonInterfaceConfig:1</serviceType>
<controlURL>/upnp/control/wancommonifconfig1</controlURL>
</service>
</serviceList>
</device>
</deviceList>
</device>
</root>`

// newTestClient returns a client pointed at the test server.
func newTestClient(t *testing.T, handler http.Handler) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c, err := NewClient(srv.URL, "", "", srv.Client())
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return c
}

func TestServicesRecursesIntoSubDevices(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tr64desc.xml" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		fmt.Fprint(w, testDeviceDescription)
	}))

	services, err := c.Services(context.Background())
	if err != nil {
		t.Fatalf("Services: %v", err)
	}
	if len(services) != 3 {
		t.Fatalf("expected 3 services, got %d", len(services))
	}
	expected := map[string]string{
		"urn:dslforum-org:service:DeviceInfo:1":               "/upnp/control/deviceinfo",
		"urn:dslforum-org:service:Hosts:1":                    "/upnp/control/hosts",
		"urn:dslforum-org:service:WANCommonInterfaceConfig:1": "/upnp/control/wancommonifconfig1",
	}
	for _, svc := range services {
		if expected[svc.Type] != svc.ControlURL {
			t.Errorf("service %s: unexpected control URL %q", svc.Type, svc.ControlURL)
		}
	}
}

func TestCallParsesOutArguments(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("SOAPAction"); got != "urn:dslforum-org:service:DeviceInfo:1#GetInfo" {
			t.Errorf("unexpected SOAPAction %q", got)
		}
		fmt.Fprint(w, `<?xml version="1.0"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
<s:Body>
<u:GetInfoResponse xmlns:u="urn:dslforum-org:service:DeviceInfo:1">
<NewModelName>FRITZ!Box 7590</NewModelName>
<NewUpTime>12345</NewUpTime>
</u:GetInfoResponse>
</s:Body>
</s:Envelope>`)
	}))

	out, err := c.Call(context.Background(), "urn:dslforum-org:service:DeviceInfo:1", "/upnp/control/deviceinfo", "GetInfo")
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if out["NewModelName"] != "FRITZ!Box 7590" || out["NewUpTime"] != "12345" {
		t.Errorf("unexpected out args: %v", out)
	}
}

func TestCallSOAPFault(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<?xml version="1.0"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
<s:Body>
<s:Fault>
<faultcode>s:Client</faultcode>
<faultstring>UPnPError</faultstring>
<detail>
<UPnPError xmlns="urn:schemas-upnp-org:control-1-0">
<errorCode>401</errorCode>
<errorDescription>Invalid Action</errorDescription>
</UPnPError>
</detail>
</s:Fault>
</s:Body>
</s:Envelope>`)
	}))

	_, err := c.Call(context.Background(), "urn:dslforum-org:service:DeviceInfo:1", "/upnp/control/deviceinfo", "NoSuchAction")
	if err == nil {
		t.Fatal("expected error")
	}
	trErr, ok := err.(*Error)
	if !ok {
		t.Fatalf("expected *Error, got %T: %v", err, err)
	}
	if trErr.Code != 401 || trErr.Description != "Invalid Action" {
		t.Errorf("unexpected fault: %+v", trErr)
	}
}

func TestCallWithArgs(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := readBody(t, r)
		if !strings.Contains(body, "<NewIndex>3</NewIndex>") {
			t.Errorf("expected NewIndex arg in envelope, got: %s", body)
		}
		fmt.Fprint(w, `<?xml version="1.0"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
<s:Body>
<u:GetGenericHostEntryResponse xmlns:u="urn:dslforum-org:service:Hosts:1">
<NewActive>1</NewActive>
<NewHostName>laptop</NewHostName>
</u:GetGenericHostEntryResponse>
</s:Body>
</s:Envelope>`)
	}))

	out, err := c.CallWithArgs(context.Background(), "urn:dslforum-org:service:Hosts:1", "/upnp/control/hosts",
		"GetGenericHostEntry", map[string]string{"NewIndex": "3"})
	if err != nil {
		t.Fatalf("CallWithArgs: %v", err)
	}
	if out["NewActive"] != "1" || out["NewHostName"] != "laptop" {
		t.Errorf("unexpected out args: %v", out)
	}
}

func TestCallDigestRetry(t *testing.T) {
	var firstCall = true
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if firstCall {
			firstCall = false
			w.Header().Set("WWW-Authenticate", `Digest realm="HTTPS Access",nonce="ABC123",algorithm=MD5,qop="auth"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Digest ") || !strings.Contains(auth, `nonce="ABC123"`) {
			t.Errorf("missing/invalid Authorization header: %q", auth)
		}
		fmt.Fprint(w, `<?xml version="1.0"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
<s:Body>
<u:GetInfoResponse xmlns:u="urn:dslforum-org:service:DeviceInfo:1">
<NewUpTime>7</NewUpTime>
</u:GetInfoResponse>
</s:Body>
</s:Envelope>`)
	}))
	defer srv.Close()

	c, err := NewClient(srv.URL, "user", "pass", srv.Client())
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	out, err := c.Call(context.Background(), "urn:dslforum-org:service:DeviceInfo:1", "/upnp/control/deviceinfo", "GetInfo")
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if out["NewUpTime"] != "7" {
		t.Errorf("unexpected out args: %v", out)
	}
}

func TestCallUnauthorizedWithoutCredentials(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))

	_, err := c.Call(context.Background(), "urn:dslforum-org:service:DeviceInfo:1", "/upnp/control/deviceinfo", "GetInfo")
	if err == nil {
		t.Fatal("expected error")
	}
	trErr, ok := err.(*Error)
	if !ok || trErr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 *Error, got %T: %v", err, err)
	}
}

func TestNewClientRejectsBadEndpoint(t *testing.T) {
	for _, endpoint := range []string{"", "fritz.box", "http://"} {
		if _, err := NewClient(endpoint, "", "", nil); err == nil {
			t.Errorf("endpoint %q: expected error", endpoint)
		}
	}
}

func readBody(t *testing.T, r *http.Request) string {
	t.Helper()
	buf, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("reading request body: %v", err)
	}
	return string(buf)
}
