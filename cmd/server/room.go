package main

import (
	"sync"
	"time"

	"github.com/txny34/futenis/game"
	"github.com/txny34/futenis/sim"
)

const (
	frameInterval   = time.Second / 60
	ticksPerFrame   = sim.TickRate / 60
	roomPointsToWin = 5
	closeNormal     = 1000
	// minHumans2v2: a 2v2 room may start early (bots fill the rest) once this
	// many people are in; see hub.fillWait.
	minHumans2v2 = 2
)

// peer is one connected browser; send must be safe for concurrent use.
type peer interface {
	send(b []byte) error
	close(code int, reason string)
}

// room is one online match: 1v1 (2 seats) or 2v2 (4 seats). Seat = sim slot;
// team = slot % 2. Every client gets its own view (game.ViewOut). In 2v2 an
// empty seat is played by the bot, also when someone leaves mid-match.
type room struct {
	mu      sync.Mutex
	mode    sim.Mode
	seats   int
	peers   [sim.MaxPlayers]peer
	inputs  [sim.MaxPlayers]sim.PlayerInput
	stop    chan struct{}
	started bool
	closed  bool
	startAt time.Time // when the match began; the idle clock starts here
}

func newRoom(mode sim.Mode) *room {
	seats := 2
	if mode == sim.Mode2v2 {
		seats = sim.MaxPlayers
	}
	return &room{mode: mode, seats: seats, stop: make(chan struct{})}
}

// join adds p and returns its slot (-1 if full, closed or already started).
// start is true for exactly one caller: the one whose join fills the room.
// humans is the head count after joining. A waiting joiner gets WAITING while
// the lock is held, so a concurrent WELCOME can never precede it.
func (r *room) join(p peer) (slot int, start bool, humans int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed || r.started {
		return -1, false, 0
	}
	slot = -1
	for i := 0; i < r.seats; i++ {
		if r.peers[i] == nil {
			r.peers[i] = p
			slot = i
			break
		}
	}
	if slot < 0 {
		return -1, false, 0
	}
	humans = r.humansLocked()
	if humans < r.seats {
		_ = p.send(encodeSignal(msgWaiting))
		return slot, false, humans
	}
	return slot, r.startLocked(), humans
}

// startEarly starts a 2v2 room that is not full yet (bots take the empty
// seats). It reports true for exactly one caller.
func (r *room) startEarly() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed || r.humansLocked() < minHumans2v2 {
		return false
	}
	return r.startLocked()
}

func (r *room) startLocked() bool {
	if r.started {
		return false
	}
	r.started = true
	r.startAt = time.Now()
	return true
}

func (r *room) humansLocked() int {
	n := 0
	for _, p := range r.peers {
		if p != nil {
			n++
		}
	}
	return n
}

// humanMaskLocked has bit i set for every seated person.
func (r *room) humanMaskLocked() int {
	m := 0
	for i, p := range r.peers {
		if p != nil {
			m |= 1 << i
		}
	}
	return m
}

// startedAt reports when the match began (ok false while waiting).
func (r *room) startedAt() (t time.Time, ok bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.startAt, r.started
}

// expireIfWaiting closes the room iff the match never started, atomically
// with join (a rival arriving now is rejected instead of meeting a dead host).
func (r *room) expireIfWaiting() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.started || r.closed || (r.mode == sim.Mode2v2 && r.humansLocked() >= minHumans2v2) {
		return false // 2v2 with enough people: the fill timer starts it with bots
	}
	r.closed = true
	close(r.stop)
	return true
}

func (r *room) isClosed() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.closed
}

// setInput stores slot's latest input, turned from the client's view into server space.
func (r *room) setInput(slot int, in sim.PlayerInput) {
	in = game.ViewInput(in, slot)
	r.mu.Lock()
	r.inputs[slot] = in
	r.mu.Unlock()
}
