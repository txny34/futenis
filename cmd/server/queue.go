package main

import "fmt"

// queueMode is the only matchmaking queue so far (1v1 casual); 2v2 comes next.
const queueMode = "1v1"

// queueRoom returns the code of the room the next queued player joins, opening
// a new one when none is waiting. Codes are "mm-" + a counter: roomCode rejects
// them, so nobody can join a queue room by typing its code. ok is false when key
// is the one already waiting: one queued connection per client, never self-paired.
func (h *hub) queueRoom(key string) (code string, ok bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if r := h.rooms[h.queueCode]; r == nil || r.isClosed() {
		h.queueOwner = "" // the waiter left: their slot is free, even for themselves
	}
	if h.queueCode == "" {
		h.queueSeq++
		h.queueCode = fmt.Sprintf("mm-%x", h.queueSeq)
		h.queueOwner = ""
	}
	return h.queueCode, h.queueOwner == "" || h.queueOwner != key
}

// closeQueueRoom stops sending players to code once its match starts or fills.
func (h *hub) closeQueueRoom(code string) {
	h.mu.Lock()
	if h.queueCode == code {
		h.queueCode, h.queueOwner = "", ""
	}
	h.mu.Unlock()
}

// claimWait records key as the player waiting in code. It fails when the queue
// moved on while key was joining (code filled, ended and was reopened elsewhere):
// waiting there would strand the player alone.
func (h *hub) claimWait(code, key string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.queueCode == "" {
		h.queueCode = code
	}
	if h.queueCode != code {
		return false
	}
	h.queueOwner = key
	return true
}

// joinQueue pairs p with the waiting player, or makes p the one waiting.
// Rooms that filled or went stale concurrently are skipped (bounded retries).
// slot -1 with a non-nil room means "rejected" (already queued or no room found).
func (h *hub) joinQueue(p peer, key string) (code string, r *room, slot int, start bool) {
	for range 3 {
		code, ok := h.queueRoom(key)
		if !ok {
			return code, &room{}, -1, false
		}
		if r, slot, start = h.join(code, p); r == nil {
			return code, nil, -1, false
		}
		switch {
		case slot >= 0 && start:
			h.closeQueueRoom(code)
			return code, r, slot, true
		case slot >= 0 && h.claimWait(code, key):
			return code, r, slot, false
		case slot >= 0:
			r.leave(slot)
			h.forget(code, r)
		default:
			h.closeQueueRoom(code)
		}
	}
	return code, &room{}, -1, false
}
