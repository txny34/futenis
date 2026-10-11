package main

import (
	"context"
	"time"

	"github.com/txny34/futenis/cmd/server/internal/ranked"
)

// Ranked rebalancing: two lonely ranked rooms (one person each) never meet on
// their own, since a waiter can't move between rooms. Every matchEvery the
// newer of two lonely rooms whose ratings now fit (the older room's window
// widened) has its waiter closed with closeRequeue; the client queues again at
// once and lands in the older room.
const (
	matchEvery          = time.Second
	closeRequeue        = 4009
	ratingLookupTimeout = 2 * time.Second // a slow database must not hold the queue join
)

// startMatcher runs the rebalancing loop once per hub (lazily, on the first ranked queue).
func (h *hub) startMatcherLocked() {
	if h.matching {
		return
	}
	h.matching = true
	tick, stop := h.matchTicker()
	go func() {
		defer stop()
		for {
			select {
			case <-h.done:
				return
			case <-tick:
			}
			h.rebalance(eloQueue, time.Now())
		}
	}()
}

func realMatchTicker() (<-chan time.Time, func()) {
	t := time.NewTicker(matchEvery)
	return t.C, t.Stop
}

// rebalance moves at most one lonely waiter per older lonely room.
func (h *hub) rebalance(mode string, now time.Time) {
	h.mu.Lock()
	q := h.queueFor(mode)
	var lonely []*openRoom // oldest first (q.open is in opening order)
	for _, o := range q.open {
		if r := h.rooms[o.code]; r != nil && !r.isClosed() && r.waitingAlone() {
			lonely = append(lonely, o)
		}
	}
	var move []*room // candidates; requeueAlone re-checks each under its own lock
	taken := map[*openRoom]bool{}
	for i, old := range lonely {
		for _, newer := range lonely[i+1:] {
			if taken[old] || taken[newer] || !old.fits(newer.elo, true, now) {
				continue
			}
			taken[old], taken[newer] = true, true
			move = append(move, h.rooms[newer.code])
			q.remove(newer.code) // the requeued player must not land back here
		}
	}
	h.mu.Unlock()
	for _, r := range move {
		if p, c := r.requeueAlone(); p != nil {
			h.releaseRanked(c)                  // before the close: the client's immediate reconnect must find the account free
			go p.Close(closeRequeue, "requeue") // the close handshake may wait on the client
		}
	}
}

// waitingAlone: not started, exactly one person seated.
func (r *room) waitingAlone() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return !r.started && !r.closed && r.humansLocked() == 1
}

// requeueAlone closes the room iff it still waits with exactly one person
// (atomically with join: a rival arriving now is refused and retries) and
// returns that person and their ranked claim; nil when the room moved on.
func (r *room) requeueAlone() (peer, ranked.Claim) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.started || r.closed || r.humansLocked() != 1 {
		return nil, ranked.Claim{}
	}
	for i, p := range r.peers {
		if p != nil {
			r.closed = true
			close(r.stop)
			r.peers[i] = nil
			return p, r.claims[i]
		}
	}
	return nil, ranked.Claim{}
}

// ratingFor is the account's current ELO in mode (eloDefault when unknown or unreachable).
func (h *hub) ratingFor(ctx context.Context, mode, uid string) int {
	ctx, cancel := context.WithTimeout(ctx, ratingLookupTimeout)
	defer cancel()
	got, err := h.ratings.Load(ctx, mode, []string{uid})
	if err != nil {
		h.logger.Printf("ranked %s: rating lookup: %v (queued as %d)", mode, err, ranked.EloDefault)
		return ranked.EloDefault
	}
	return ranked.RatingOf(got, uid)
}
