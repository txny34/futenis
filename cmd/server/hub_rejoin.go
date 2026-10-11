package main

import (
	"context"

	"github.com/coder/websocket"
	"github.com/txny34/futenis/cmd/server/internal/match"
	"github.com/txny34/futenis/cmd/server/internal/wire"
)

// seatRef locates a rejoin token's seat.
type seatRef struct {
	r    *match.Room
	slot int
}

// play runs a seated peer until it disconnects. A dropped connection in a
// running match keeps the seat for h.grace (internal/match/rejoin.go); anything else
// leaves it for good. kept reports that the seat is still taken (held, or
// replaced by a newer connection of the same player): its ranked account
// stays claimed until hub.expireSeat or the last play frees it.
func (h *hub) play(ctx context.Context, code string, slot int, r *match.Room, c *websocket.Conn) (kept bool) {
	p := wsPeer{c}
	defer c.CloseNow()
	lost := h.readLoop(ctx, code, slot, r, c)
	if lost && r.Hold(slot, p, h.grace, func() { h.expireSeat(code, r, slot) }) {
		h.logger.Printf("room %s slot %d: connection lost, seat held", code, slot)
		return true
	}
	left := r.Leave(slot, p, lost)
	closed := r.IsClosed()
	if closed { // a 2v2 goes on (or keeps waiting) while people remain
		h.forget(code, r)
	}
	return !left && !closed
}

func (h *hub) expireSeat(code string, r *match.Room, slot int) {
	c := r.SeatClaim(slot)
	if !r.ExpireAway(slot) {
		return // back in time
	}
	h.releaseRanked(c)
	if r.IsClosed() {
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
	if !ok || !ref.r.Reattach(ref.slot, token, p) {
		p.Close(wire.CloseGone, "match over")
		return
	}
	h.logger.Printf("room %s slot %d: rejoined", ref.r.Code(), ref.slot)
	cl := ref.r.SeatClaim(ref.slot)
	if !h.play(ctx, ref.r.Code(), ref.slot, ref.r, c) {
		h.releaseRanked(cl)
	}
}
