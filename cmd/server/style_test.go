package main

import (
	"context"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/txny34/futenis/cmd/server/internal/wire"
)

// A player's STYLE reaches the rival (and itself) as STYLES in server order.
func TestStyleReachesTheRival(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv := newTestHub(make(chan time.Time))
	defer srv.Close()
	c1, c2, _ := startPair(ctx, t, srv)
	if err := c2.Write(ctx, websocket.MessageBinary, wire.EncodeStyle(wire.Style{0, 4, 1})); err != nil {
		t.Fatal(err)
	}
	for _, c := range []*websocket.Conn{c1, c2} {
		m := readMsg(ctx, t, c, wire.MsgStyles, wire.StylesMsgLen)
		if got := m[wire.HeaderLen+wire.StyleLen : wire.HeaderLen+wire.StyleLen+3]; got[0] != 1 || got[1] != 4 || got[2] != 1 {
			t.Fatalf("seat 1 look %v", got)
		}
	}
}
