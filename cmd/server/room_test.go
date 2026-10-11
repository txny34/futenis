package main

import (
	"encoding/binary"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/txny34/futenis/cmd/server/internal/wire"
	"github.com/txny34/futenis/game"
	"github.com/txny34/futenis/sim"
)

const waitTimeout = time.Second

type fakePeer struct {
	mu     sync.Mutex
	msgs   [][]byte
	closed bool
}

func (f *fakePeer) Send(b []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.msgs = append(f.msgs, append([]byte{}, b...))
	return nil
}

func (f *fakePeer) Close(int, string) {
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
	out := make([]float64, (len(b)-wire.HeaderLen)/8)
	for i := range out {
		out[i] = math.Float64frombits(binary.LittleEndian.Uint64(b[wire.HeaderLen+i*8:]))
	}
	return out
}

func TestRoomJoinFull(t *testing.T) {
	r := newRoom(sim.Mode1v1, "test")
	if slot, start, _ := r.join(&fakePeer{}); slot != 0 || start {
		t.Fatal("first join")
	}
	if slot, start, _ := r.join(&fakePeer{}); slot != 1 || !start {
		t.Fatal("second join should start")
	}
	if slot, start, _ := r.join(&fakePeer{}); slot != -1 || start {
		t.Fatal("third peer accepted")
	}
}

// TestRoomWaitingOrder checks only a lone joiner gets WAITING, and always
// before anything else, so WELCOME is never followed by WAITING.
func TestRoomWaitingOrder(t *testing.T) {
	for range 50 {
		r := newRoom(sim.Mode1v1, "test")
		host, guest := &fakePeer{}, &fakePeer{}
		var wg sync.WaitGroup
		starts := make(chan bool, 2)
		for _, p := range []*fakePeer{host, guest} {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, start, _ := r.join(p)
				if start {
					sendTo(r.peers[0], wire.EncodeWelcome(0))
					sendTo(r.peers[1], wire.EncodeWelcome(1))
				}
				starts <- start
			}()
		}
		wg.Wait()
		if n := btoi(<-starts) + btoi(<-starts); n != 1 {
			t.Fatalf("%d starters", n)
		}
		waits := 0
		for _, p := range []*fakePeer{host, guest} {
			m, _ := p.snapshot()
			for i, b := range m {
				if b[0] == wire.MsgWaiting {
					waits++
					if i != 0 {
						t.Fatalf("WAITING at position %d", i)
					}
				}
			}
		}
		if waits != 1 {
			t.Fatalf("%d WAITING messages, want 1", waits)
		}
	}
}

func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}

func TestRoomFramesRotated(t *testing.T) {
	r := newRoom(sim.Mode1v1, "test")
	host, guest := &fakePeer{}, &fakePeer{}
	r.join(host)
	r.join(guest)
	tick := make(chan time.Time)
	done := make(chan struct{})
	go func() { r.run(tick); close(done) }()
	for range 3 {
		tick <- time.Time{}
	}
	hm := host.waitFor(t, 4)[1:] // host got WAITING first
	gm := guest.waitFor(t, 3)
	rot := make([]float64, game.OutLen)
	for i := range 3 {
		if len(hm[i]) != wire.FrameLen || hm[i][0] != wire.MsgFrame || gm[i][0] != wire.MsgFrame {
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
	r.leave(0, nil, false)
	<-done
}

func TestRoomGuestInputRotated(t *testing.T) {
	r := newRoom(sim.Mode1v1, "test")
	in := sim.PlayerInput{MoveX: 1, MoveZ: -0.5, TargetX: 4, TargetZ: 2, Buttons: 3, Spin: 1}
	r.setInput(0, nil, in)
	r.setInput(1, nil, in)
	if r.inputs[0] != in || r.inputs[1] != game.RotateInput(in) {
		t.Fatalf("inputs %+v", r.inputs)
	}
}

func TestRoomLeave(t *testing.T) {
	r := newRoom(sim.Mode1v1, "test")
	host, guest := &fakePeer{}, &fakePeer{}
	r.join(host)
	r.join(guest)
	r.leave(1, nil, false)
	r.leave(0, nil, false) // second leave is a no-op
	msgs, closed := host.snapshot()
	if len(msgs) != 2 || msgs[0][0] != wire.MsgWaiting || msgs[1][0] != wire.MsgPeerLeft || !closed {
		t.Fatalf("host msgs %v closed %v", msgs, closed)
	}
	if slot, _, _ := r.join(&fakePeer{}); !r.isClosed() || slot != -1 {
		t.Fatal("closed room accepted a peer")
	}
	if m, _ := guest.snapshot(); len(m) != 0 {
		t.Fatalf("leaver got %v", m)
	}
}
