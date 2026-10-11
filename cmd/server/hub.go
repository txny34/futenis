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
	"github.com/txny34/futenis/cmd/server/internal/netlimit"
	"github.com/txny34/futenis/sim"
)

const (
	writeTimeout = 100 * time.Millisecond
	readLimit    = 1024
	closeFull    = 4001
	closeBusy    = 4002
	maxRooms     = 256
)

var (
	roomCode    = regexp.MustCompile(`^[A-Z0-9]{3,12}$`)
	rejoinToken = regexp.MustCompile(`^[0-9a-f]{32}$`)
)

// hub maps room codes to rooms.
type hub struct {
	mu       sync.Mutex
	rooms    map[string]*room
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

	auth     *verifier         // Supabase token check; nil = ranked disabled
	active   map[string]uint64 // accounts with a live ranked connection (one each) -> claim generation
	claimSeq uint64
	ratings  ratingStore // ranked ELO store; nil = ranked disabled

	seats map[string]seatRef // rejoin token -> held seat (room_rejoin.go)
	grace time.Duration      // how long a dropped seat is held (injectable)

	lobbies     []*lobby                          // shared online lobbies (lobby.go), opened on demand
	lobbyConns  *netlimit.ConnLimiter             // per-client cap for /lobby (stricter than matches)
	lobbyTicker func() (<-chan time.Time, func()) // injectable for tests
	lobbyIdle   time.Duration
	done        chan struct{} // never closed in production; stops the lobby loop in tests
}

func newHub(origins []string, logger *log.Logger) *hub {
	return &hub{rooms: map[string]*room{}, origins: origins, logger: logger, ticker: realTicker, maxRooms: maxRooms,
		conns: netlimit.NewConnLimiter(maxConns, maxConnsPerIP), idle: idleTimeout, wait: rivalTimeout, fill: fillWait,
		queues: map[string]*queue{}, seats: map[string]seatRef{}, grace: rejoinGrace, active: map[string]uint64{}, lobbyConns: netlimit.NewConnLimiter(maxLobby*maxLobbyShard, maxLobbyPerIP), lobbyTicker: realLobbyTicker, matchTicker: realMatchTicker, lobbyIdle: lobbyIdle, done: make(chan struct{})}
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
		r.code = code
		h.rooms[code] = r
	}
	return r
}

func (h *hub) forget(code string, r *room) {
	h.mu.Lock()
	if h.rooms[code] == r {
		delete(h.rooms, code)
	}
	for _, t := range r.tokenList() {
		if h.seats[t].r == r {
			delete(h.seats, t)
		}
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
	var start, kept bool
	var cl claim // ranked reservation (zero outside ranked)
	rankedMode, ranked := rankedModes[queued]
	network := "" // ranked: the client's limit key, seated next to its account
	if ranked {
		acc, ok := h.authenticate(ctx, c)
		uid := acc.id
		if !ok {
			p.close(closeNoAuth, "sign in required")
			return
		}
		if cl, ok = h.claimRanked(acc); !ok {
			p.close(closeFull, "already in a ranked match")
			return
		}
		defer func() {
			if !kept { // a held or replaced seat keeps the account claimed; hub.expireSeat frees it
				h.releaseRanked(cl)
			}
		}()
		network = key
		key = "user:" + uid // one queued connection per account
	}
	if queued != "" {
		elo := eloDefault
		if ranked {
			elo = h.ratingFor(ctx, rankedMode, cl.uid)
		}
		code, r, slot, start, humans = h.joinQueue(queued, p, key, elo)
		if ranked && r != nil && slot >= 0 && !r.markRanked(slot, cl, network) {
			r.leave(slot, p, false)
			if r.isClosed() {
				h.forget(code, r)
			}
			h.leaveQueue(queued, code, key)
			p.close(closeFull, "same account or network already in this match")
			return
		}
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
	if start {
		h.announce(r, rankedMode)
	} else {
		if queued != "" {
			defer h.leaveQueue(queued, code, key) // only a seated waiter owns a queue spot
		}
		h.armFill(r, humans, queued, code)
		stop := h.expireAlone(r, p)
		defer stop()
	}
	kept = h.play(ctx, code, slot, r, c)
}

// expireAlone closes p's room if nobody else shows up within h.wait. While a
// 2v2 room has enough people for the bot fill it re-checks later instead of
// giving up, so a waiter whose company left is never stranded. stop cancels it.
func (h *hub) expireAlone(r *room, p peer) (stop func()) {
	var mu sync.Mutex
	var t *time.Timer
	check := func() {
		if r.expireIfWaiting() {
			p.close(closeNoRival, "no rival")
			return
		}
		if _, started := r.startedAt(); !started && !r.isClosed() {
			mu.Lock()
			t.Reset(h.wait)
			mu.Unlock()
		}
	}
	mu.Lock()
	t = time.AfterFunc(h.wait, check)
	mu.Unlock()
	return func() { mu.Lock(); t.Stop(); mu.Unlock() }
}

// readLoop applies INPUT/answers PING until the peer disconnects, idles out,
// floods or sends garbage.
// It reports lost when the connection dropped (not a normal close, nor one the
// server made): the seat may then be held for a rejoin.
func (h *hub) readLoop(ctx context.Context, code string, slot int, r *room, c *websocket.Conn) (lost bool) {
	p := wsPeer{c}
	// Idle only counts once the match runs: a waiting host often hides the tab
	// to share the link (no rAF, no PING); the no-rival timeout covers that.
	var lastSeen atomic.Int64
	lastSeen.Store(time.Now().UnixNano())
	done := make(chan struct{})
	defer close(done)
	var kicked atomic.Bool
	go h.watchIdle(r, p, &lastSeen, &kicked, done)
	var rate netlimit.RateWindow
	pong := make([]byte, pingMsgLen)
	for {
		typ, b, err := c.Read(ctx)
		if err != nil {
			st := websocket.CloseStatus(err) // 1000 leave, 1001 tab closed: both on purpose
			return !kicked.Load() && st != websocket.StatusNormalClosure && st != websocket.StatusGoingAway
		}
		lastSeen.Store(time.Now().UnixNano())
		if !rate.AllowN(time.Now(), maxMsgsPerWindow) {
			h.logger.Printf("room %s slot %d: rate limit", code, slot)
			p.close(closeRate, "rate limit")
			return false
		}
		if typ == websocket.MessageBinary && isPing(b) {
			_ = p.send(encodePong(pong, b))
			continue
		}
		in, err := decodeInput(b)
		if typ != websocket.MessageBinary || err != nil {
			h.logger.Printf("room %s slot %d: bad message (type %v, err %v)", code, slot, typ, err)
			_ = c.Close(websocket.StatusUnsupportedData, "bad message")
			return false
		}
		r.setInput(slot, p, in)
	}
}

// armFill: the second person in a 2v2 room starts the countdown after which
// bots fill the empty seats.
func (h *hub) armFill(r *room, humans int, queued, code string) {
	if r.mode != sim.Mode2v2 || humans != minHumans2v2 {
		return
	}
	time.AfterFunc(h.fill, func() {
		if !r.startEarly() {
			return
		}
		if queued != "" {
			h.closeQueueRoom(queued, code) // started: no longer the open room
		}
		h.announce(r, "")
	})
}

// announce sends WELCOME to every seated peer and starts the match loop.
// rankedMode ("1v1"/"2v2", "" = casual) wires the ELO report at game over.
func (h *hub) announce(r *room, rankedMode string) {
	r.mu.Lock()
	peers := r.peers
	if rankedMode != "" && r.ranked {
		r.onOver = func(w int, users [sim.MaxPlayers]string) { h.reportRanked(rankedMode, r, users, w) }
		r.release = h.releaseRanked
	}
	r.mu.Unlock()
	tokens := r.issueTokens()
	h.mu.Lock()
	for i, p := range peers {
		if p != nil {
			h.seats[tokens[i]] = seatRef{r: r, slot: i}
		}
	}
	h.mu.Unlock()
	for i, p := range peers {
		sendTo(p, encodeWelcome(i)) // nil if that seat is empty
		if p != nil {
			sendTo(p, encodeSeat(tokens[i]))
		}
	}
	tick, stop := h.ticker()
	go func() { defer stop(); r.run(tick) }()
}

// watchIdle closes p once the match runs and nothing arrived for h.idle.
func (h *hub) watchIdle(r *room, p peer, lastSeen *atomic.Int64, kicked *atomic.Bool, done <-chan struct{}) {
	t := time.NewTimer(h.idle)
	defer t.Stop()
	for {
		select {
		case <-done:
			return
		case <-t.C:
		}
		startAt, started := r.startedAt()
		if r.isPaused() {
			lastSeen.Store(time.Now().UnixNano()) // waiting for a dropped rival never counts as idle
		}
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
			kicked.Store(true)
			p.close(closeIdle, "idle")
			return
		}
		t.Reset(max(left, h.idle/4))
	}
}
