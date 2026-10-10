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
	lobbyInterval = time.Second / 10
	lobbyIdle     = 30 * time.Second // nothing received (hidden tab, dead peer): close
)

type lobbyPeer struct {
	p      peer
	pos    presence
	name   string
	placed bool // sent at least one PRESENCE
	buf    []byte
}

type lobby struct {
	mu      sync.Mutex
	peers   map[int]*lobbyPeer
	nextID  int
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

// join adds p and sends it the names already known. ok is false when full.
func (l *lobby) join(p peer) (id int, ok bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.peers) >= maxLobby {
		return 0, false
	}
	l.nextID++
	id = l.nextID
	for oid, o := range l.peers {
		if o.name != "" {
			_ = p.send(encodeLobbyName(oid, o.name))
		}
	}
	l.peers[id] = &lobbyPeer{p: p, buf: make([]byte, rosterMsgLen(maxLobby))}
	return id, true
}

func (l *lobby) setPos(id int, pos presence) {
	l.mu.Lock()
	if lp := l.peers[id]; lp != nil {
		lp.pos, lp.placed = pos, true
	}
	l.mu.Unlock()
}

// setName stores id's sanitized name and tells everyone else.
func (l *lobby) setName(id int, name string) {
	l.mu.Lock()
	lp := l.peers[id]
	if lp == nil || lp.name == name {
		l.mu.Unlock()
		return
	}
	lp.name = name
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
	l.out = l.out[:0]
	for id, lp := range l.peers {
		l.entries = l.entries[:0]
		for oid, o := range l.peers {
			if oid != id && o.placed {
				l.entries = append(l.entries, lobbyEntry{id: oid, p: o.pos})
			}
		}
		n := encodeRoster(lp.buf, l.entries)
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
