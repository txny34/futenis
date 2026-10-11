// Package lobby is the shared online lobby: every connected player sees the
// others walk around, with moderation of public names (report.go).
package lobby

import (
	"sync"
	"time"

	"github.com/txny34/futenis/cmd/server/internal/wire"
)

// Shared online lobby: every connected player sees the others walk around.
// Positions are client-reported (the lobby has no rules) and bounded by
// wire.DecodePresence; the server relays a ROSTER of the others 10 times a second.
// Peer is one connected browser; Send must be safe for concurrent use.
type Peer interface {
	Send(b []byte) error
	Close(code int, reason string)
}

const (
	Max          = 48
	MaxShards    = 8 // a full lobby opens another, up to Max*MaxShards people
	MaxPerClient = 2
	MaxMsgs      = 30 // per second: 10 PRESENCE + 1 PING need far less than the match cap
	nameEvery    = 2 * time.Second
	Interval     = time.Second / 10
	Idle         = 30 * time.Second // nothing received (hidden tab, dead Peer): close
)

type lobbyPeer struct {
	p      Peer
	pos    wire.Presence
	name   string
	placed bool // sent at least one PRESENCE
	nameAt time.Time
	buf    []byte

	compact    bool            // wants ROSTER_Q (wire.EncodeRosterQ)
	key        string          // client limit key (report.go)
	reportedBy map[string]bool // networks that reported this name
	flagged    bool            // name hidden after reports: fixed to names.Fallback
}

type Lobby struct {
	mu      sync.Mutex
	peers   map[int]*lobbyPeer
	nextID  int
	dirty   bool         // something changed since the last ROSTER (idle lobby: no traffic)
	entries []wire.Entry // scratch for broadcast (run goroutine only)
	out     []rosterOut  // scratch for broadcast (run goroutine only)
}

type rosterOut struct {
	p   Peer
	msg []byte
}

func New() *Lobby {
	return &Lobby{peers: map[int]*lobbyPeer{}, entries: make([]wire.Entry, 0, Max), out: make([]rosterOut, 0, Max)}
}

// Join adds p and then sends it the names already known (outside the lock:
// a slow joiner never blocks the lobby). ok is false when full.
func (l *Lobby) Join(p Peer) (id int, ok bool) {
	l.mu.Lock()
	if len(l.peers) >= Max {
		l.mu.Unlock()
		return 0, false
	}
	l.nextID++
	id = l.nextID
	names := make([][]byte, 0, len(l.peers))
	for oid, o := range l.peers {
		if o.name != "" {
			names = append(names, wire.EncodeLobbyName(oid, o.name))
		}
	}
	l.peers[id] = &lobbyPeer{p: p, buf: make([]byte, wire.RosterLen(Max))}
	l.dirty = true
	l.mu.Unlock()
	for _, m := range names {
		_ = p.Send(m)
	}
	return id, true
}

func (l *Lobby) Full() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.peers) >= Max
}

func (l *Lobby) SetPos(id int, pos wire.Presence) {
	l.mu.Lock()
	if lp := l.peers[id]; lp != nil && (!lp.placed || lp.pos != pos) {
		lp.pos, lp.placed, l.dirty = pos, true, true
	}
	l.mu.Unlock()
}

// SetName stores id's sanitized name and tells everyone else. At most one
// change per nameEvery reaches the others (spam guard); extra ones are ignored.
func (l *Lobby) SetName(id int, name string, now time.Time) {
	l.mu.Lock()
	lp := l.peers[id]
	if lp == nil || lp.flagged || lp.name == name || now.Sub(lp.nameAt) < nameEvery {
		l.mu.Unlock()
		return
	}
	lp.name, lp.nameAt = name, now
	others := l.othersLocked(id)
	l.mu.Unlock()
	msg := wire.EncodeLobbyName(id, name)
	for _, o := range others {
		_ = o.Send(msg)
	}
}

func (l *Lobby) Leave(id int) {
	l.mu.Lock()
	delete(l.peers, id)
	l.dirty = true
	others := l.othersLocked(id)
	l.mu.Unlock()
	msg := wire.EncodeGone(id)
	for _, o := range others {
		_ = o.Send(msg)
	}
}

func (l *Lobby) othersLocked(id int) []Peer {
	out := make([]Peer, 0, len(l.peers))
	for oid, o := range l.peers {
		if oid != id {
			out = append(out, o.p)
		}
	}
	return out
}

// broadcast sends every Peer a ROSTER of the others that have a position.
// Messages are built under the lock and sent after it: a slow Peer never
// blocks joins or position updates. Each Peer's buffer is only reused on the
// next tick, by this same goroutine.
func (l *Lobby) broadcast() {
	l.mu.Lock()
	if !l.dirty {
		l.mu.Unlock()
		return
	}
	l.dirty = false
	l.out = l.out[:0]
	for id, lp := range l.peers {
		l.entries = l.entries[:0]
		for oid, o := range l.peers {
			if oid != id && o.placed {
				l.entries = append(l.entries, wire.Entry{ID: oid, P: o.pos})
			}
		}
		var n int
		if lp.compact {
			n = wire.EncodeRosterQ(lp.buf, l.entries)
		} else {
			n = wire.EncodeRoster(lp.buf, l.entries)
		}
		l.out = append(l.out, rosterOut{p: lp.p, msg: lp.buf[:n]})
	}
	l.mu.Unlock()
	for _, o := range l.out {
		_ = o.p.Send(o.msg) // bounded by the write timeout; a dead Peer leaves via its read loop
	}
}

// Run broadcasts on every tick until stop closes.
func (l *Lobby) Run(tick <-chan time.Time, stop <-chan struct{}) {
	for {
		select {
		case <-stop:
			return
		case <-tick:
			l.broadcast()
		}
	}
}
