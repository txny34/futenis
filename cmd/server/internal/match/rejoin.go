package match

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/txny34/futenis/cmd/server/internal/ranked"
	"github.com/txny34/futenis/cmd/server/internal/wire"
	"github.com/txny34/futenis/sim"
)

// Reconnection: a started match keeps a dropped player's seat for RejoinGrace.
// 1v1 and ranked matches pause meanwhile (a bot never plays someone's seat
// there); casual 2v2 goes on with the bot in that seat. The player comes back
// with the seat's secret token (SEAT message, sent after WELCOME), also when
// the server has not noticed the drop yet (the new connection replaces the old).
const (
	RejoinGrace = 20 * time.Second
	MaxHolds    = 4 // per room: pauses can't be chained to stall a match
	tokenBytes  = 16
	TokenLen    = 2 * tokenBytes // hex
)

func newToken() string {
	b := make([]byte, tokenBytes)
	_, _ = rand.Read(b) // crypto/rand.Read never fails (Go ≥ 1.24)
	return hex.EncodeToString(b)
}

// IssueTokens gives every seated person a fresh rejoin token.
func (r *Room) IssueTokens() [sim.MaxPlayers]string {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, p := range r.peers {
		if p != nil {
			r.tokens[i] = newToken()
		}
	}
	return r.tokens
}

// TokenList is a copy of the seats' rejoin tokens ("" = none issued).
func (r *Room) TokenList() [sim.MaxPlayers]string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.tokens
}

// SeatClaim is slot's ranked reservation (zero outside ranked).
func (r *Room) SeatClaim(slot int) ranked.Claim {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.claims[slot]
}

// Hold keeps slot's seat after p's connection dropped. It refuses (the caller
// then leaves for good) when p no longer owns the seat, the match is not
// running or is over, nobody else is connected, or the room used up its holds.
// expire runs after grace unless the player is back by then.
func (r *Room) Hold(slot int, p Peer, grace time.Duration, expire func()) bool {
	r.mu.Lock()
	if r.closed || !r.started || r.over || r.peers[slot] != p || r.holds >= MaxHolds || r.humansLocked() < 2 {
		r.mu.Unlock()
		return false
	}
	r.peers[slot] = nil
	r.inputs[slot] = sim.PlayerInput{}
	r.holds++
	r.away[slot] = time.AfterFunc(grace, expire)
	others := r.peers
	r.mu.Unlock()
	away := wire.EncodeAway(grace)
	for _, o := range others {
		SendTo(o, away)
	}
	return true
}

// ExpireAway ends a hold that ran out: the seat is left for good (1v1 ends,
// ranked is a forfeit, casual 2v2 keeps the bot). False if the player came back.
func (r *Room) ExpireAway(slot int) bool {
	r.mu.Lock()
	if r.away[slot] == nil {
		r.mu.Unlock()
		return false
	}
	r.away[slot] = nil
	r.mu.Unlock()
	r.Leave(slot, nil, false)
	return true
}

// Reattach seats p in slot again: a held seat, or one whose old connection is
// still open (the drop was not noticed yet: the old one is closed and its
// play() leaves the seat alone). False when the match is over or the token is stale.
func (r *Room) Reattach(slot int, token string, p Peer) bool {
	r.mu.Lock()
	t, old := r.away[slot], r.peers[slot]
	if r.closed || r.tokens[slot] != token || (t == nil && old == nil) || (t != nil && !t.Stop()) {
		r.mu.Unlock()
		return false
	}
	if t == nil && (r.over || r.holds >= MaxHolds) {
		r.mu.Unlock()
		return false
	}
	if t == nil {
		r.holds++ // a replacement counts like a hold
	}
	r.away[slot] = nil
	r.peers[slot] = p
	others, styles := r.peers, wire.EncodeStyles(r.styles)
	r.mu.Unlock()
	SendTo(p, wire.EncodeWelcome(slot))
	SendTo(p, wire.EncodeSeat(token))
	SendTo(p, styles) // its own STYLE after WELCOME is unchanged: no relay would follow
	if old != nil {
		go old.Close(wire.CloseReplace, "replaced") // the close handshake waits on a peer that may be dead
	}
	if t != nil {
		back := wire.EncodeSignal(wire.MsgPeerBack)
		for i, o := range others {
			if i != slot {
				SendTo(o, back)
			}
		}
	}
	return true
}

// pausedLocked: a held seat stops 1v1 and ranked matches (the bot only stands
// in for a casual 2v2 seat).
func (r *Room) pausedLocked() bool {
	return (r.mode != sim.Mode2v2 || r.ranked) && r.anyAwayLocked()
}

// IsPaused reports a match stopped while a dropped player may come back.
func (r *Room) IsPaused() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.pausedLocked()
}

func (r *Room) anyAwayLocked() bool {
	for _, t := range r.away {
		if t != nil {
			return true
		}
	}
	return false
}

// markOver: game over reached; seats are no longer held from now on.
func (r *Room) markOver() {
	r.mu.Lock()
	r.over = true
	r.mu.Unlock()
}

// stopHoldsLocked cancels pending holds of a closing room and returns their
// ranked accounts (the caller frees them through r.release).
func (r *Room) stopHoldsLocked() []ranked.Claim {
	var gone []ranked.Claim
	for i, t := range r.away {
		if t != nil {
			t.Stop()
			r.away[i] = nil
			gone = append(gone, r.claims[i])
		}
	}
	return gone
}
