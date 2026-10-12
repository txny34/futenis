package lobby

import (
	"time"

	"github.com/txny34/futenis/cmd/server/internal/wire"
)

// Looks and emotes in the shared lobby: each member's latest look is relayed
// to the others (and sent to newcomers on Join); an emote is relayed once.
// Both are throttled per member: each relay makes clients rebuild meshes or
// play an animation.

const (
	lookEvery  = 300 * time.Millisecond
	emoteEvery = time.Second
)

// SetLook stores id's look and tells everyone else (ignored when unchanged or
// within lookEvery of the last change).
func (l *Lobby) SetLook(id int, st wire.Style, now time.Time) {
	l.mu.Lock()
	lp := l.peers[id]
	if lp == nil || lp.look == st || now.Sub(lp.lookAt) < lookEvery {
		l.mu.Unlock()
		return
	}
	lp.look, lp.lookAt = st, now
	others := l.othersLocked(id)
	l.mu.Unlock()
	msg := wire.EncodeLobbyLook(id, st)
	for _, o := range others {
		_ = o.Send(msg)
	}
}

// Emote relays id's emote to everyone else (at most one per emoteEvery).
func (l *Lobby) Emote(id int, kind byte, now time.Time) {
	l.mu.Lock()
	lp := l.peers[id]
	if lp == nil || now.Sub(lp.emoteAt) < emoteEvery {
		l.mu.Unlock()
		return
	}
	lp.emoteAt = now
	others := l.othersLocked(id)
	l.mu.Unlock()
	msg := wire.EncodeLobbyEmote(id, kind)
	for _, o := range others {
		_ = o.Send(msg)
	}
}

// looksLocked: every known look but id's own, for a newcomer.
func (l *Lobby) looksLocked(id int) [][]byte {
	out := make([][]byte, 0, len(l.peers))
	for oid, o := range l.peers {
		if oid != id && o.look[0] != 0 {
			out = append(out, wire.EncodeLobbyLook(oid, o.look))
		}
	}
	return out
}
