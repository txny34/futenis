package match

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/txny34/futenis/ai"
	"github.com/txny34/futenis/cmd/server/internal/wire"
	"github.com/txny34/futenis/game"
	"github.com/txny34/futenis/sim"
)

type tapePeer struct {
	mu   sync.Mutex
	msgs [][]byte
	got  chan struct{}
}

func (p *tapePeer) Send(b []byte) error {
	p.mu.Lock()
	p.msgs = append(p.msgs, append([]byte(nil), b...))
	p.mu.Unlock()
	if b[0] == wire.MsgTape && int(binary.LittleEndian.Uint16(b[2:]))+1 == int(binary.LittleEndian.Uint16(b[4:])) {
		close(p.got)
	}
	return nil
}
func (p *tapePeer) Close(int, string) {}

func TestTapeReachesEveryPeer(t *testing.T) {
	s := game.NewSession()
	if err := s.Configure(int(sim.Mode1v1), int(ai.Normal), roomPointsToWin, 0b11); err != nil {
		t.Fatal(err)
	}
	tw := newRoomTape(s, sim.Mode1v1, 0b11)
	var in [sim.MaxPlayers]sim.PlayerInput
	for f := 0; f < 50; f++ {
		in[0].MoveX = float64(f % 3)
		tw.Frame(&in, ticksPerFrame, 0b11)
	}
	a, b := &tapePeer{got: make(chan struct{})}, &tapePeer{got: make(chan struct{})}
	sendTape([sim.MaxPlayers]Peer{a, b}, tapeMessages(tw.Bytes()))
	for _, p := range []*tapePeer{a, b} {
		select {
		case <-p.got:
		case <-time.After(2 * time.Second):
			t.Fatal("tape not delivered")
		}
		var gz []byte
		for _, m := range p.msgs {
			gz = append(gz, m[wire.HeaderLen:]...)
		}
		zr, err := gzip.NewReader(bytes.NewReader(gz))
		if err != nil {
			t.Fatal(err)
		}
		raw, err := io.ReadAll(zr)
		if err != nil {
			t.Fatal(err)
		}
		setup, _, steps, err := game.DecodeTape(raw)
		if err != nil {
			t.Fatal(err)
		}
		if setup.Mode != int(sim.Mode1v1) || setup.PointsToWin != roomPointsToWin || setup.Mask != 0b11 || len(steps) == 0 {
			t.Fatalf("setup %+v steps %d", setup, len(steps))
		}
	}
}

func TestNoTapeNoMessages(t *testing.T) {
	if tapeMessages(nil) != nil {
		t.Fatal("nil tape produced messages")
	}
}
