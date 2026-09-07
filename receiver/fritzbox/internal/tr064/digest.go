package tr064

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
)

// digestAuth holds cached digest challenge parameters and the nonce count.
// The Fritz!Box enforces nonce-count replay protection: every request must
// increment nc for the cached nonce, otherwise the box rejects it (usually
// with a spurious 500 "XML error" soap fault). So the challenge is cached
// per client and nc is bumped on each authorization header generated.
type digestAuth struct {
	mu       sync.Mutex
	realm    string
	nonce    string
	qopAuth  bool
	cnonce   string
	nonceCnt int
}

// authorizationFor builds the Authorization header value for method+uri.
// The returned bool reports whether digest parameters are available.
func (d *digestAuth) authorizationFor(method, uri, username, password string) string {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.realm == "" || d.nonce == "" {
		return ""
	}
	d.nonceCnt++
	nc := fmt.Sprintf("%08x", d.nonceCnt)

	ha1 := md5Hex(username + ":" + d.realm + ":" + password)
	ha2 := md5Hex(method + ":" + uri)
	var response string
	if d.qopAuth {
		response = md5Hex(ha1 + ":" + d.nonce + ":" + nc + ":" + d.cnonce + ":auth:" + ha2)
	} else {
		response = md5Hex(ha1 + ":" + d.nonce + ":" + ha2)
	}

	var b strings.Builder
	b.WriteString(`Digest username="`)
	b.WriteString(username)
	b.WriteString(`", realm="`)
	b.WriteString(d.realm)
	b.WriteString(`", nonce="`)
	b.WriteString(d.nonce)
	b.WriteString(`", uri="`)
	b.WriteString(uri)
	b.WriteString(`", response="`)
	b.WriteString(response)
	b.WriteString(`"`)
	if d.qopAuth {
		b.WriteString(`, qop=auth, nc=`)
		b.WriteString(nc)
		b.WriteString(`, cnonce="`)
		b.WriteString(d.cnonce)
		b.WriteString(`"`)
	}
	return b.String()
}

// updateChallenge parses a WWW-Authenticate digest challenge into d. It
// resets the nonce count on new nonces. qop="auth" (optionally listed among
// others) is required when qop is present; MD5 only.
func (d *digestAuth) updateChallenge(challenge string) error {
	if !strings.HasPrefix(challenge, "Digest ") {
		return fmt.Errorf("unsupported authentication scheme in challenge %q", challenge)
	}
	params := parseDigestParams(strings.TrimPrefix(challenge, "Digest "))

	realm := params["realm"]
	nonce := params["nonce"]
	if realm == "" || nonce == "" {
		return fmt.Errorf("digest challenge missing realm or nonce")
	}
	if alg := params["algorithm"]; alg != "" && !strings.EqualFold(alg, "MD5") {
		return fmt.Errorf("unsupported digest algorithm %q", alg)
	}
	qopAuth := false
	if qop := params["qop"]; qop != "" {
		for _, opt := range strings.Split(qop, ",") {
			if strings.TrimSpace(opt) == "auth" {
				qopAuth = true
				break
			}
		}
		if !qopAuth {
			return fmt.Errorf("unsupported digest qop %q", qop)
		}
	}

	c, err := newCnonce()
	if err != nil {
		return err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.nonce != nonce {
		d.nonceCnt = 0
	}
	d.realm = realm
	d.nonce = nonce
	d.qopAuth = qopAuth
	d.cnonce = c
	return nil
}

// parseDigestParams parses the comma-separated key=value list of a digest
// challenge into a map. Multiple qop tokens are joined by the caller.
func parseDigestParams(s string) map[string]string {
	params := map[string]string{}
	for _, part := range strings.Split(s, ",") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			continue
		}
		params[strings.ToLower(kv[0])] = strings.Trim(kv[1], `"`)
	}
	return params
}

func md5Hex(s string) string {
	sum := md5.Sum([]byte(s)) //nolint:gosec // MD5 required by RFC 7616 and Fritz!Box.
	return hex.EncodeToString(sum[:])
}

func newCnonce() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating cnonce: %w", err)
	}
	return hex.EncodeToString(b), nil
}
