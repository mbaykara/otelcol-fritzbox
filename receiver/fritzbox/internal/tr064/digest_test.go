package tr064

import (
	"strings"
	"testing"
)

// TestDigestAuthorizationProgression ensures nc increments per call, which
// the Fritz!Box requires (it enforces nonce-count replay protection).
func TestDigestAuthorizationProgression(t *testing.T) {
	var d digestAuth
	if err := d.updateChallenge(`Digest realm="HTTPS Access", nonce="N1", algorithm=MD5, qop="auth"`); err != nil {
		t.Fatalf("updateChallenge: %v", err)
	}
	first := d.authorizationFor("POST", "/upnp/control/x", "u", "p")
	second := d.authorizationFor("POST", "/upnp/control/x", "u", "p")
	if !strings.Contains(first, "nc=00000001") {
		t.Errorf("first auth: expected nc=00000001, got %q", first)
	}
	if !strings.Contains(second, "nc=00000002") {
		t.Errorf("second auth: expected nc=00000002, got %q", second)
	}
	if first == second {
		t.Error("different nc must produce different responses")
	}
	if strings.Contains(first, "nc=00000001") && strings.Contains(second, "nc=00000002") && d.cnonce == "" {
		t.Error("cnonce must be set for qop=auth")
	}
}

// TestDigestNonceResetOnRenewal resets the counter when the box rotates the
// nonce.
func TestDigestNonceResetOnRenewal(t *testing.T) {
	var d digestAuth
	if err := d.updateChallenge(`Digest realm="r", nonce="N1", qop="auth"`); err != nil {
		t.Fatal(err)
	}
	d.authorizationFor("POST", "/x", "u", "p")
	if err := d.updateChallenge(`Digest realm="r", nonce="N2", qop="auth"`); err != nil {
		t.Fatal(err)
	}
	auth := d.authorizationFor("POST", "/x", "u", "p")
	if !strings.Contains(auth, "nc=00000001") {
		t.Errorf("nonce renewal must reset nc, got %q", auth)
	}
}

func TestDigestWithoutQOP(t *testing.T) {
	var d digestAuth
	if err := d.updateChallenge(`Digest realm="r", nonce="N1"`); err != nil {
		t.Fatal(err)
	}
	auth := d.authorizationFor("GET", "/x", "u", "p")
	if strings.Contains(auth, "qop=") {
		t.Errorf("challenge without qop must omit qop in response: %q", auth)
	}
}

func TestDigestChallengeValidation(t *testing.T) {
	var d digestAuth
	if err := d.updateChallenge("Basic abc"); err == nil {
		t.Error("Basic must be rejected")
	}
	if err := d.updateChallenge(`Digest realm="r"`); err == nil {
		t.Error("missing nonce must error")
	}
	if err := d.updateChallenge(`Digest nonce="n"`); err == nil {
		t.Error("missing realm must error")
	}
	if err := d.updateChallenge(`Digest realm="r", nonce="n", algorithm=SHA-256`); err == nil {
		t.Error("SHA-256 must be rejected")
	}
	if err := d.updateChallenge(`Digest realm="r", nonce="n", qop="auth-int"`); err == nil {
		t.Error("qop without auth must be rejected")
	}
}

func TestParseDigestParams(t *testing.T) {
	params := parseDigestParams(`realm="HTTPS Access",nonce="ABC123",algorithm=MD5,qop="auth"`)
	want := map[string]string{
		"realm":     "HTTPS Access",
		"nonce":     "ABC123",
		"algorithm": "MD5",
		"qop":       "auth",
	}
	for k, v := range want {
		if params[k] != v {
			t.Errorf("param %s: want %q, got %q", k, v, params[k])
		}
	}
}

func TestStripEndpoint(t *testing.T) {
	if got := stripEndpoint("http://fritz.box:49000/upnp/control/wancommonifconfig1"); got != "/upnp/control/wancommonifconfig1" {
		t.Errorf("got %q", got)
	}
	if got := stripEndpoint("http://fritz.box:49000"); got != "/" {
		t.Errorf("root path: got %q", got)
	}
}
