package match

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"io"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/txny34/futenis/cmd/server/internal/wire"
	"github.com/txny34/futenis/game"
	"github.com/txny34/futenis/sim"
)

// overPeer keeps the first GameOver frame it was sent and the TAPE chunks.
type overPeer struct {
	mu    sync.Mutex
	over  []float64
	tape  [][]byte
	total int
}

func (p *overPeer) Send(b []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	switch b[0] {
	case wire.MsgFrame:
		if p.over == nil {
			if f := decodeBody(b); int(f[sim.SnapLen+sim.SnapPhase]) == int(sim.PhaseGameOver) {
				p.over = f
			}
		}
	case wire.MsgTape:
		p.tape = append(p.tape, append([]byte(nil), b...))
		p.total = int(binary.LittleEndian.Uint16(b[4:]))
	}
	return nil
}

func (p *overPeer) Close(int, string) {}

func (p *overPeer) ready() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.over != nil && p.total > 0 && len(p.tape) == p.total
}

// replaySeat re-simulates the peer's tape, viewed from seat, and returns the final frame.
func replaySeat(t *testing.T, p *overPeer, seat int) []float64 {
	t.Helper()
	var gz []byte
	for _, m := range p.tape {
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
	s, cfg, steps, err := game.DecodeTape(raw)
	if err != nil {
		t.Fatal(err)
	}
	g := game.NewSession()
	if err := g.ConfigureFirst(s.Mode, s.Difficulty, s.PointsToWin, s.Mask, s.FirstServer); err != nil {
		t.Fatal(err)
	}
	if err := g.ReadConfig(cfg, make([]float64, sim.CfgLen)); err != nil {
		t.Fatal(err)
	}
	g.Reset()
	if err := g.SetView(seat, s.Mode == int(sim.Mode2v2)); err != nil {
		t.Fatal(err)
	}
	out := make([]float64, game.OutLen)
	mask := s.Mask
	for _, st := range steps {
		if st.Mask != mask {
			mask = st.Mask
			if err := g.SetHumanMask(mask); err != nil {
				t.Fatal(err)
			}
		}
		for left := st.Ticks; left > 0; left -= game.MaxSteps {
			g.Run(st.In, min(left, game.MaxSteps), out)
		}
	}
	return out
}

// A whole online match, end to end: the tape the server sends replays, from
// each seat, to exactly the GameOver frame that seat was sent.
func TestRoomTapeReplaysTheMatch(t *testing.T) {
	if testing.Short() {
		t.Skip("plays a whole match")
	}
	r := NewRoom(sim.Mode1v1, "tape")
	a, b := &overPeer{}, &overPeer{}
	r.Join(a)
	r.Join(b)
	tick := make(chan time.Time)
	done := make(chan struct{})
	go func() { r.Run(tick); close(done) }()
	const maxFrames = 1_000_000 // idle players: the match ends on serve faults
	for i := 0; i < maxFrames && !(a.ready() && b.ready()); i++ {
		tick <- time.Time{}
	}
	if !a.ready() || !b.ready() {
		t.Fatal("no game over + tape within the frame budget")
	}
	r.Leave(0, a, false)
	<-done
	for seat, p := range []*overPeer{a, b} {
		got := replaySeat(t, p, seat)
		if !slices.Equal(got[sim.SnapLen:2*sim.SnapLen], p.over[sim.SnapLen:2*sim.SnapLen]) {
			t.Fatalf("seat %d: replayed GameOver snapshot differs from the one the server sent", seat)
		}
	}
}
