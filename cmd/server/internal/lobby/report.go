package lobby

import (
	"github.com/txny34/futenis/cmd/server/internal/names"
	"github.com/txny34/futenis/cmd/server/internal/wire"
)

// Lobby moderation: anyone can report another player's name (REPORT, body: the
// target id). Once ReportsToHide different networks (client limit keys, so
// two tabs count once) reported it, the name becomes names.Fallback for everyone
// and stays so for that connection. Hiding a player for oneself is client-only.
const (
	ReportsToHide = 2
)

// SetClient records id's network (limit key, for report counting) and roster format.
func (l *Lobby) SetClient(id int, key string, compact bool) {
	l.mu.Lock()
	if lp := l.peers[id]; lp != nil {
		lp.key, lp.compact = key, compact
	}
	l.mu.Unlock()
}

// Report counts one Report of target's name by from (network key). It returns
// the reported name and whether this Report hid it.
func (l *Lobby) Report(from, target int) (name string, hidden bool) {
	l.mu.Lock()
	src, t := l.peers[from], l.peers[target]
	// Only someone actually in the lobby (placed) reports, once per network, a
	// real chosen name; a repeat returns "" (nothing to log).
	if src == nil || t == nil || from == target || t.flagged || !src.placed || src.key == t.key ||
		t.name == "" || t.name == names.Fallback || t.reportedBy[src.key] {
		l.mu.Unlock()
		return "", false
	}
	if t.reportedBy == nil {
		t.reportedBy = map[string]bool{}
	}
	t.reportedBy[src.key] = true
	name = t.name
	if len(t.reportedBy) < ReportsToHide {
		l.mu.Unlock()
		return name, false
	}
	t.flagged, t.name = true, names.Fallback
	all := l.othersLocked(-1)
	l.mu.Unlock()
	msg := wire.EncodeLobbyName(target, names.Fallback)
	for _, o := range all {
		_ = o.Send(msg)
	}
	return name, true
}
