package main

import (
	"context"

	"github.com/coder/websocket"
)

// seatRef locates a rejoin token's seat.
type seatRef struct {
	r    *room
	slot int
}

// play runs a seated peer until it disconnects. A dropped connection in a
// running match keeps the seat for h.grace (room_rejoin.go); anything else
// leaves it for good. kept reports that the seat is still taken (held, or
// replaced by a newer connection of the same player): its ranked account
// stays claimed until hub.expireSeat or the last play frees it.
func (h *hub) play(ctx context.Context, code string, slot int, r *room, c *websocket.Conn) (kept bool) {
	p := wsPeer{c}
	defer c.CloseNow()
	lost := h.readLoop(ctx, code, slot, r, c)
	if lost && r.hold(slot, p, h.grace, func() { h.expireSeat(code, r, slot) }) {
		h.logger.Printf("room %s slot %d: connection lost, seat held", code, slot)
		return true
	}
	left := r.leave(slot, p, lost)
	closed := r.isClosed()
	if closed { // a 2v2 goes on (or keeps waiting) while people remain
		h.forget(code, r)
	}
	return !left && !closed
}

func (h *hub) expireSeat(code string, r *room, slot int) {
	c := r.seatClaim(slot)
	if !r.expireAway(slot) {
		return // back in time
	}
	h.releaseRanked(c)
	if r.isClosed() {
		h.forget(code, r)
	}
}

// serveRejoin puts a dropped player back in its seat (?rejoin=TOKEN): a held
// seat, or one whose old connection the server still believes alive.
func (h *hub) serveRejoin(ctx context.Context, token string, c *websocket.Conn) {
	p := wsPeer{c}
	h.mu.Lock()
	ref, ok := h.seats[token]
	h.mu.Unlock()
	if !ok || !ref.r.reattach(ref.slot, token, p) {
		p.Close(closeGone, "match over")
		return
	}
	h.logger.Printf("room %s slot %d: rejoined", ref.r.code, ref.slot)
	cl := ref.r.seatClaim(ref.slot)
	if !h.play(ctx, ref.r.code, ref.slot, ref.r, c) {
		h.releaseRanked(cl)
	}
}
