// Package security holds validators used anywhere graphify touches outside
// input: URLs for ingest, file paths for graph reads, labels destined for
// JSON/HTML rendering.
package security

import (
	"errors"
	"fmt"
	"html"
	"net"
	"net/url"
	"path/filepath"
	"strings"
	"unicode"
)

const MaxLabelLen = 256

// SanitizeLabel strips control characters, caps length, and HTML-escapes.
func SanitizeLabel(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r == '\n' || r == '\t' || r == '\r' {
			b.WriteByte(' ')
			continue
		}
		if unicode.IsControl(r) {
			continue
		}
		b.WriteRune(r)
	}
	out := strings.TrimSpace(b.String())
	if len(out) > MaxLabelLen {
		out = out[:MaxLabelLen]
	}
	return html.EscapeString(out)
}

// ValidateURL requires http/https and blocks private / metadata IPs.
func ValidateURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("parse url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("unsupported scheme: %s", u.Scheme)
	}
	host := u.Hostname()
	if host == "" {
		return nil, errors.New("url has no host")
	}
	// Resolve host; block private, loopback, multicast, link-local.
	if ip := net.ParseIP(host); ip != nil {
		if err := guardIP(ip); err != nil {
			return nil, err
		}
	} else {
		addrs, err := net.LookupIP(host)
		if err == nil {
			for _, ip := range addrs {
				if err := guardIP(ip); err != nil {
					return nil, err
				}
			}
		}
	}
	// Block common cloud metadata hostnames.
	if host == "metadata.google.internal" ||
		strings.HasSuffix(host, ".internal") {
		return nil, fmt.Errorf("metadata host blocked: %s", host)
	}
	return u, nil
}

func guardIP(ip net.IP) error {
	switch {
	case ip.IsLoopback(), ip.IsPrivate(), ip.IsLinkLocalUnicast(),
		ip.IsLinkLocalMulticast(), ip.IsMulticast(), ip.IsUnspecified():
		return fmt.Errorf("address blocked: %s", ip)
	}
	// AWS/GCP/Azure metadata endpoints.
	if ip.String() == "169.254.169.254" {
		return errors.New("cloud metadata endpoint blocked")
	}
	return nil
}

// ValidateGraphPath ensures p resolves inside root (typically "graphify-out").
func ValidateGraphPath(p, root string) (string, error) {
	absP, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	absR, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(absP, absR+string(filepath.Separator)) && absP != absR {
		return "", fmt.Errorf("%s escapes %s", absP, absR)
	}
	return absP, nil
}
