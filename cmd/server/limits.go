package main

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"time"
)

// Public-exposure limits (docs/design/online-spike.md §7).
const (
	maxConns      = 2 * maxRooms
	maxConnsPerIP = 4
	// idleTimeout closes a peer that sent nothing (no INPUT, no PING): a hidden
	// tab stops its game loop, so this is also the AFK kick.
	idleTimeout = 20 * time.Second
	// rivalTimeout closes a host still alone in its room.
	rivalTimeout = 5 * time.Minute
	// msgWindow/maxMsgsPerWindow: INPUT is sent at <= 120 Hz plus 1 PING/s;
	// twice that leaves room for jitter bursts.
	msgWindow        = time.Second
	maxMsgsPerWindow = 2 * (120 + 1)
	closeIdle        = 4003
	closeRate        = 4004
	closeNoRival     = 4005
)

// connLimiter caps total connections and connections per client IP.
type connLimiter struct {
	mu    sync.Mutex
	total int
	perIP map[string]int
	max   int
	maxIP int
}

func newConnLimiter(max, maxIP int) *connLimiter {
	return &connLimiter{perIP: map[string]int{}, max: max, maxIP: maxIP}
}

// acquire reserves a slot for ip; release must be called iff it returns true.
func (l *connLimiter) acquire(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.total >= l.max || l.perIP[ip] >= l.maxIP {
		return false
	}
	l.total++
	l.perIP[ip]++
	return true
}

func (l *connLimiter) release(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.total--
	if l.perIP[ip]--; l.perIP[ip] <= 0 {
		delete(l.perIP, ip)
	}
}

// clientIP is the TCP peer address, or behind a trusted proxy (Fly.io) the
// Fly-Client-IP header, else the last X-Forwarded-For entry: both are set by
// the proxy, which clients cannot spoof. Only trust them when the server is
// unreachable except through that proxy.
func clientIP(req *http.Request, trustProxy bool) string {
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

// rateWindow counts messages in fixed windows (one per connection, read loop only).
type rateWindow struct {
	start time.Time
	n     int
}

// allow records one message at now and reports whether the window still has room.
func (w *rateWindow) allow(now time.Time) bool { return w.allowN(now, maxMsgsPerWindow) }

// allowN is allow with a caller-chosen cap per window (the lobby needs far fewer).
func (w *rateWindow) allowN(now time.Time, max int) bool {
	if now.Sub(w.start) >= msgWindow {
		w.start = now
		w.n = 0
	}
	w.n++
	return w.n <= max
}

// ipv6Prefix is the block one subscriber usually controls: rotating addresses
// inside it must not multiply the per-IP caps.
const ipv6Prefix = 64

// limitKey is the per-client bucket for ip: IPv4 as is, IPv6 by its /64.
func limitKey(ip string) string {
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
