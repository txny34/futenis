// Package netlimit holds the server's public-exposure guards: connection
// caps per client, fixed-window message rates and the client identity used
// for both (docs/design/online-spike.md §7).
package netlimit

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"time"
)

// Window is the length of one RateWindow.
const Window = time.Second

// ConnLimiter caps total connections and connections per client IP.
type ConnLimiter struct {
	mu    sync.Mutex
	total int
	perIP map[string]int
	max   int
	maxIP int
}

func NewConnLimiter(max, maxIP int) *ConnLimiter {
	return &ConnLimiter{perIP: map[string]int{}, max: max, maxIP: maxIP}
}

// Acquire reserves a slot for ip; release must be called iff it returns true.
func (l *ConnLimiter) Acquire(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.total >= l.max || l.perIP[ip] >= l.maxIP {
		return false
	}
	l.total++
	l.perIP[ip]++
	return true
}

func (l *ConnLimiter) Release(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.total--
	if l.perIP[ip]--; l.perIP[ip] <= 0 {
		delete(l.perIP, ip)
	}
}

// ClientIP is the TCP peer address, or behind a trusted proxy (Fly.io) the
// Fly-Client-IP header, else the last X-Forwarded-For entry: both are set by
// the proxy, which clients cannot spoof. Only trust them when the server is
// unreachable except through that proxy.
func ClientIP(req *http.Request, trustProxy bool) string {
	if trustProxy {
		if ip := strings.TrimSpace(req.Header.Get("Fly-Client-IP")); ip != "" {
			return ip
		}
		xff := req.Header.Get("X-Forwarded-For")
		if ip := strings.TrimSpace(xff[strings.LastIndexByte(xff, ',')+1:]); ip != "" {
			return ip
		}
	}
	host, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		return req.RemoteAddr
	}
	return host
}

// RateWindow counts messages in fixed windows (one per connection, read loop only).
type RateWindow struct {
	start time.Time
	n     int
}

// AllowN records one message at now and reports whether the window still has
// room for max messages.
func (w *RateWindow) AllowN(now time.Time, max int) bool {
	if now.Sub(w.start) >= Window {
		w.start = now
		w.n = 0
	}
	w.n++
	return w.n <= max
}

// ipv6Prefix is the block one subscriber usually controls: rotating addresses
// inside it must not multiply the per-IP caps.
const ipv6Prefix = 64

// LimitKey is the per-client bucket for ip: IPv4 as is, IPv6 by its /64.
func LimitKey(ip string) string {
	a, err := netip.ParseAddr(ip)
	if err != nil {
		return ip
	}
	a = a.Unmap()
	if a.Is4() {
		return a.String()
	}
	return netip.PrefixFrom(a, ipv6Prefix).Masked().String()
}
