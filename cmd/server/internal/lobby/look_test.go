package lobby

import (
	"testing"
	"time"

	"github.com/txny34/futenis/cmd/server/internal/peertest"
	"github.com/txny34/futenis/cmd/server/internal/wire"
)

func count(p *peertest.Peer, typ byte) int {
	msgs, _ := p.Snapshot()
	n := 0
	for _, m := range msgs {
		if m[0] == typ {
			n++
		}
	}
	return n
}

func TestLookRelayedThrottledAndSentToNewcomers(t *testing.T) {
	l := New()
	a, b := &peertest.Peer{}, &peertest.Peer{}
	ida, _ := l.Join(a)
	l.Join(b)
	t0 := time.Now()
	l.SetLook(ida, wire.Style{1, 2}, t0)
	l.SetLook(ida, wire.Style{1, 3}, t0.Add(lookEvery/2)) // too soon
	l.SetLook(ida, wire.Style{1, 3}, t0.Add(lookEvery))
	if n := count(b, wire.MsgLobbyLook); n != 2 || count(a, wire.MsgLobbyLook) != 0 {
		t.Fatalf("b got %d looks, a got %d", n, count(a, wire.MsgLobbyLook))
	}
	c := &peertest.Peer{}
	l.Join(c)
	msgs, _ := c.Snapshot()
	if len(msgs) != 1 || msgs[0][0] != wire.MsgLobbyLook || msgs[0][wire.HeaderLen+8+1] != 3 {
		t.Fatalf("newcomer got %v", msgs)
	}
}

func TestEmoteRelayedOncePerSecond(t *testing.T) {
	l := New()
	a, b := &peertest.Peer{}, &peertest.Peer{}
	ida, _ := l.Join(a)
	l.Join(b)
	t0 := time.Now()
	l.Emote(ida, 1, t0)
	l.Emote(ida, 2, t0.Add(emoteEvery/2))
	if n := count(b, wire.MsgLobbyEmote); n != 1 {
		t.Fatalf("%d emotes relayed, want 1", n)
	}
}

func TestDecodeEmoteAndLobbyStyle(t *testing.T) {
	ok := []byte{wire.MsgEmote, wire.Version, 0, 0, 0, 0, 0, 0, 3}
	if k, good := wire.DecodeEmote(ok); !good || k != 3 {
		t.Fatal("valid emote refused")
	}
	bad := append([]byte{}, ok...)
	bad[wire.HeaderLen] = wire.EmoteKinds
	if _, good := wire.DecodeEmote(bad); good {
		t.Fatal("unknown emote kind accepted")
	}
	for _, n := range []int{8, 9, wire.StyleLen} { // v1, v2 and v3 looks
		b := make([]byte, wire.HeaderLen+n)
		b[0], b[1] = wire.MsgLobbyStyle, wire.Version
		if st, good := wire.DecodeLobbyStyle(b); !good || st[0] != 1 {
			t.Fatalf("%d-byte look refused", n)
		}
	}
}
