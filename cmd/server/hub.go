package main

import (
	"context"
	"log"
	"net/http"
	"regexp"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
)

const (
	writeTimeout = 100 * time.Millisecond
	readLimit    = 1024
	closeFull    = 4001
	closeBusy    = 4002
	maxRooms     = 256
)

var roomCode = regexp.MustCompile(`^[A-Z0-9]{3,12}$`)

// hub maps room codes to rooms.
type hub struct {
	mu       sync.Mutex
	rooms    map[string]*room
	origins  []string
	proxied  bool                              // trust X-Forwarded-For (behind the hosting proxy)
	ticker   func() (<-chan time.Time, func()) // injectable for tests
	logger   *log.Logger
	maxRooms int
	conns    *connLimiter
	idle     time.Duration // injectable for tests
	wait     time.Duration

	queueSeq  int    // matchmaking room counter (queue.go)
	queueCode string // room waiting for a second queued player, "" if none
}

func newHub(origins []string, logger *log.Logger) *hub {
	return &hub{rooms: map[string]*room{}, origins: origins, logger: logger, ticker: realTicker, maxRooms: maxRooms,
		conns: newConnLimiter(maxConns, maxConnsPerIP), idle: idleTimeout, wait: rivalTimeout}
}

func realTicker() (<-chan time.Time, func()) {
	t := time.NewTicker(frameInterval)
	return t.C, t.Stop
}

// roomFor returns the open room for code, creating it if missing or closed.
// It returns nil when a new room is needed but the hub is at capacity.
func (h *hub) roomFor(code string) *room {
	h.mu.Lock()
	defer h.mu.Unlock()
	r := h.rooms[code]
	if r == nil || r.isClosed() {
		if r == nil && len(h.rooms) >= h.maxRooms {
			return nil
		}
		r = newRoom()
		h.rooms[code] = r
	}
	return r
}

func (h *hub) forget(code string, r *room) {
	h.mu.Lock()
	if h.rooms[code] == r {
		delete(h.rooms, code)
	}
	h.mu.Unlock()
}

// wsPeer adapts a websocket.Conn to peer.
type wsPeer struct{ c *websocket.Conn }

func (p wsPeer) send(b []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
	defer cancel()
	return p.c.Write(ctx, websocket.MessageBinary, b)
}

func (p wsPeer) close(code int, reason string) { _ = p.c.Close(websocket.StatusCode(code), reason) }

func (h *hub) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	q := req.URL.Query()
	code, queued := q.Get("room"), q.Has("queue")
	if queued && q.Get("queue") != queueMode {
		http.Error(w, "bad queue", http.StatusBadRequest)
		return
	}
	if !queued && !roomCode.MatchString(code) {
		http.Error(w, "bad room code", http.StatusBadRequest)
		return
	}
	ip := clientIP(req, h.proxied)
	if !h.conns.acquire(ip) {
		http.Error(w, "too many connections", http.StatusTooManyRequests)
		return
	}
	defer h.conns.release(ip)
	c, err := websocket.Accept(w, req, &websocket.AcceptOptions{OriginPatterns: h.origins})
	if err != nil {
		return // Accept already wrote the HTTP error
	}
	c.SetReadLimit(readLimit)
	h.serve(req.Context(), code, queued, c)
}

// join finds or creates the room for code and joins p, retrying once if the
// room closed concurrently. r is nil when the hub is full.
func (h *hub) join(code string, p peer) (r *room, slot int, start bool) {
	for range 2 {
		if r = h.roomFor(code); r == nil {
			return nil, -1, false
		}
		if slot, start = r.join(p); slot >= 0 || !r.isClosed() {
			return r, slot, start
		}
	}
	return r, -1, false
}

// serve joins the room and runs the read loop until the peer disconnects.
func (h *hub) serve(ctx context.Context, code string, queued bool, c *websocket.Conn) {
	p := wsPeer{c}
	var r *room
	var slot int
	var start bool
	if queued {
		code, r, slot, start = h.joinQueue(p)
	} else {
		r, slot, start = h.join(code, p)
	}
	if r == nil {
		p.close(closeBusy, "server full")
		return
	}
	if slot < 0 {
		p.close(closeFull, "room full")
		return
	}
	defer func() { r.leave(slot); h.forget(code, r); c.CloseNow() }()
	if start {
		h.announce(r)
	} else {
		noRival := time.AfterFunc(h.wait, func() {
			if r.expireIfWaiting() {
				p.close(closeNoRival, "no rival")
			}
		})
		defer noRival.Stop()
	}
	h.readLoop(ctx, code, slot, r, c)
}

// readLoop applies INPUT/answers PING until the peer disconnects, idles out,
// floods or sends garbage.
func (h *hub) readLoop(ctx context.Context, code string, slot int, r *room, c *websocket.Conn) {
	p := wsPeer{c}
	// Idle only counts once the match runs: a waiting host often hides the tab
	// to share the link (no rAF, no PING); the no-rival timeout covers that.
	var lastSeen atomic.Int64
	lastSeen.Store(time.Now().UnixNano())
	done := make(chan struct{})
	defer close(done)
	go h.watchIdle(r, p, &lastSeen, done)
	var rate rateWindow
	pong := make([]byte, pingMsgLen)
	for {
		typ, b, err := c.Read(ctx)
		if err != nil {
			return
		}
		lastSeen.Store(time.Now().UnixNano())
		if !rate.allow(time.Now()) {
			h.logger.Printf("room %s slot %d: rate limit", code, slot)
			p.close(closeRate, "rate limit")
			return
		}
		if typ == websocket.MessageBinary && isPing(b) {
			_ = p.send(encodePong(pong, b))
			continue
		}
		in, err := decodeInput(b)
		if typ != websocket.MessageBinary || err != nil {
			h.logger.Printf("room %s slot %d: bad message (type %v, err %v)", code, slot, typ, err)
			_ = c.Close(websocket.StatusUnsupportedData, "bad message")
			return
		}
		r.setInput(slot, in)
	}
}

// announce sends WELCOME to both peers and starts the match loop.
func (h *hub) announce(r *room) {
	r.mu.Lock()
	p0, p1 := r.peers[0], r.peers[1]
	r.mu.Unlock()
	sendTo(p0, encodeWelcome(0)) // nil if that peer already left
	sendTo(p1, encodeWelcome(1))
	tick, stop := h.ticker()
	go func() { defer stop(); r.run(tick) }()
}

// watchIdle closes p once the match runs and nothing arrived for h.idle.
func (h *hub) watchIdle(r *room, p peer, lastSeen *atomic.Int64, done <-chan struct{}) {
	t := time.NewTimer(h.idle)
	defer t.Stop()
	for {
		select {
		case <-done:
			return
		case <-t.C:
		}
		if !r.isStarted() {
			lastSeen.Store(time.Now().UnixNano()) // queue/lobby wait never counts as idle
		}
		left := h.idle - time.Since(time.Unix(0, lastSeen.Load()))
		if left <= 0 && r.isStarted() {
			p.close(closeIdle, "idle")
			return
		}
		t.Reset(max(left, h.idle/4))
	}
}
