package main

import (
	"context"
	"testing"
	"time"

	"github.com/txny34/futenis/cmd/server/internal/wire"
)

// A look and an emote reach the other lobby members over the socket, and the
// sender keeps showing up in their roster.
func TestLobbyRelaysLooksAndEmotes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	tick := make(chan time.Time)
	srv, _ := newLobbyServer(t, tick)
	a := dialLobby(ctx, t, srv, "10.5.9.1")
	b := dialLobby(ctx, t, srv, "10.5.9.2")
	look := []byte{wire.MsgLobbyStyle, wire.Version, 0, 0, 0, 0, 0, 0, 1, 3, 1, 4, 2, 2, 200, 220, 78}
	write(ctx, t, a, look)
	if got := readType(ctx, t, b, wire.MsgLobbyLook); got[wire.HeaderLen+8+1] != 3 || got[wire.HeaderLen+8+8] != 78 {
		t.Fatalf("look %v", got)
	}
	write(ctx, t, a, []byte{wire.MsgEmote, wire.Version, 0, 0, 0, 0, 0, 0, 2})
	if got := readType(ctx, t, b, wire.MsgLobbyEmote); got[len(got)-1] != 2 {
		t.Fatalf("emote %v", got)
	}
	write(ctx, t, a, wire.EncodePresence(1, 0, 1, 0))
	time.Sleep(30 * time.Millisecond)
	tick <- time.Time{}
	if r := readType(ctx, t, b, wire.MsgRoster); wire.F64(r, 0) != 1 {
		t.Fatalf("a missing from b's roster: count %v", wire.F64(r, 0))
	}
}
