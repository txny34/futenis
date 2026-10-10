package main

import (
	"sync"
	"time"

	"github.com/txny34/futenis/ai"
	"github.com/txny34/futenis/game"
	"github.com/txny34/futenis/sim"
)

const (
	roomSlots        = 2
	frameInterval    = time.Second / 60
	ticksPerFrame    = sim.TickRate / 60
	roomPointsToWin  = 5
	humanMaskOneVOne = 0b11
	closeNormal      = 1000
)

// peer is one connected browser; send must be safe for concurrent use.
type peer interface {
	send(b []byte) error
	close(code int, reason string)
}

// room is a private 1v1 match: slot 0 = host, slot 1 = guest (rotated view).
type room struct {
	mu      sync.Mutex
	peers   [roomSlots]peer
	inputs  [sim.MaxPlayers]sim.PlayerInput
	stop    chan struct{}
	started bool
	closed  bool
	startAt time.Time // when the second peer joined; the idle clock starts here
}

func newRoom() *room { return &room{stop: make(chan struct{})} }

// join adds p and returns its slot (-1 if full or closed). start is true for
// exactly one caller: the one whose join fills the room. A lone joiner gets
// WAITING while the lock is held, so a concurrent WELCOME can never precede
// it (the send is bounded by the peer's write timeout).
func (r *room) join(p peer) (slot int, start bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return -1, false
	}
	slot = -1
	for i := range r.peers {
		if r.peers[i] == nil {
			r.peers[i] = p
			slot = i
			break
		}
	}
	if slot < 0 {
		return -1, false
	}
	if r.peers[1-slot] == nil {
		_ = p.send(encodeSignal(msgWaiting))
		return slot, false
	}
	if !r.started {
		r.started = true
		r.startAt = time.Now()
		start = true
	}
	return slot, start
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
	if r.started || r.closed {
		return false
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

// setInput stores slot's latest input (client view; the guest's is rotated).
func (r *room) setInput(slot int, in sim.PlayerInput) {
	if slot == 1 {
		in = game.RotateInput(in)
	}
	r.mu.Lock()
	r.inputs[slot] = in
	r.mu.Unlock()
}

// leave removes slot, tells the other peer and stops the room.
func (r *room) leave(slot int) {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return
	}
	r.closed = true
	other := r.peers[1-slot]
	r.peers[slot] = nil
	close(r.stop)
	r.mu.Unlock()
	if other != nil {
		_ = other.send(encodeSignal(msgPeerLeft))
		other.close(closeNormal, "peer left")
	}
}

// run plays the match until the room stops. tick is injectable for tests.
func (r *room) run(tick <-chan time.Time) {
	s := game.NewSession()
	if err := s.Configure(int(sim.Mode1v1), int(ai.Normal), roomPointsToWin, humanMaskOneVOne); err != nil {
		panic(err) // constant arguments: a failure is a programming error
	}
	out := make([]float64, game.OutLen)
	rot := make([]float64, game.OutLen)
	host := make([]byte, frameMsgLen)
	guest := make([]byte, frameMsgLen)
	for {
		select {
		case <-r.stop:
			return
		case <-tick:
		}
		r.mu.Lock()
		in := r.inputs
		p0, p1 := r.peers[0], r.peers[1]
		r.mu.Unlock()
		s.Run(in, ticksPerFrame, out)
		game.RotateOut1v1(rot, out)
		encodeFrame(host, out)
		encodeFrame(guest, rot)
		sendTo(p0, host)
		sendTo(p1, guest)
	}
}

func sendTo(p peer, b []byte) {
	if p != nil {
		_ = p.send(b) // a dead peer is detected by its read loop, which calls leave
	}
}
