package main

import (
	"context"
	"log"
	"net/http"
	"regexp"
	"sync"
	"time"

	"github.com/coder/websocket"
)

const (
	writeTimeout = 100 * time.Millisecond
	readLimit    = 1024
	closeFull    = 4001
)

var roomCode = regexp.MustCompile(`^[A-Z0-9]{3,12}$`)

// hub maps room codes to rooms.
type hub struct {
	mu      sync.Mutex
	rooms   map[string]*room
	origins []string
	ticker  func() (<-chan time.Time, func()) // injectable for tests
	logger  *log.Logger
}

func newHub(origins []string, logger *log.Logger) *hub {
	return &hub{rooms: map[string]*room{}, origins: origins, logger: logger, ticker: realTicker}
}

func realTicker() (<-chan time.Time, func()) {
	t := time.NewTicker(frameInterval)
	return t.C, t.Stop
}

// roomFor returns the open room for code, creating it if missing or closed.
func (h *hub) roomFor(code string) *room {
	h.mu.Lock()
	defer h.mu.Unlock()
	r := h.rooms[code]
	if r == nil || r.isClosed() {
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
	code := req.URL.Query().Get("room")
	if !roomCode.MatchString(code) {
		http.Error(w, "bad room code", http.StatusBadRequest)
		return
	}
	c, err := websocket.Accept(w, req, &websocket.AcceptOptions{OriginPatterns: h.origins})
	if err != nil {
		return // Accept already wrote the HTTP error
	}
	c.SetReadLimit(readLimit)
	h.serve(req.Context(), code, c)
}

// serve joins the room and runs the read loop until the peer disconnects.
func (h *hub) serve(ctx context.Context, code string, c *websocket.Conn) {
	p := wsPeer{c}
	r := h.roomFor(code)
	slot := r.join(p)
	if slot < 0 {
		p.close(closeFull, "room full")
		return
	}
	defer func() { r.leave(slot); h.forget(code, r); c.CloseNow() }()
	h.announce(r, p)
	pong := make([]byte, pingMsgLen)
	for {
		_, b, err := c.Read(ctx)
		if err != nil {
			return
		}
		if isPing(b) {
			_ = p.send(encodePong(pong, b))
			continue
		}
		in, err := decodeInput(b)
		if err != nil {
			h.logger.Printf("room %s slot %d: %v", code, slot, err)
			return
		}
		r.setInput(slot, in)
	}
}

// announce sends WAITING to a lone host, or WELCOME to both and starts the match.
func (h *hub) announce(r *room, p peer) {
	if !r.ready() {
		_ = p.send(encodeSignal(msgWaiting))
		return
	}
	r.mu.Lock()
	p0, p1 := r.peers[0], r.peers[1]
	r.mu.Unlock()
	_ = p0.send(encodeWelcome(0))
	_ = p1.send(encodeWelcome(1))
	tick, stop := h.ticker()
	go func() { defer stop(); r.run(tick) }()
}
