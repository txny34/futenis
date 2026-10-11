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
// with the seat's secret token (SEAT message, sent after WELCOME).
const (
	rejoinGrace = 20 * time.Second
	maxRejoins  = 3
	tokenBytes  = 16
	tokenLen    = 2 * tokenBytes // hex
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

// hold keeps slot's seat after its connection dropped. It refuses (the caller
// then leaves for good) when the match is not running, nobody else is
// connected, or the seat used up its holds. expire runs after grace unless the
// player is back by then.
func (r *room) hold(slot int, grace time.Duration, expire func()) bool {
	r.mu.Lock()
	if r.closed || !r.started || r.peers[slot] == nil || r.rejoins[slot] >= maxRejoins || r.humansLocked() < 2 {
		r.mu.Unlock()
		return false
	}
	r.peers[slot] = nil
	r.inputs[slot] = sim.PlayerInput{}
	r.rejoins[slot]++
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
	r.leave(slot)
	return true
}

// reattach seats p again in its held slot. False when the hold expired, the
// room closed or the token is stale.
func (r *room) reattach(slot int, token string, p peer) bool {
	r.mu.Lock()
	t := r.away[slot]
	if r.closed || t == nil || r.tokens[slot] != token || !t.Stop() {
		r.mu.Unlock()
		return false
	}
	r.away[slot] = nil
	r.peers[slot] = p
	others := r.peers
	r.mu.Unlock()
	sendTo(p, encodeWelcome(slot))
	sendTo(p, encodeSeat(token))
	back := encodeSignal(msgPeerBack)
	for i, o := range others {
		if i != slot {
			sendTo(o, back)
		}
	}
	return true
}

// pausedLocked: a held seat stops 1v1 and ranked matches (the bot only stands
// in for a casual 2v2 seat).
func (r *room) pausedLocked() bool {
	if r.mode == sim.Mode2v2 && !r.ranked {
		return false
	}
	for _, t := range r.away {
		if t != nil {
			return true
		}
	}
	return false
}

// stopHoldsLocked cancels pending holds of a closing room.
func (r *room) stopHoldsLocked() {
	for i, t := range r.away {
		if t != nil {
			t.Stop()
			r.away[i] = nil
		}
	}
}
