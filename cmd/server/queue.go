package main

import (
	"fmt"
	"time"

	"github.com/txny34/futenis/sim"
)

// Matchmaking: one queue per mode (/ws?queue=1v1 | 2v2). Each queue has at
// most one room open for joiners. A 2v2 room starts when full, or fillWait
// after its second person arrives (bots take the empty seats).
var queueModes = map[string]sim.Mode{
	"1v1": sim.Mode1v1, "2v2": sim.Mode2v2, "ranked1v1": sim.Mode1v1, "ranked2v2": sim.Mode2v2,
}

const (
	fillWait = 20 * time.Second

	// Ranked 1v1 pairing by ELO: a waiting room takes a newcomer whose rating
	// is within eloGapStart, widening by eloGapPerSec while it waits; after
	// eloGapOpen anyone fits. Other queues ignore ratings (one open room):
	// split 2v2 groups could otherwise wait for each other forever.
	eloGapStart  = 150
	eloGapPerSec = 25
	eloGapOpen   = 30 * time.Second
	maxOpenRooms = 64 // per queue: beyond this a newcomer joins the closest room anyway
	// staleOpen: a chosen room whose first player never arrived (joining takes
	// microseconds; a fresh entry has no room until then) is dropped after this.
	staleOpen = 5 * time.Second
)

// queue holds the rooms of one mode open for joiners and who waits in each
// (limit keys): one queued connection per client, never matched with itself.
type queue struct{ open []*openRoom }

type openRoom struct {
	code   string
	elo    int // rating of who opened it (ranked)
	since  time.Time
	owners map[string]bool
}

func (h *hub) queueFor(mode string) *queue {
	q := h.queues[mode]
	if q == nil {
		q = &queue{}
		h.queues[mode] = q
	}
	return q
}

func (q *queue) find(code string) (int, *openRoom) {
	for i, o := range q.open {
		if o.code == code {
			return i, o
		}
	}
	return -1, nil
}

func (q *queue) remove(code string) {
	if i, _ := q.find(code); i >= 0 {
		q.open = append(q.open[:i], q.open[i+1:]...)
	}
}

// eloQueue is the only queue paired by rating.
const eloQueue = "ranked1v1"

// fits reports whether a player rated elo may join o now.
func (o *openRoom) fits(elo int, byElo bool, now time.Time) bool {
	if !byElo {
		return true
	}
	waited := now.Sub(o.since)
	gap := eloGapStart + int(waited.Seconds()*eloGapPerSec)
	return waited >= eloGapOpen || abs(elo-o.elo) <= gap
}

func abs(v int) int { return max(v, -v) }

// queueRoom returns the code the next queued player of mode (rated elo)
// joins: the open room closest in rating that fits, else a new one. Codes are
// "mm-<mode>-<n>": roomCode rejects them, so nobody can join a queue room by
// typing its code. ok is false when key already waits in the chosen room.
func (h *hub) queueRoom(mode, key string, elo int) (code string, ok bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	q := h.queueFor(mode)
	byElo := mode == eloQueue
	if byElo {
		h.startMatcherLocked()
	}
	now := time.Now()
	live := q.open[:0]
	for _, o := range q.open {
		if r := h.rooms[o.code]; (r == nil && now.Sub(o.since) > staleOpen) || (r != nil && r.isClosed()) {
			continue // everyone waiting there left
		}
		live = append(live, o)
	}
	q.open = live
	var best *openRoom
	for _, o := range q.open {
		if o.owners[key] {
			return o.code, false
		}
		if (o.fits(elo, byElo, now) || len(q.open) >= maxOpenRooms) && (best == nil || abs(elo-o.elo) < abs(elo-best.elo)) {
			best = o
		}
	}
	if best == nil {
		h.queueSeq++
		best = &openRoom{code: fmt.Sprintf("mm-%s-%x", mode, h.queueSeq), elo: elo, since: now, owners: map[string]bool{}}
		q.open = append(q.open, best)
	}
	return best.code, true
}

// closeQueueRoom stops sending players to code once its match starts or fills.
func (h *hub) closeQueueRoom(mode, code string) {
	h.mu.Lock()
	h.queueFor(mode).remove(code)
	h.mu.Unlock()
}

// claimWait records key as waiting in code. It fails when the room stopped
// taking players while key was joining: waiting there would strand the player.
func (h *hub) claimWait(mode, code, key string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	_, o := h.queueFor(mode).find(code)
	if o == nil {
		return false
	}
	o.owners[key] = true
	return true
}

// leaveQueue forgets key as a waiter of code (it may queue again).
func (h *hub) leaveQueue(mode, code, key string) {
	h.mu.Lock()
	if _, o := h.queueFor(mode).find(code); o != nil {
		delete(o.owners, key)
	}
	h.mu.Unlock()
}

// joinQueue seats p in mode's open room, or rejects it (slot -1 with a non-nil
// room: already queued / no room found). Rooms that filled, started or went
// stale concurrently are skipped (bounded retries).
func (h *hub) joinQueue(mode string, p peer, key string, elo int) (code string, r *room, slot int, start bool, humans int) {
	for range 3 {
		code, ok := h.queueRoom(mode, key, elo)
		if !ok {
			return code, &room{}, -1, false, 0
		}
		if r, slot, start, humans = h.join(code, queueModes[mode], p); r == nil {
			return code, nil, -1, false, 0
		}
		switch {
		case slot >= 0 && start:
			h.closeQueueRoom(mode, code)
			return code, r, slot, true, humans
		case slot >= 0 && h.claimWait(mode, code, key):
			return code, r, slot, false, humans
		case slot >= 0:
			r.leave(slot, p, false)
			if r.isClosed() {
				h.forget(code, r)
			}
		default:
			h.closeQueueRoom(mode, code)
		}
	}
	return code, &room{}, -1, false, 0
}
