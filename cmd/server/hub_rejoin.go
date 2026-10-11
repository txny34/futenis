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
// leaves it for good.
func (h *hub) play(ctx context.Context, code string, slot int, r *room, c *websocket.Conn) {
	defer c.CloseNow()
	lost := h.readLoop(ctx, code, slot, r, c)
	if lost && r.hold(slot, h.grace, func() { h.expireSeat(code, r, slot) }) {
		h.logger.Printf("room %s slot %d: connection lost, seat held", code, slot)
		return
	}
	r.leave(slot)
	if r.isClosed() { // a 2v2 goes on (or keeps waiting) while people remain
		h.forget(code, r)
	}
}

func (h *hub) expireSeat(code string, r *room, slot int) {
	if r.expireAway(slot) && r.isClosed() {
		h.forget(code, r)
	}
}

// serveRejoin puts a dropped player back in its held seat (?rejoin=TOKEN).
func (h *hub) serveRejoin(ctx context.Context, token string, c *websocket.Conn) {
	p := wsPeer{c}
	h.mu.Lock()
	ref, ok := h.seats[token]
	h.mu.Unlock()
	if !ok || !ref.r.reattach(ref.slot, token, p) {
		p.close(closeGone, "match over")
		return
	}
	h.logger.Printf("room %s slot %d: rejoined", ref.r.code, ref.slot)
	h.play(ctx, ref.r.code, ref.slot, ref.r, c)
}
