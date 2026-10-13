package match

import (
	"testing"
	"time"

	"github.com/txny34/futenis/cmd/server/internal/peertest"
	"github.com/txny34/futenis/cmd/server/internal/wire"
	"github.com/txny34/futenis/sim"
)

// lastStyles returns the newest STYLES message p got (nil if none).
func lastStyles(p *peertest.Peer) []byte {
	msgs, _ := p.Snapshot()
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i][0] == wire.MsgStyles {
			return msgs[i]
		}
	}
	return nil
}

func styleAt(msg []byte, slot int) wire.Style {
	var st wire.Style
	copy(st[:], msg[wire.HeaderLen+slot*wire.StyleLen:])
	return st
}

func countStyles(p *peertest.Peer) int {
	msgs, _ := p.Snapshot()
	n := 0
	for _, m := range msgs {
		if m[0] == wire.MsgStyles {
			n++
		}
	}
	return n
}

func TestStyleRelayedToEverySeat(t *testing.T) {
	r := NewRoom(sim.Mode1v1, "test")
	a, b := &peertest.Peer{}, &peertest.Peer{}
	r.Join(a)
	r.Join(b)
	st := wire.Style{1, 3, 1, 2, 4, 0, 128, 200}
	t0 := time.Now()
	r.SetStyle(1, b, st, t0)
	for _, p := range []*peertest.Peer{a, b} {
		msg := lastStyles(p)
		if msg == nil || len(msg) != wire.StylesMsgLen || styleAt(msg, 1) != st || styleAt(msg, 0) != (wire.Style{}) {
			t.Fatalf("STYLES = %v", msg)
		}
	}
	later := t0.Add(styleEvery)
	r.SetStyle(1, b, st, later)                              // unchanged: no new relay
	r.SetStyle(0, b, st, later)                              // not b's seat: ignored
	r.SetStyle(1, b, wire.Style{1, 4}, t0.Add(styleEvery/2)) // too soon: dropped (spam)
	if n := countStyles(a); n != 1 {
		t.Fatalf("%d STYLES relays, want 1", n)
	}
	r.SetStyle(1, b, wire.Style{1, 4}, later)
	if n := countStyles(a); n != 2 {
		t.Fatalf("%d STYLES relays after the wait, want 2", n)
	}
}

func TestStyleIgnoredBeforeTheMatchStarts(t *testing.T) {
	r := NewRoom(sim.Mode1v1, "test")
	a := &peertest.Peer{}
	r.Join(a)
	r.SetStyle(0, a, wire.Style{1, 2}, time.Now())
	if lastStyles(a) != nil {
		t.Fatal("a waiting room relayed a look")
	}
}

func TestStyleClearedWhenTheBotTakesTheSeat(t *testing.T) {
	r := NewRoom(sim.Mode2v2, "test")
	a, b := &peertest.Peer{}, &peertest.Peer{}
	sa, _, _ := r.Join(a)
	sb, _, _ := r.Join(b)
	r.StartEarly()
	r.SetStyle(sb, b, wire.Style{1, 2}, time.Now())
	r.Leave(sb, b, false)
	if msg := lastStyles(a); msg == nil || styleAt(msg, sb) != (wire.Style{}) {
		t.Fatalf("leaver's look kept: %v", msg)
	}
	if lastStyles(a) == nil || sa == sb {
		t.Fatal("setup")
	}
}

func TestReattachSendsEveryLook(t *testing.T) {
	r := NewRoom(sim.Mode1v1, "test")
	a, b := &peertest.Peer{}, &peertest.Peer{}
	r.Join(a)
	r.Join(b)
	tokens := r.IssueTokens()
	r.SetStyle(0, a, wire.Style{1, 5}, time.Now())
	back := &peertest.Peer{}
	if !r.Reattach(1, tokens[1], back) {
		t.Fatal("reattach refused")
	}
	r.SetStyle(1, b, wire.Style{1, 9}, time.Now().Add(time.Hour)) // the replaced connection
	if msg := lastStyles(back); styleAt(msg, 1) != (wire.Style{}) {
		t.Fatalf("a replaced connection changed the look: %v", msg)
	}
	if msg := lastStyles(back); msg == nil || styleAt(msg, 0) != (wire.Style{1, 5}) {
		t.Fatalf("rejoined seat got %v", msg)
	}
}

func TestDecodeStyle(t *testing.T) {
	st, ok := wire.DecodeStyle(wire.EncodeStyle(wire.Style{0, 1, 2, 3, 4, 5, 6, 7}))
	if !ok || st != (wire.Style{1, 1, 2, 3, 4, 5, 6, 7}) {
		t.Fatalf("got %v %v (present must be forced on)", st, ok)
	}
	v2 := wire.EncodeStyle(wire.Style{0, 1, 2, 3, 4, 5, 6, 7, 8})[:wire.HeaderLen+9]
	if st, ok := wire.DecodeStyle(v2); !ok || st[8] != 8 || st[9] != 0 {
		t.Fatalf("9-byte look: %v %v (tail must be zero)", st, ok)
	}
	if _, ok := wire.DecodeStyle(make([]byte, wire.StyleMsgLen+1)); ok {
		t.Fatal("wrong length accepted")
	}
}
