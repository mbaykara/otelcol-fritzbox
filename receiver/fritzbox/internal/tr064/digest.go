package tr064

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
)

// digestAuthorization builds an RFC 7616 MD5 digest Authorization header
// value from a WWW-Authenticate challenge header. Only the subset of
// parameters sent by Fritz!Box devices is supported (algorithm=MD5,
// qop=auth).
func digestAuthorization(challenge, method, uri, username, password string) (string, error) {
	if !strings.HasPrefix(challenge, "Digest ") {
		return "", fmt.Errorf("unsupported authentication scheme in challenge %q", challenge)
	}
	params := parseDigestParams(strings.TrimPrefix(challenge, "Digest "))

	realm := params["realm"]
	nonce := params["nonce"]
	if realm == "" || nonce == "" {
		return "", fmt.Errorf("digest challenge missing realm or nonce")
	}
	if alg := params["algorithm"]; alg != "" && !strings.EqualFold(alg, "MD5") {
		return "", fmt.Errorf("unsupported digest algorithm %q", alg)
	}

	ha1 := md5Hex(username + ":" + realm + ":" + password)
	ha2 := md5Hex(method + ":" + uri)

	var response, cnonce, nc string
	qop := params["qop"]
	if qop != "" {
		// Fritz!Box offers qop="auth".
		if !strings.Contains(qop, "auth") {
			return "", fmt.Errorf("unsupported digest qop %q", qop)
		}
		c, err := newCnonce()
		if err != nil {
			return "", err
		}
		cnonce = c
		nc = "00000001"
		response = md5Hex(ha1 + ":" + nonce + ":" + nc + ":" + cnonce + ":auth:" + ha2)
	} else {
		response = md5Hex(ha1 + ":" + nonce + ":" + ha2)
	}

	var b strings.Builder
	b.WriteString(`Digest username="`)
	b.WriteString(username)
	b.WriteString(`", realm="`)
	b.WriteString(realm)
	b.WriteString(`", nonce="`)
	b.WriteString(nonce)
	b.WriteString(`", uri="`)
	b.WriteString(uri)
	b.WriteString(`", response="`)
	b.WriteString(response)
	b.WriteString(`"`)
	if qop != "" {
		b.WriteString(`, qop=auth, nc=`)
		b.WriteString(nc)
		b.WriteString(`, cnonce="`)
		b.WriteString(cnonce)
		b.WriteString(`"`)
	}
	return b.String(), nil
}

// parseDigestParams parses the comma-separated key=value list of a digest
// challenge into a map.
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
