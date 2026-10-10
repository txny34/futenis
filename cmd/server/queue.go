package main

import "fmt"

// queueMode is the only matchmaking queue so far (1v1 casual); 2v2 comes next.
const queueMode = "1v1"

// queueRoom returns the code of the room the next queued player joins, opening
// a new one when none is waiting. Codes are "mm-" + a counter: roomCode rejects
// them, so nobody can join a queue room by typing its code.
func (h *hub) queueRoom() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.queueCode == "" {
		h.queueSeq++
		h.queueCode = fmt.Sprintf("mm-%x", h.queueSeq)
	}
	return h.queueCode
}

// closeQueueRoom stops sending players to code once its match starts or fills.
func (h *hub) closeQueueRoom(code string) {
	h.mu.Lock()
	if h.queueCode == code {
		h.queueCode = ""
	}
	h.mu.Unlock()
}

// joinQueue pairs p with the waiting player, or makes p the one waiting.
// A room that filled concurrently is skipped (bounded retries).
func (h *hub) joinQueue(p peer) (code string, r *room, slot int, start bool) {
	for range 3 {
		code = h.queueRoom()
		if r, slot, start = h.join(code, p); r == nil {
			return code, nil, -1, false
		}
		if slot >= 0 {
			if start {
				h.closeQueueRoom(code)
			}
			return code, r, slot, start
		}
		h.closeQueueRoom(code)
	}
	return code, r, -1, false
}
