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
}

func newRoom() *room { return &room{stop: make(chan struct{})} }

// join adds p and returns its slot, or -1 if the room is full or closed.
func (r *room) join(p peer) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return -1
	}
	for i := range r.peers {
		if r.peers[i] == nil {
			r.peers[i] = p
			return i
		}
	}
	return -1
}

// ready reports whether both slots are taken and the loop has not started;
// it marks the room started so only one caller launches run.
func (r *room) ready() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.started || r.peers[0] == nil || r.peers[1] == nil {
		return false
	}
	r.started = true
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
