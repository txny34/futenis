package main

import (
	"testing"
	"time"
)

func TestLobbyReportsHideANameAfterTwoNetworks(t *testing.T) {
	l := newLobby()
	target, _ := l.join(&fakePeer{})
	a, _ := l.join(&fakePeer{})
	a2, _ := l.join(&fakePeer{}) // second tab of a
	b, _ := l.join(&fakePeer{})
	l.setKey(target, "net-t")
	l.setKey(a, "net-a")
	l.setKey(a2, "net-a")
	l.setKey(b, "net-b")
	l.setName(target, "Ofensivo", time.Now())

	if name, hidden := l.report(a, target); name != "Ofensivo" || hidden {
		t.Fatalf("first report: %q hidden=%v", name, hidden)
	}
	if _, hidden := l.report(a2, target); hidden {
		t.Fatal("two tabs of one network count once")
	}
	if _, hidden := l.report(target, target); hidden {
		t.Fatal("self reports are ignored")
	}
	if _, hidden := l.report(b, target); !hidden {
		t.Fatal("a second network hides the name")
	}
	l.mu.Lock()
	got := l.peers[target].name
	l.mu.Unlock()
	if got != fallbackName {
		t.Fatalf("name %q, want %q", got, fallbackName)
	}
	l.setName(target, "OtroNombre", time.Now().Add(time.Minute))
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.peers[target].name != fallbackName {
		t.Fatal("a hidden name can't be changed back")
	}
}

func TestLobbyReportFromSameNetworkIgnored(t *testing.T) {
	l := newLobby()
	target, _ := l.join(&fakePeer{})
	sib, _ := l.join(&fakePeer{})
	l.setKey(target, "net")
	l.setKey(sib, "net")
	if name, _ := l.report(sib, target); name != "" {
		t.Fatal("a player's own network can't report it")
	}
}
