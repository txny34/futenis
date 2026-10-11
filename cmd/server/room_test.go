package main

import (
	"encoding/binary"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/txny34/futenis/cmd/server/internal/peertest"
	"github.com/txny34/futenis/cmd/server/internal/wire"
	"github.com/txny34/futenis/game"
	"github.com/txny34/futenis/sim"
)

const waitTimeout = time.Second

// waitFor polls until p has at least n messages.
func waitFor(t *testing.T, p *peertest.Peer, n int) [][]byte {
	t.Helper()
	deadline := time.Now().Add(waitTimeout)
	for time.Now().Before(deadline) {
		if m, _ := p.Snapshot(); len(m) >= n {
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
	r := NewRoom(sim.Mode1v1, "test")
	if slot, start, _ := r.Join(&peertest.Peer{}); slot != 0 || start {
		t.Fatal("first join")
	}
	if slot, start, _ := r.Join(&peertest.Peer{}); slot != 1 || !start {
		t.Fatal("second join should start")
	}
	if slot, start, _ := r.Join(&peertest.Peer{}); slot != -1 || start {
		t.Fatal("third peer accepted")
	}
}

// TestRoomWaitingOrder checks only a lone joiner gets WAITING, and always
// before anything else, so WELCOME is never followed by WAITING.
func TestRoomWaitingOrder(t *testing.T) {
	for range 50 {
		r := NewRoom(sim.Mode1v1, "test")
		host, guest := &peertest.Peer{}, &peertest.Peer{}
		var wg sync.WaitGroup
		starts := make(chan bool, 2)
		for _, p := range []*peertest.Peer{host, guest} {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, start, _ := r.Join(p)
				if start {
					SendTo(r.peers[0], wire.EncodeWelcome(0))
					SendTo(r.peers[1], wire.EncodeWelcome(1))
				}
				starts <- start
			}()
		}
		wg.Wait()
		if n := btoi(<-starts) + btoi(<-starts); n != 1 {
			t.Fatalf("%d starters", n)
		}
		waits := 0
		for _, p := range []*peertest.Peer{host, guest} {
			m, _ := p.Snapshot()
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
	r := NewRoom(sim.Mode1v1, "test")
	host, guest := &peertest.Peer{}, &peertest.Peer{}
	r.Join(host)
	r.Join(guest)
	tick := make(chan time.Time)
	done := make(chan struct{})
	go func() { r.Run(tick); close(done) }()
	for range 3 {
		tick <- time.Time{}
	}
	hm := waitFor(t, host, 4)[1:] // host got WAITING first
	gm := waitFor(t, guest, 3)
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
	r.Leave(0, nil, false)
	<-done
}

func TestRoomGuestInputRotated(t *testing.T) {
	r := NewRoom(sim.Mode1v1, "test")
	in := sim.PlayerInput{MoveX: 1, MoveZ: -0.5, TargetX: 4, TargetZ: 2, Buttons: 3, Spin: 1}
	r.SetInput(0, nil, in)
	r.SetInput(1, nil, in)
	if r.inputs[0] != in || r.inputs[1] != game.RotateInput(in) {
		t.Fatalf("inputs %+v", r.inputs)
	}
}

func TestRoomLeave(t *testing.T) {
	r := NewRoom(sim.Mode1v1, "test")
	host, guest := &peertest.Peer{}, &peertest.Peer{}
	r.Join(host)
	r.Join(guest)
	r.Leave(1, nil, false)
	r.Leave(0, nil, false) // second leave is a no-op
	msgs, closed := host.Snapshot()
	if len(msgs) != 2 || msgs[0][0] != wire.MsgWaiting || msgs[1][0] != wire.MsgPeerLeft || !closed {
		t.Fatalf("host msgs %v closed %v", msgs, closed)
	}
	if slot, _, _ := r.Join(&peertest.Peer{}); !r.IsClosed() || slot != -1 {
		t.Fatal("closed room accepted a peer")
	}
	if m, _ := guest.Snapshot(); len(m) != 0 {
		t.Fatalf("leaver got %v", m)
	}
}
