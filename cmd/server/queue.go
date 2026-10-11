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

const fillWait = 20 * time.Second

// queue is the open room of one mode and the clients waiting in it (limit
// keys): one queued connection per client, never matched with itself.
type queue struct {
	code   string
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

// queueRoom returns the code the next queued player of mode joins, opening a
// new room when none is open. Codes are "mm-<mode>-<n>": roomCode rejects them,
// so nobody can join a queue room by typing its code. ok is false when key is
// already waiting there.
func (h *hub) queueRoom(mode, key string) (code string, ok bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	q := h.queueFor(mode)
	if r := h.rooms[q.code]; q.code != "" && (r == nil || r.isClosed()) {
		q.code = "" // everyone waiting left
	}
	if q.code == "" {
		h.queueSeq++
		q.code, q.owners = fmt.Sprintf("mm-%s-%x", mode, h.queueSeq), map[string]bool{}
	}
	return q.code, !q.owners[key]
}

// closeQueueRoom stops sending players to code once its match starts or fills.
func (h *hub) closeQueueRoom(mode, code string) {
	h.mu.Lock()
	if q := h.queueFor(mode); q.code == code {
		q.code, q.owners = "", nil
	}
	h.mu.Unlock()
}

// claimWait records key as waiting in code. It fails when the queue moved on
// while key was joining: waiting there would strand the player.
func (h *hub) claimWait(mode, code, key string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	q := h.queueFor(mode)
	if q.code == "" {
		q.code, q.owners = code, map[string]bool{}
	}
	if q.code != code {
		return false
	}
	q.owners[key] = true
	return true
}

// leaveQueue forgets key as a waiter of code (it may queue again).
func (h *hub) leaveQueue(mode, code, key string) {
	h.mu.Lock()
	if q := h.queueFor(mode); q.code == code {
		delete(q.owners, key)
	}
	h.mu.Unlock()
}

// joinQueue seats p in mode's open room, or rejects it (slot -1 with a non-nil
// room: already queued / no room found). Rooms that filled, started or went
// stale concurrently are skipped (bounded retries).
func (h *hub) joinQueue(mode string, p peer, key string) (code string, r *room, slot int, start bool, humans int) {
	for range 3 {
		code, ok := h.queueRoom(mode, key)
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
			r.leave(slot, p)
			if r.isClosed() {
				h.forget(code, r)
			}
		default:
			h.closeQueueRoom(mode, code)
		}
	}
	return code, &room{}, -1, false, 0
}
