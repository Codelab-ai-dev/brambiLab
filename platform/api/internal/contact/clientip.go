package contact

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// ClientIP returns the effective client address. Only hops inside trusted may add to
// X-Forwarded-For: walking the chain from the right (the closest hop first), the first address
// that is not a trusted proxy is the client. Anything to its left was written by the client and
// is ignored, so a visitor cannot choose its own identity. If every hop is trusted (local
// development), the leftmost address is used.
func ClientIP(r *http.Request, trusted []netip.Prefix) netip.Addr {
	remote := parseAddr(r.RemoteAddr)
	if !remote.IsValid() || !isTrusted(remote, trusted) {
		return remote
	}
	var chain []netip.Addr
	for _, h := range r.Header.Values("X-Forwarded-For") {
		for _, part := range strings.Split(h, ",") {
			a := parseAddr(strings.TrimSpace(part))
			if !a.IsValid() {
				// A malformed entry breaks the chain: trust nothing further left.
				chain = nil
				continue
			}
			chain = append(chain, a)
		}
	}
	for i := len(chain) - 1; i >= 0; i-- {
		if !isTrusted(chain[i], trusted) {
			return chain[i]
		}
	}
	if len(chain) > 0 {
		return chain[0]
	}
	return remote
}

func parseAddr(s string) netip.Addr {
	if host, _, err := net.SplitHostPort(s); err == nil {
		s = host
	}
	s = strings.Trim(s, "[]")
	a, err := netip.ParseAddr(s)
	if err != nil {
		return netip.Addr{}
	}
	return a.Unmap()
}

func isTrusted(a netip.Addr, trusted []netip.Prefix) bool {
	for _, p := range trusted {
		if p.Contains(a) {
			return true
		}
	}
	return false
}
