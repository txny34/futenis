package lobby

import (
	"testing"
	"time"

	"github.com/txny34/futenis/cmd/server/internal/names"
	"github.com/txny34/futenis/cmd/server/internal/peertest"
	"github.com/txny34/futenis/cmd/server/internal/wire"
)

func TestLobbyReportsHideANameAfterTwoNetworks(t *testing.T) {
	l := New()
	target, _ := l.Join(&peertest.Peer{})
	a, _ := l.Join(&peertest.Peer{})
	a2, _ := l.Join(&peertest.Peer{}) // second tab of a
	b, _ := l.Join(&peertest.Peer{})
	l.SetClient(target, "net-t", false)
	l.SetClient(a, "net-a", false)
	l.SetClient(a2, "net-a", false)
	l.SetClient(b, "net-b", false)
	l.SetName(target, "Ofensivo", time.Now())
	for _, id := range []int{a, a2, b, target} {
		l.SetPos(id, wire.Presence{})
	}
	if name, _ := l.Report(a, target+100); name != "" {
		t.Fatal("unknown target")
	}

	if name, hidden := l.Report(a, target); name != "Ofensivo" || hidden {
		t.Fatalf("first report: %q hidden=%v", name, hidden)
	}
	if name, hidden := l.Report(a2, target); hidden || name != "" {
		t.Fatal("two tabs of one network count once (and the repeat is not logged)")
	}
	if _, hidden := l.Report(target, target); hidden {
		t.Fatal("self reports are ignored")
	}
	if _, hidden := l.Report(b, target); !hidden {
		t.Fatal("a second network hides the name")
	}
	l.mu.Lock()
	got := l.peers[target].name
	l.mu.Unlock()
	if got != names.Fallback {
		t.Fatalf("name %q, want %q", got, names.Fallback)
	}
	l.SetName(target, "OtroNombre", time.Now().Add(time.Minute))
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.peers[target].name != names.Fallback {
		t.Fatal("a hidden name can't be changed back")
	}
}

func TestLobbyReportFromSameNetworkIgnored(t *testing.T) {
	l := New()
	target, _ := l.Join(&peertest.Peer{})
	sib, _ := l.Join(&peertest.Peer{})
	l.SetClient(target, "net", false)
	l.SetClient(sib, "net", false)
	l.SetName(target, "Alguien", time.Now())
	l.SetPos(sib, wire.Presence{})
	if name, _ := l.Report(sib, target); name != "" {
		t.Fatal("a player's own network can't report it")
	}
}

func TestLobbyReportNeedsAChosenNameAndAPlacedReporter(t *testing.T) {
	l := New()
	target, _ := l.Join(&peertest.Peer{})
	a, _ := l.Join(&peertest.Peer{})
	l.SetClient(target, "t", false)
	l.SetClient(a, "a", false)
	l.SetName(target, "Alguien", time.Now())
	if name, _ := l.Report(a, target); name != "" {
		t.Fatal("a reporter that never walked in the lobby doesn't count")
	}
	l.SetPos(a, wire.Presence{})
	nameless, _ := l.Join(&peertest.Peer{})
	l.SetClient(nameless, "n", false)
	if name, _ := l.Report(a, nameless); name != "" {
		t.Fatal("a player without a name can't be muted before choosing one")
	}
}
