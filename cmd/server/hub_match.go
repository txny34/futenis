package main

import (
	"time"

	"github.com/txny34/futenis/cmd/server/internal/match"
	"github.com/txny34/futenis/cmd/server/internal/wire"
	"github.com/txny34/futenis/sim"
)

// Starting a match: WELCOME + rejoin tokens to every seat, ranked hooks, the
// sim loop, and the 2v2 bot fill after the countdown.

// armFill: the second person in a 2v2 room starts the countdown after which
// bots fill the empty seats.
func (h *hub) armFill(r *match.Room, humans int, queued, code string) {
	if r.Mode() != sim.Mode2v2 || humans != match.MinHumans2v2 {
		return
	}
	time.AfterFunc(h.fill, func() {
		if !r.StartEarly() {
			return
		}
		if queued != "" {
			h.closeQueueRoom(queued, code) // started: no longer the open room
		}
		h.announce(r, "")
	})
}

// announce sends WELCOME to every seated peer and starts the match loop.
// rankedMode ("1v1"/"2v2", "" = casual) wires the ELO report at game over.
func (h *hub) announce(r *match.Room, rankedMode string) {
	if rankedMode != "" {
		r.SetRankedHooks(func(w int, users [sim.MaxPlayers]string) { h.reportRanked(rankedMode, r, users, w) }, h.releaseRanked)
	}
	peers := r.Peers()
	tokens := r.IssueTokens()
	h.mu.Lock()
	for i, p := range peers {
		if p != nil {
			h.seats[tokens[i]] = seatRef{r: r, slot: i}
		}
	}
	h.mu.Unlock()
	for i, p := range peers {
		match.SendTo(p, wire.EncodeWelcome(i)) // nil if that seat is empty
		if p != nil {
			match.SendTo(p, wire.EncodeSeat(tokens[i]))
		}
	}
	tick, stop := h.ticker()
	go func() { defer stop(); r.Run(tick) }()
}
