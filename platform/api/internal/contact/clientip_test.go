package contact

import (
	"net/http"
	"net/netip"
	"testing"
)

func TestClientIPIgnoresSpoofedHops(t *testing.T) {
	trusted := LoadConfig(func(string) string { return "" }).TrustedProxies
	cases := []struct {
		name, remote string
		xff          []string
		want         string
	}{
		{"direct public client, header ignored", "203.0.113.9:5000", []string{"198.51.100.1"}, "203.0.113.9"},
		{"through Traefik and Caddy", "172.18.0.5:40000", []string{"203.0.113.7, 172.18.0.2"}, "203.0.113.7"},
		{"spoofed left part ignored", "172.18.0.5:40000", []string{"1.2.3.4, 203.0.113.7, 172.18.0.2"}, "203.0.113.7"},
		{"multiple headers", "172.18.0.5:40000", []string{"1.2.3.4", "203.0.113.7", "172.18.0.2"}, "203.0.113.7"},
		{"garbage breaks the chain", "172.18.0.5:40000", []string{"1.2.3.4, bogus, 172.18.0.2"}, "172.18.0.2"},
		{"all private (local)", "172.18.0.5:40000", []string{"172.18.0.1"}, "172.18.0.1"},
		{"no header", "172.18.0.5:40000", nil, "172.18.0.5"},
		{"ipv6 client", "[::1]:1", []string{"2001:db8::1"}, "2001:db8::1"},
		{"mapped ipv4", "[::ffff:172.18.0.5]:1", []string{"203.0.113.7"}, "203.0.113.7"},
	}
	for _, c := range cases {
		r := &http.Request{RemoteAddr: c.remote, Header: http.Header{}}
		for _, h := range c.xff {
			r.Header.Add("X-Forwarded-For", h)
		}
		if got := ClientIP(r, trusted); got != netip.MustParseAddr(c.want) {
			t.Errorf("%s: %v, want %s", c.name, got, c.want)
		}
	}
}

func TestConfigDisabledUnlessComplete(t *testing.T) {
	env := map[string]string{"CONTACT_ENABLED": "true", "RESEND_API_KEY": "re_test", "CONTACT_FROM": "BrambiLab <contacto@example.com>", "CONTACT_TO": "owner@example.com"}
	get := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }
	if c := LoadConfig(get(env)); !c.Enabled || c.APIURL != "https://api.resend.com" {
		t.Fatalf("complete config: %+v", c.Problems)
	}
	for key, bad := range map[string]string{
		"CONTACT_ENABLED": "", "RESEND_API_KEY": "", "CONTACT_FROM": "not an address", "CONTACT_TO": "Owner <o@example.com>",
		"TRUSTED_PROXIES": "nope", "CONTACT_RATE_GLOBAL": "0", "RESEND_API_URL": "ftp://x",
	} {
		m := map[string]string{}
		for k, v := range env {
			m[k] = v
		}
		m[key] = bad
		if c := LoadConfig(get(m)); c.Enabled || len(c.Problems) == 0 {
			t.Errorf("%s=%q should disable contact", key, bad)
		}
	}
	if c := LoadConfig(get(map[string]string{})); c.Enabled {
		t.Fatal("empty config enabled")
	}
}
