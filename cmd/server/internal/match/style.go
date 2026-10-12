package match

import (
	"github.com/txny34/futenis/cmd/server/internal/wire"
	"github.com/txny34/futenis/sim"
)

// Players' looks (hair, cap, colors): each seat sends its own STYLE, the room
// keeps the latest per seat and relays all of them (STYLES) whenever one
// changes, so everyone sees everyone's look. Visual only: never reaches the sim.

// SetStyle records slot's look from p (ignored once p was replaced) and tells
// every seated peer when it changed.
func (r *Room) SetStyle(slot int, p Peer, st wire.Style) {
	r.mu.Lock()
	if r.closed || r.peers[slot] != p || r.styles[slot] == st {
		r.mu.Unlock()
		return
	}
	r.styles[slot] = st
	msg, peers := wire.EncodeStyles(r.styles), r.peers
	r.mu.Unlock()
	sendAll(peers, msg)
}

// clearStyleLocked forgets slot's look (the bot takes the seat) and returns
// the STYLES message for the peers still seated.
func (r *Room) clearStyleLocked(slot int) []byte {
	r.styles[slot] = wire.Style{}
	return wire.EncodeStyles(r.styles)
}

func sendAll(peers [sim.MaxPlayers]Peer, msg []byte) {
	for _, o := range peers {
		SendTo(o, msg)
	}
}
