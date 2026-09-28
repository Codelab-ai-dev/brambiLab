// Package contact receives contact-form messages, stores them with a durable send job and sends
// them to the owner through Resend (web-v1.md §13, §13.1, WEB-007). It is disabled unless fully
// configured: without configuration nothing is accepted and nothing is sent.
package contact

import (
	"fmt"
	"net/mail"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	defaultAPIURL    = "https://api.resend.com"
	TemplateVersion  = 1
	defaultRetention = 30 * 24 * time.Hour
)

// Config is read once at startup from the environment (Go service only).
type Config struct {
	Enabled bool
	// Problems explains why contact is disabled (for logs; never exposed publicly).
	Problems []string

	APIKey string
	From   string // "Name <addr@verified.domain>" or a plain address
	To     string // fixed recipient
	APIURL string // tests only

	PerClient      int           // requests per client per ClientWindow
	ClientWindow   time.Duration //
	Global         int           // accepted messages per GlobalWindow
	GlobalWindow   time.Duration //
	Retention      time.Duration
	TrustedProxies []netip.Prefix
}

// privateRanges are trusted as proxy hops by default: the Docker network of Coolify → Caddy → Go.
var privateRanges = []string{"127.0.0.0/8", "::1/128", "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "fc00::/7"}

func LoadConfig(getenv func(string) string) Config {
	c := Config{
		APIKey: strings.TrimSpace(getenv("RESEND_API_KEY")), From: strings.TrimSpace(getenv("CONTACT_FROM")), To: strings.TrimSpace(getenv("CONTACT_TO")),
		APIURL:    strings.TrimRight(strings.TrimSpace(getenv("RESEND_API_URL")), "/"),
		PerClient: 5, ClientWindow: 15 * time.Minute, Global: 100, GlobalWindow: time.Hour, Retention: defaultRetention,
	}
	if c.APIURL == "" {
		c.APIURL = defaultAPIURL
	}
	problem := func(format string, args ...any) { c.Problems = append(c.Problems, fmt.Sprintf(format, args...)) }

	for env, dst := range map[string]*int{"CONTACT_RATE_PER_CLIENT": &c.PerClient, "CONTACT_RATE_GLOBAL": &c.Global} {
		if v := getenv(env); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil || n < 1 || n > 100000 {
				problem("%s must be an integer between 1 and 100000", env)
				continue
			}
			*dst = n
		}
	}
	if v := getenv("CONTACT_RETENTION_DAYS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 365 {
			problem("CONTACT_RETENTION_DAYS must be between 1 and 365")
		} else {
			c.Retention = time.Duration(n) * 24 * time.Hour
		}
	}
	proxies := getenv("TRUSTED_PROXIES")
	if proxies == "" {
		proxies = strings.Join(privateRanges, ",")
	}
	for _, p := range strings.Split(proxies, ",") {
		pfx, err := netip.ParsePrefix(strings.TrimSpace(p))
		if err != nil {
			problem("TRUSTED_PROXIES has an invalid CIDR %q", p)
			continue
		}
		c.TrustedProxies = append(c.TrustedProxies, pfx.Masked())
	}

	switch strings.ToLower(strings.TrimSpace(getenv("CONTACT_ENABLED"))) {
	case "true", "1", "yes":
	default:
		problem("CONTACT_ENABLED is not true")
	}
	if c.APIKey == "" {
		problem("RESEND_API_KEY is empty")
	} else if strings.ContainsAny(c.APIKey, " \r\n\t") {
		problem("RESEND_API_KEY contains whitespace")
	}
	if a, err := mail.ParseAddress(c.From); err != nil || strings.ContainsAny(c.From, "\r\n") {
		problem("CONTACT_FROM must be an address such as \"BrambiLab <contacto@your-verified-domain>\"")
	} else {
		c.From = a.String()
	}
	if a, err := mail.ParseAddress(c.To); err != nil || a.Name != "" || a.Address != c.To {
		problem("CONTACT_TO must be a single plain address")
	}
	if u, err := url.Parse(c.APIURL); err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || u.User != nil {
		problem("RESEND_API_URL must be an http(s) base URL")
	}
	c.Enabled = len(c.Problems) == 0
	return c
}
