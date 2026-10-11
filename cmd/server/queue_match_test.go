package main

import (
	"io"
	"log"
	"testing"
	"time"

	"github.com/txny34/futenis/sim"
)

func newMatchHub() *hub {
	h := newHub([]string{"*"}, log.New(io.Discard, "", 0))
	h.matching = true // tests drive rebalance by hand
	return h
}

// openAt queues key at elo and seats a fake peer there; since is backdated by waited.
func openAt(t *testing.T, h *hub, key string, elo int, waited time.Duration) (string, *fakePeer) {
	t.Helper()
	code, ok := h.queueRoom("ranked1v1", key, elo)
	if !ok {
		t.Fatalf("%s already queued", key)
	}
	p := &fakePeer{}
	if _, slot, _, _ := h.join(code, sim.Mode1v1, p); slot < 0 {
		t.Fatalf("%s not seated", key)
	}
	h.mu.Lock()
	_, o := h.queueFor("ranked1v1").find(code)
	o.since = time.Now().Add(-waited)
	h.mu.Unlock()
	return code, p
}

func TestRankedQueuePairsCloseRatings(t *testing.T) {
	h := newMatchHub()
	a, _ := openAt(t, h, "a", 1000, 0)
	if code, _ := h.queueRoom("ranked1v1", "b", 1120); code != a {
		t.Fatal("close ratings must share a room")
	}
	if code, _ := h.queueRoom("ranked1v1", "c", 1600); code == a {
		t.Fatal("a 600 gap must not pair at once")
	}
}

func TestRankedWindowWidensWithWaiting(t *testing.T) {
	h := newMatchHub()
	a, _ := openAt(t, h, "a", 1000, 20*time.Second) // gap 150 + 25*20 = 650
	if code, _ := h.queueRoom("ranked1v1", "c", 1600); code != a {
		t.Fatal("after 20 s a 600 gap fits")
	}
}

func TestCasualQueueIgnoresRatings(t *testing.T) {
	h := newMatchHub()
	a, _ := h.queueRoom("1v1", "a", 800)
	if code, _ := h.queueRoom("1v1", "b", 2000); code != a {
		t.Fatal("casual queues have one open room")
	}
}

func TestRebalanceMovesTheNewerLonelyWaiter(t *testing.T) {
	h := newMatchHub()
	old, pOld := openAt(t, h, "a", 1000, 0)
	_, pNew := openAt(t, h, "b", 1600, 0)
	h.rebalance("ranked1v1", time.Now())
	if closedSoon(pNew) || closedSoon(pOld) {
		t.Fatal("no move while the gap is too wide")
	}
	h.mu.Lock()
	_, o := h.queueFor("ranked1v1").find(old)
	o.since = time.Now().Add(-20 * time.Second) // the old room's window widened
	h.mu.Unlock()
	h.rebalance("ranked1v1", time.Now())
	if !closedSoon(pNew) {
		t.Fatal("the newer waiter must be asked to requeue")
	}
	if closedSoon(pOld) {
		t.Fatal("the older waiter stays")
	}
	if code, _ := h.queueRoom("ranked1v1", "b2", 1600); code != old {
		h.mu.Lock()
		defer h.mu.Unlock()
		t.Fatalf("requeued player went to %s, want the older room %s", code, old)
	}
}

func closedSoon(p *fakePeer) bool {
	for range 20 {
		p.mu.Lock()
		c := p.closed
		p.mu.Unlock()
		if c {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return false
}
