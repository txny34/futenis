package main

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
	"github.com/txny34/futenis/cmd/server/internal/match"
	"github.com/txny34/futenis/cmd/server/internal/netlimit"
	"github.com/txny34/futenis/cmd/server/internal/ranked"
	"github.com/txny34/futenis/cmd/server/internal/wire"
	"github.com/txny34/futenis/sim"
)

// One connection's life in a match: authenticate and seat it (serve), read
// INPUT/PING until it leaves, and the idle and no-rival timers.

// serve joins the room and runs the read loop until the peer disconnects.
func (h *hub) serve(ctx context.Context, code, queued, key string, c *websocket.Conn) {
	p := wsPeer{c}
	var r *match.Room
	var slot, humans int
	var start, kept bool
	var cl ranked.Claim // ranked reservation (zero outside ranked)
	rankedMode, isRanked := rankedModes[queued]
	network := "" // ranked: the client's limit key, seated next to its account
	if isRanked {
		acc, ok := h.authenticate(ctx, c)
		uid := acc.ID
		if !ok {
			p.Close(closeNoAuth, "sign in required")
			return
		}
		if cl, ok = h.claimRanked(acc); !ok {
			p.Close(closeFull, "already in a ranked match")
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
		elo := ranked.EloDefault
		if isRanked {
			elo = h.ratingFor(ctx, rankedMode, cl.UID)
		}
		code, r, slot, start, humans = h.joinQueue(queued, p, key, elo)
		if isRanked && r != nil && slot >= 0 && !r.MarkRanked(slot, cl, network) {
			r.Leave(slot, p, false)
			if r.IsClosed() {
				h.forget(code, r)
			}
			h.leaveQueue(queued, code, key)
			p.Close(closeFull, "same account or network already in this match")
			return
		}
	} else {
		r, slot, start, humans = h.join(code, sim.Mode1v1, p)
	}
	if r == nil {
		p.Close(closeBusy, "server full")
		return
	}
	if slot < 0 {
		p.Close(closeFull, "room full")
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
func (h *hub) expireAlone(r *match.Room, p match.Peer) (stop func()) {
	var mu sync.Mutex
	var t *time.Timer
	check := func() {
		if r.ExpireIfWaiting() {
			p.Close(closeNoRival, "no rival")
			return
		}
		if _, started := r.StartedAt(); !started && !r.IsClosed() {
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
func (h *hub) readLoop(ctx context.Context, code string, slot int, r *match.Room, c *websocket.Conn) (lost bool) {
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
	pong := make([]byte, wire.PingLen)
	for {
		typ, b, err := c.Read(ctx)
		if err != nil {
			st := websocket.CloseStatus(err) // 1000 leave, 1001 tab closed: both on purpose
			return !kicked.Load() && st != websocket.StatusNormalClosure && st != websocket.StatusGoingAway
		}
		lastSeen.Store(time.Now().UnixNano())
		if !rate.AllowN(time.Now(), maxMsgsPerWindow) {
			h.logger.Printf("room %s slot %d: rate limit", code, slot)
			p.Close(closeRate, "rate limit")
			return false
		}
		if typ == websocket.MessageBinary && wire.IsPing(b) {
			_ = p.Send(wire.EncodePong(pong, b))
			continue
		}
		in, err := wire.DecodeInput(b)
		if typ != websocket.MessageBinary || err != nil {
			h.logger.Printf("room %s slot %d: bad message (type %v, err %v)", code, slot, typ, err)
			_ = c.Close(websocket.StatusUnsupportedData, "bad message")
			return false
		}
		r.SetInput(slot, p, in)
	}
}

// watchIdle closes p once the match runs and nothing arrived for h.idle.
func (h *hub) watchIdle(r *match.Room, p match.Peer, lastSeen *atomic.Int64, kicked *atomic.Bool, done <-chan struct{}) {
	t := time.NewTimer(h.idle)
	defer t.Stop()
	for {
		select {
		case <-done:
			return
		case <-t.C:
		}
		startAt, started := r.StartedAt()
		if r.IsPaused() {
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
			p.Close(closeIdle, "idle")
			return
		}
		t.Reset(max(left, h.idle/4))
	}
}
