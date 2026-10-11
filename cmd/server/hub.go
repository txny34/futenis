package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"regexp"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/txny34/futenis/cmd/server/internal/auth"
	"github.com/txny34/futenis/cmd/server/internal/lobby"
	"github.com/txny34/futenis/cmd/server/internal/match"
	"github.com/txny34/futenis/cmd/server/internal/netlimit"
	"github.com/txny34/futenis/cmd/server/internal/ranked"
	"github.com/txny34/futenis/sim"
)

const (
	writeTimeout = 100 * time.Millisecond
	readLimit    = 1024
	maxRooms     = 256
)

var (
	roomCode    = regexp.MustCompile(`^[A-Z0-9]{3,12}$`)
	rejoinToken = regexp.MustCompile(`^[0-9a-f]{32}$`)
)

// hub maps room codes to rooms.
type hub struct {
	mu       sync.Mutex
	rooms    map[string]*match.Room
	origins  []string
	proxied  bool                              // trust X-Forwarded-For (behind the hosting proxy)
	ticker   func() (<-chan time.Time, func()) // injectable for tests
	logger   *log.Logger
	maxRooms int
	conns    *netlimit.ConnLimiter
	idle     time.Duration // injectable for tests
	wait     time.Duration

	fill        time.Duration // 2v2: wait before bots fill the empty seats (injectable)
	queueSeq    int           // matchmaking room counter (queue.go)
	queues      map[string]*queue
	matching    bool                              // ranked rebalancing loop started (queue_match.go)
	matchTicker func() (<-chan time.Time, func()) // injectable for tests

	auth     *auth.Verifier    // Supabase token check; nil = ranked disabled
	active   map[string]uint64 // accounts with a live ranked connection (one each) -> claim generation
	claimSeq uint64
	ratings  ranked.Store // ranked ELO store; nil = ranked disabled

	seats map[string]seatRef // rejoin token -> held seat (internal/match/rejoin.go)
	grace time.Duration      // how long a dropped seat is held (injectable)

	lobbies     []*lobby.Lobby                    // shared online lobbies (internal/lobby), opened on demand
	lobbyConns  *netlimit.ConnLimiter             // per-client cap for /lobby (stricter than matches)
	lobbyTicker func() (<-chan time.Time, func()) // injectable for tests
	lobbyIdle   time.Duration
	done        chan struct{} // never closed in production; stops the lobby loop in tests
}

func newHub(origins []string, logger *log.Logger) *hub {
	return &hub{
		rooms:    map[string]*match.Room{},
		origins:  origins,
		logger:   logger,
		ticker:   realTicker,
		maxRooms: maxRooms,
		conns:    netlimit.NewConnLimiter(maxConns, maxConnsPerIP),
		idle:     idleTimeout,
		wait:     rivalTimeout,
		fill:     fillWait,
		queues:   map[string]*queue{},
		seats:    map[string]seatRef{},
		grace:    match.RejoinGrace,
		active:   map[string]uint64{},

		matchTicker: realMatchTicker,
		lobbyConns:  netlimit.NewConnLimiter(lobby.Max*lobby.MaxShards, lobby.MaxPerClient),
		lobbyTicker: realLobbyTicker,
		lobbyIdle:   lobby.Idle,
		done:        make(chan struct{}),
	}
}

func realTicker() (<-chan time.Time, func()) {
	t := time.NewTicker(match.FrameInterval)
	return t.C, t.Stop
}

// roomFor returns the open room for code, creating it if missing or closed.
// It returns nil when a new room is needed but the hub is at capacity.
func (h *hub) roomFor(code string, mode sim.Mode) *match.Room {
	h.mu.Lock()
	defer h.mu.Unlock()
	r := h.rooms[code]
	if r == nil || r.IsClosed() {
		if r == nil && len(h.rooms) >= h.maxRooms {
			return nil
		}
		r = match.NewRoom(mode, code)
		h.rooms[code] = r
	}
	return r
}

func (h *hub) forget(code string, r *match.Room) {
	h.mu.Lock()
	if h.rooms[code] == r {
		delete(h.rooms, code)
	}
	for _, t := range r.TokenList() {
		if h.seats[t].r == r {
			delete(h.seats, t)
		}
	}
	h.mu.Unlock()
}

// wsPeer adapts a websocket.Conn to peer.
type wsPeer struct{ c *websocket.Conn }

func (p wsPeer) Send(b []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
	defer cancel()
	return p.c.Write(ctx, websocket.MessageBinary, b)
}

func (p wsPeer) Close(code int, reason string) { _ = p.c.Close(websocket.StatusCode(code), reason) }

func (h *hub) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	q := req.URL.Query()
	code, queued, rejoin := q.Get("room"), q.Get("queue"), q.Get("rejoin")
	if q.Has("rejoin") && !rejoinToken.MatchString(rejoin) {
		http.Error(w, "bad rejoin token", http.StatusBadRequest)
		return
	}
	if _, ok := queueModes[queued]; q.Has("queue") && !ok {
		http.Error(w, "bad queue", http.StatusBadRequest)
		return
	}
	if queued == "" && rejoin == "" && !roomCode.MatchString(code) {
		http.Error(w, "bad room code", http.StatusBadRequest)
		return
	}
	ip := netlimit.LimitKey(netlimit.ClientIP(req, h.proxied))
	if !h.conns.Acquire(ip) {
		http.Error(w, "too many connections", http.StatusTooManyRequests)
		return
	}
	defer h.conns.Release(ip)
	c, err := websocket.Accept(w, req, &websocket.AcceptOptions{OriginPatterns: h.origins})
	if err != nil {
		return // Accept already wrote the HTTP error
	}
	c.SetReadLimit(readLimit)
	if rejoin != "" {
		h.serveRejoin(req.Context(), rejoin, c)
		return
	}
	h.serve(req.Context(), code, queued, ip, c)
}

var (
	errBusy    = errors.New("hub: at its room cap")
	errRefused = errors.New("hub: no seat (room full or already queued)")
)

// seatErr classifies a join result: no room (hub full) or no seat.
func seatErr(r *match.Room, slot int) error {
	switch {
	case r == nil:
		return errBusy
	case slot < 0:
		return errRefused
	}
	return nil
}

// join finds or creates the room for code and joins p, retrying once if the
// room closed concurrently. r is nil when the hub is full.
func (h *hub) join(code string, mode sim.Mode, p match.Peer) (r *match.Room, slot int, start bool, humans int) {
	for range 2 {
		if r = h.roomFor(code, mode); r == nil {
			return nil, -1, false, 0
		}
		if slot, start, humans = r.Join(p); slot >= 0 || !r.IsClosed() {
			return r, slot, start, humans
		}
	}
	return r, -1, false, 0
}
