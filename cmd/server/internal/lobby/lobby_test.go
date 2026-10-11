package lobby

import (
	"sync"
	"testing"
)

// fakePeer records what the lobby sends.
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

func TestLobbyFull(t *testing.T) {
	l := New()
	for i := 0; i < Max; i++ {
		if _, ok := l.Join(&fakePeer{}); !ok {
			t.Fatalf("join %d rejected", i)
		}
	}
	if _, ok := l.Join(&fakePeer{}); ok {
		t.Fatal("lobby over capacity")
	}
}
