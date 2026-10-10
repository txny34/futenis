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
	"github.com/txny34/futenis/sim"
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

	fill     time.Duration // 2v2: wait before bots fill the empty seats (injectable)
	queueSeq int           // matchmaking room counter (queue.go)
	queues   map[string]*queue
}

func newHub(origins []string, logger *log.Logger) *hub {
	return &hub{rooms: map[string]*room{}, origins: origins, logger: logger, ticker: realTicker, maxRooms: maxRooms,
		conns: newConnLimiter(maxConns, maxConnsPerIP), idle: idleTimeout, wait: rivalTimeout, fill: fillWait,
		queues: map[string]*queue{}}
}

func realTicker() (<-chan time.Time, func()) {
	t := time.NewTicker(frameInterval)
	return t.C, t.Stop
}

// roomFor returns the open room for code, creating it if missing or closed.
// It returns nil when a new room is needed but the hub is at capacity.
func (h *hub) roomFor(code string, mode sim.Mode) *room {
	h.mu.Lock()
	defer h.mu.Unlock()
	r := h.rooms[code]
	if r == nil || r.isClosed() {
		if r == nil && len(h.rooms) >= h.maxRooms {
			return nil
		}
		r = newRoom(mode)
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
	code, queued := q.Get("room"), q.Get("queue")
	if _, ok := queueModes[queued]; q.Has("queue") && !ok {
		http.Error(w, "bad queue", http.StatusBadRequest)
		return
	}
	if queued == "" && !roomCode.MatchString(code) {
		http.Error(w, "bad room code", http.StatusBadRequest)
		return
	}
	ip := limitKey(clientIP(req, h.proxied))
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
	h.serve(req.Context(), code, queued, ip, c)
}

// join finds or creates the room for code and joins p, retrying once if the
// room closed concurrently. r is nil when the hub is full.
func (h *hub) join(code string, mode sim.Mode, p peer) (r *room, slot int, start bool, humans int) {
	for range 2 {
		if r = h.roomFor(code, mode); r == nil {
			return nil, -1, false, 0
		}
		if slot, start, humans = r.join(p); slot >= 0 || !r.isClosed() {
			return r, slot, start, humans
		}
	}
	return r, -1, false, 0
}

// serve joins the room and runs the read loop until the peer disconnects.
func (h *hub) serve(ctx context.Context, code, queued, key string, c *websocket.Conn) {
	p := wsPeer{c}
	var r *room
	var slot, humans int
	var start bool
	if queued != "" {
		code, r, slot, start, humans = h.joinQueue(queued, p, key)
		defer h.leaveQueue(queued, code, key)
	} else {
		r, slot, start, humans = h.join(code, sim.Mode1v1, p)
	}
	if r == nil {
		p.close(closeBusy, "server full")
		return
	}
	if slot < 0 {
		p.close(closeFull, "room full")
		return
	}
	defer func() {
		r.leave(slot)
		if r.isClosed() { // a 2v2 goes on (or keeps waiting) while people remain
			h.forget(code, r)
		}
		c.CloseNow()
	}()
	if start {
		h.announce(r)
	} else {
		h.armFill(r, humans)
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

// armFill: the second person in a 2v2 room starts the countdown after which
// bots fill the empty seats.
func (h *hub) armFill(r *room, humans int) {
	if r.mode != sim.Mode2v2 || humans != minHumans2v2 {
		return
	}
	time.AfterFunc(h.fill, func() {
		if r.startEarly() {
			h.announce(r)
		}
	})
}

// announce sends WELCOME to every seated peer and starts the match loop.
func (h *hub) announce(r *room) {
	r.mu.Lock()
	peers := r.peers
	r.mu.Unlock()
	for i, p := range peers {
		sendTo(p, encodeWelcome(i)) // nil if that seat is empty
	}
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
		startAt, started := r.startedAt()
		if !started {
			t.Reset(h.idle / 4) // waiting for a rival never counts as idle
			continue
		}
		// The clock runs from the later of the last message and the match start.
		last := time.Unix(0, lastSeen.Load())
		if startAt.After(last) {
			last = startAt
		}
		left := h.idle - time.Since(last)
		if left <= 0 {
			p.close(closeIdle, "idle")
			return
		}
		t.Reset(max(left, h.idle/4))
	}
}
