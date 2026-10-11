package main

import (
	"context"
	"time"
)

// Ranked rebalancing: two lonely ranked rooms (one person each) never meet on
// their own, since a waiter can't move between rooms. Every matchEvery the
// newer of two lonely rooms whose ratings now fit (the older room's window
// widened) has its waiter closed with closeRequeue; the client queues again at
// once and lands in the older room.
const (
	matchEvery   = time.Second
	closeRequeue = 4009
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
			for mode := range rankedModes {
				h.rebalance(mode, time.Now())
			}
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
	var move []*room
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
		r.requeueAlone()
	}
}

// waitingAlone: not started, exactly one person seated.
func (r *room) waitingAlone() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return !r.started && !r.closed && r.humansLocked() == 1
}

// requeueAlone asks the lone waiter to queue again (closeRequeue).
func (r *room) requeueAlone() {
	r.mu.Lock()
	var p peer
	for _, o := range r.peers {
		if o != nil {
			p = o
		}
	}
	r.mu.Unlock()
	if p != nil {
		go p.close(closeRequeue, "requeue") // the close handshake may wait on the client
	}
}

// ratingFor is the account's current ELO in mode (eloDefault when unknown or unreachable).
func (h *hub) ratingFor(ctx context.Context, mode, uid string) int {
	ctx, cancel := context.WithTimeout(ctx, ratingsTimeout)
	defer cancel()
	got, err := h.ratings.load(ctx, mode, []string{uid})
	if err != nil {
		return eloDefault
	}
	return ratingOf(got, uid)
}
