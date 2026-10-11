package main

import (
	"sync"
	"time"
)

// Shared online lobby: every connected player sees the others walk around.
// Positions are client-reported (the lobby has no rules) and bounded by
// decodePresence; the server relays a ROSTER of the others 10 times a second.
const (
	maxLobby      = 48
	maxLobbyShard = 8 // a full lobby opens another, up to maxLobby*maxLobbyShard people
	maxLobbyPerIP = 2
	maxLobbyMsgs  = 30 // per second: 10 PRESENCE + 1 PING need far less than the match cap
	nameEvery     = 2 * time.Second
	lobbyInterval = time.Second / 10
	lobbyIdle     = 30 * time.Second // nothing received (hidden tab, dead peer): close
)

type lobbyPeer struct {
	p      peer
	pos    presence
	name   string
	placed bool // sent at least one PRESENCE
	nameAt time.Time
	buf    []byte

	compact    bool            // wants ROSTER_Q (lobby_compact.go)
	key        string          // client limit key (lobby_report.go)
	reportedBy map[string]bool // networks that reported this name
	flagged    bool            // name hidden after reports: fixed to fallbackName
}

type lobby struct {
	mu      sync.Mutex
	peers   map[int]*lobbyPeer
	nextID  int
	dirty   bool         // something changed since the last ROSTER (idle lobby: no traffic)
	entries []lobbyEntry // scratch for broadcast (run goroutine only)
	out     []rosterOut  // scratch for broadcast (run goroutine only)
}

type rosterOut struct {
	p   peer
	msg []byte
}

func newLobby() *lobby {
	return &lobby{peers: map[int]*lobbyPeer{}, entries: make([]lobbyEntry, 0, maxLobby), out: make([]rosterOut, 0, maxLobby)}
}

// join adds p and then sends it the names already known (outside the lock:
// a slow joiner never blocks the lobby). ok is false when full.
func (l *lobby) join(p peer) (id int, ok bool) {
	l.mu.Lock()
	if len(l.peers) >= maxLobby {
		l.mu.Unlock()
		return 0, false
	}
	l.nextID++
	id = l.nextID
	names := make([][]byte, 0, len(l.peers))
	for oid, o := range l.peers {
		if o.name != "" {
			names = append(names, encodeLobbyName(oid, o.name))
		}
	}
	l.peers[id] = &lobbyPeer{p: p, buf: make([]byte, rosterMsgLen(maxLobby))}
	l.dirty = true
	l.mu.Unlock()
	for _, m := range names {
		_ = p.send(m)
	}
	return id, true
}

func (l *lobby) full() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.peers) >= maxLobby
}

func (l *lobby) setPos(id int, pos presence) {
	l.mu.Lock()
	if lp := l.peers[id]; lp != nil && (!lp.placed || lp.pos != pos) {
		lp.pos, lp.placed, l.dirty = pos, true, true
	}
	l.mu.Unlock()
}

// setName stores id's sanitized name and tells everyone else. At most one
// change per nameEvery reaches the others (spam guard); extra ones are ignored.
func (l *lobby) setName(id int, name string, now time.Time) {
	l.mu.Lock()
	lp := l.peers[id]
	if lp == nil || lp.flagged || lp.name == name || now.Sub(lp.nameAt) < nameEvery {
		l.mu.Unlock()
		return
	}
	lp.name, lp.nameAt = name, now
	others := l.othersLocked(id)
	l.mu.Unlock()
	msg := encodeLobbyName(id, name)
	for _, o := range others {
		_ = o.send(msg)
	}
}

func (l *lobby) leave(id int) {
	l.mu.Lock()
	delete(l.peers, id)
	l.dirty = true
	others := l.othersLocked(id)
	l.mu.Unlock()
	msg := encodeGone(id)
	for _, o := range others {
		_ = o.send(msg)
	}
}

func (l *lobby) othersLocked(id int) []peer {
	out := make([]peer, 0, len(l.peers))
	for oid, o := range l.peers {
		if oid != id {
			out = append(out, o.p)
		}
	}
	return out
}

// broadcast sends every peer a ROSTER of the others that have a position.
// Messages are built under the lock and sent after it: a slow peer never
// blocks joins or position updates. Each peer's buffer is only reused on the
// next tick, by this same goroutine.
func (l *lobby) broadcast() {
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
				l.entries = append(l.entries, lobbyEntry{id: oid, p: o.pos})
			}
		}
		var n int
		if lp.compact {
			n = encodeRosterQ(lp.buf, l.entries)
		} else {
			n = encodeRoster(lp.buf, l.entries)
		}
		l.out = append(l.out, rosterOut{p: lp.p, msg: lp.buf[:n]})
	}
	l.mu.Unlock()
	for _, o := range l.out {
		_ = o.p.send(o.msg) // bounded by the write timeout; a dead peer leaves via its read loop
	}
}

// run broadcasts on every tick until stop closes.
func (l *lobby) run(tick <-chan time.Time, stop <-chan struct{}) {
	for {
		select {
		case <-stop:
			return
		case <-tick:
			l.broadcast()
		}
	}
}
