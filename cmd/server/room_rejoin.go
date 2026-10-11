package main

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/txny34/futenis/sim"
)

// Reconnection: a started match keeps a dropped player's seat for rejoinGrace.
// 1v1 and ranked matches pause meanwhile (a bot never plays someone's seat
// there); casual 2v2 goes on with the bot in that seat. The player comes back
// with the seat's secret token (SEAT message, sent after WELCOME), also when
// the server has not noticed the drop yet (the new connection replaces the old).
const (
	rejoinGrace  = 20 * time.Second
	maxHolds     = 4 // per room: pauses can't be chained to stall a match
	tokenBytes   = 16
	tokenLen     = 2 * tokenBytes // hex
	closeReplace = 4008           // old connection of a player who rejoined
)

func newToken() string {
	b := make([]byte, tokenBytes)
	_, _ = rand.Read(b) // crypto/rand.Read never fails (Go ≥ 1.24)
	return hex.EncodeToString(b)
}

// issueTokens gives every seated person a fresh rejoin token.
func (r *room) issueTokens() [sim.MaxPlayers]string {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, p := range r.peers {
		if p != nil {
			r.tokens[i] = newToken()
		}
	}
	return r.tokens
}

func (r *room) tokenList() [sim.MaxPlayers]string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.tokens
}

// seatClaim is slot's ranked reservation (zero outside ranked).
func (r *room) seatClaim(slot int) claim {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.claims[slot]
}

// hold keeps slot's seat after p's connection dropped. It refuses (the caller
// then leaves for good) when p no longer owns the seat, the match is not
// running or is over, nobody else is connected, or the room used up its holds.
// expire runs after grace unless the player is back by then.
func (r *room) hold(slot int, p peer, grace time.Duration, expire func()) bool {
	r.mu.Lock()
	if r.closed || !r.started || r.over || r.peers[slot] != p || r.holds >= maxHolds || r.humansLocked() < 2 {
		r.mu.Unlock()
		return false
	}
	r.peers[slot] = nil
	r.inputs[slot] = sim.PlayerInput{}
	r.holds++
	r.away[slot] = time.AfterFunc(grace, expire)
	others := r.peers
	r.mu.Unlock()
	away := encodeAway(grace)
	for _, o := range others {
		sendTo(o, away)
	}
	return true
}

// expireAway ends a hold that ran out: the seat is left for good (1v1 ends,
// ranked is a forfeit, casual 2v2 keeps the bot). False if the player came back.
func (r *room) expireAway(slot int) bool {
	r.mu.Lock()
	if r.away[slot] == nil {
		r.mu.Unlock()
		return false
	}
	r.away[slot] = nil
	r.mu.Unlock()
	r.leave(slot, nil, false)
	return true
}

// reattach seats p in slot again: a held seat, or one whose old connection is
// still open (the drop was not noticed yet: the old one is closed and its
// play() leaves the seat alone). False when the match is over or the token is stale.
func (r *room) reattach(slot int, token string, p peer) bool {
	r.mu.Lock()
	t, old := r.away[slot], r.peers[slot]
	if r.closed || r.tokens[slot] != token || (t == nil && old == nil) || (t != nil && !t.Stop()) {
		r.mu.Unlock()
		return false
	}
	if t == nil && (r.over || r.holds >= maxHolds) {
		r.mu.Unlock()
		return false
	}
	if t == nil {
		r.holds++ // a replacement counts like a hold
	}
	r.away[slot] = nil
	r.peers[slot] = p
	others := r.peers
	r.mu.Unlock()
	sendTo(p, encodeWelcome(slot))
	sendTo(p, encodeSeat(token))
	if old != nil {
		go old.close(closeReplace, "replaced") // the close handshake waits on a peer that may be dead
	}
	if t != nil {
		back := encodeSignal(msgPeerBack)
		for i, o := range others {
			if i != slot {
				sendTo(o, back)
			}
		}
	}
	return true
}

// pausedLocked: a held seat stops 1v1 and ranked matches (the bot only stands
// in for a casual 2v2 seat).
func (r *room) pausedLocked() bool {
	return (r.mode != sim.Mode2v2 || r.ranked) && r.anyAwayLocked()
}

func (r *room) isPaused() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.pausedLocked()
}

func (r *room) anyAwayLocked() bool {
	for _, t := range r.away {
		if t != nil {
			return true
		}
	}
	return false
}

// markOver: game over reached; seats are no longer held from now on.
func (r *room) markOver() {
	r.mu.Lock()
	r.over = true
	r.mu.Unlock()
}

// stopHoldsLocked cancels pending holds of a closing room and returns their
// ranked accounts (the caller frees them through r.release).
func (r *room) stopHoldsLocked() []claim {
	var gone []claim
	for i, t := range r.away {
		if t != nil {
			t.Stop()
			r.away[i] = nil
			gone = append(gone, r.claims[i])
		}
	}
	return gone
}
