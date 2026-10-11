package lobby

import (
	"testing"

	"github.com/txny34/futenis/cmd/server/internal/peertest"
)

func TestLobbyFull(t *testing.T) {
	l := New()
	for i := 0; i < Max; i++ {
		if _, ok := l.Join(&peertest.Peer{}); !ok {
			t.Fatalf("join %d rejected", i)
		}
	}
	if _, ok := l.Join(&peertest.Peer{}); ok {
		t.Fatal("lobby over capacity")
	}
}
