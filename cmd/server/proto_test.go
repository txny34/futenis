package main

import (
	"context"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/txny34/futenis/cmd/server/internal/wire"
)

func TestHubRefusesOtherProtocolVersion(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv := newTestHub(make(chan time.Time))
	defer srv.Close()
	c := dialRoom(ctx, t, srv, "ABC&proto=99")
	expectClose(ctx, t, c, wire.CloseOutdated)
}

func TestHubAcceptsCurrentOrMissingProtocolVersion(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv := newTestHub(make(chan time.Time))
	defer srv.Close()
	for _, code := range []string{"ABC&proto=1", "XYZ"} {
		c := dialRoom(ctx, t, srv, code)
		readMsg(ctx, t, c, wire.MsgWaiting, wire.HeaderLen)
	}
}

func TestLobbyRefusesOtherProtocolVersion(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv, _ := newLobbyServer(t, make(chan time.Time))
	c, _, err := websocket.Dial(ctx, "ws"+srv.URL[4:]+"/?proto=99", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.CloseNow()
	expectClose(ctx, t, c, wire.CloseOutdated)
}
