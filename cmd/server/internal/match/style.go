package match

import (
	"time"

	"github.com/txny34/futenis/cmd/server/internal/wire"
	"github.com/txny34/futenis/sim"
)

// Players' looks (hair, cap, colors): each seat sends its own STYLE, the room
// keeps the latest per seat and relays all of them (STYLES) whenever one
// changes, so everyone sees everyone's look. Visual only: never reaches the sim.

// styleEvery: at most one look change per seat in this time. Each relay
// makes every client rebuild meshes; looks change in the Estilos modal, rarely.
const styleEvery = 300 * time.Millisecond

// SetStyle records slot's look from p (ignored once p was replaced, before
// the match starts: WELCOME comes first, or within styleEvery of the last
// change) and tells every seated peer.
func (r *Room) SetStyle(slot int, p Peer, st wire.Style, now time.Time) {
	r.mu.Lock()
	if r.closed || !r.started || r.peers[slot] != p || r.styles[slot] == st || now.Sub(r.styleAt[slot]) < styleEvery {
		r.mu.Unlock()
		return
	}
	r.styles[slot], r.styleAt[slot] = st, now
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
