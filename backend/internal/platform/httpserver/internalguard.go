package httpserver

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// InternalOnly hides paths under prefix from anyone outside the allowed networks with a plain 404, so the internal
// surface is invisible rather than merely locked. The peer is the TCP peer: put this behind a proxy only if that
// proxy is itself inside the allowed range and enforces the same rule.
func InternalOnly(prefix string, cidrs []string) Middleware {
	var nets []netip.Prefix
	for _, c := range cidrs {
		if p, err := netip.ParsePrefix(strings.TrimSpace(c)); err == nil {
			nets = append(nets, p)
		}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, prefix) && !peerAllowed(r.RemoteAddr, nets) {
				http.NotFound(w, r)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func peerAllowed(remote string, nets []netip.Prefix) bool {
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		host = remote
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return false
	}
	addr = addr.Unmap()
	for _, n := range nets {
		if n.Contains(addr) {
			return true
		}
	}
	return false
}

// ParseCIDRs splits a comma-separated list, dropping blanks.
func ParseCIDRs(raw string) []string { return ParseOrigins(raw) }
