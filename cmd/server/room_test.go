package main

import (
	"encoding/binary"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/txny34/futenis/game"
	"github.com/txny34/futenis/sim"
)

const waitTimeout = time.Second

type fakePeer struct {
	mu     sync.Mutex
	msgs   [][]byte
	closed bool
}

func (f *fakePeer) send(b []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.msgs = append(f.msgs, append([]byte{}, b...))
	return nil
}

func (f *fakePeer) close(int, string) {
	f.mu.Lock()
	f.closed = true
	f.mu.Unlock()
}

func (f *fakePeer) snapshot() ([][]byte, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([][]byte{}, f.msgs...), f.closed
}

// waitFor polls until the peer has at least n messages.
func (f *fakePeer) waitFor(t *testing.T, n int) [][]byte {
	t.Helper()
	deadline := time.Now().Add(waitTimeout)
	for time.Now().Before(deadline) {
		if m, _ := f.snapshot(); len(m) >= n {
			return m
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("timed out waiting for %d messages", n)
	return nil
}

func decodeBody(b []byte) []float64 {
	out := make([]float64, (len(b)-headerLen)/8)
	for i := range out {
		out[i] = math.Float64frombits(binary.LittleEndian.Uint64(b[headerLen+i*8:]))
	}
	return out
}

func TestRoomJoinFull(t *testing.T) {
	r := newRoom()
	if r.join(&fakePeer{}) != 0 || r.ready() {
		t.Fatal("first join")
	}
	if r.join(&fakePeer{}) != 1 || !r.ready() || r.ready() {
		t.Fatal("second join / ready once")
	}
	if r.join(&fakePeer{}) != -1 {
		t.Fatal("third peer accepted")
	}
}

func TestRoomFramesRotated(t *testing.T) {
	r := newRoom()
	host, guest := &fakePeer{}, &fakePeer{}
	r.join(host)
	r.join(guest)
	tick := make(chan time.Time)
	done := make(chan struct{})
	go func() { r.run(tick); close(done) }()
	for range 3 {
		tick <- time.Time{}
	}
	hm := host.waitFor(t, 3)
	gm := guest.waitFor(t, 3)
	rot := make([]float64, game.OutLen)
	for i := range 3 {
		if len(hm[i]) != frameMsgLen || hm[i][0] != msgFrame || gm[i][0] != msgFrame {
			t.Fatalf("frame %d bad header", i)
		}
		game.RotateOut1v1(rot, decodeBody(hm[i]))
		g := decodeBody(gm[i])
		for j := range rot {
			if rot[j] != g[j] {
				t.Fatalf("frame %d float %d: guest %v want %v", i, j, g[j], rot[j])
			}
		}
	}
	r.leave(0)
	<-done
}

func TestRoomGuestInputRotated(t *testing.T) {
	r := newRoom()
	in := sim.PlayerInput{MoveX: 1, MoveZ: -0.5, TargetX: 4, TargetZ: 2, Buttons: 3, Spin: 1}
	r.setInput(0, in)
	r.setInput(1, in)
	if r.inputs[0] != in || r.inputs[1] != game.RotateInput(in) {
		t.Fatalf("inputs %+v", r.inputs)
	}
}

func TestRoomLeave(t *testing.T) {
	r := newRoom()
	host, guest := &fakePeer{}, &fakePeer{}
	r.join(host)
	r.join(guest)
	r.leave(1)
	r.leave(0) // second leave is a no-op
	msgs, closed := host.snapshot()
	if len(msgs) != 1 || msgs[0][0] != msgPeerLeft || !closed {
		t.Fatalf("host msgs %v closed %v", msgs, closed)
	}
	if !r.isClosed() || r.join(&fakePeer{}) != -1 {
		t.Fatal("closed room accepted a peer")
	}
	if m, _ := guest.snapshot(); len(m) != 0 {
		t.Fatalf("leaver got %v", m)
	}
}
