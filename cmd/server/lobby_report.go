package main

// Lobby moderation: anyone can report another player's name (REPORT, body: the
// target id). Once reportsToHide different networks (client limit keys, so
// two tabs count once) reported it, the name becomes fallbackName for everyone
// and stays so for that connection. Hiding a player for oneself is client-only.
const (
	msgReport     = 0x12 // client: f64 target id
	reportsToHide = 2
)

func isReportMsg(b []byte) bool {
	return len(b) == headerLen+8 && b[0] == msgReport && b[1] == protoVersion
}

// setKey records id's network (limit key) for report counting.
func (l *lobby) setKey(id int, key string) {
	l.mu.Lock()
	if lp := l.peers[id]; lp != nil {
		lp.key = key
	}
	l.mu.Unlock()
}

// report counts one report of target's name by from (network key). It returns
// the reported name and whether this report hid it.
func (l *lobby) report(from, target int) (name string, hidden bool) {
	l.mu.Lock()
	src, t := l.peers[from], l.peers[target]
	if src == nil || t == nil || from == target || t.flagged || src.key == t.key {
		l.mu.Unlock()
		return "", false
	}
	if t.reportedBy == nil {
		t.reportedBy = map[string]bool{}
	}
	t.reportedBy[src.key] = true
	name = t.name
	if len(t.reportedBy) < reportsToHide {
		l.mu.Unlock()
		return name, false
	}
	t.flagged, t.name = true, fallbackName
	all := l.othersLocked(-1)
	l.mu.Unlock()
	msg := encodeLobbyName(target, fallbackName)
	for _, o := range all {
		_ = o.send(msg)
	}
	return name, true
}
