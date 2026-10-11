package match

import "github.com/txny34/futenis/cmd/server/internal/ranked"

// WaitingAlone: not started, exactly one person seated.
func (r *Room) WaitingAlone() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return !r.started && !r.closed && r.humansLocked() == 1
}

// RequeueAlone closes the room iff it still waits with exactly one person
// (atomically with join: a rival arriving now is refused and retries) and
// returns that person and their ranked claim; nil when the room moved on.
func (r *Room) RequeueAlone() (Peer, ranked.Claim) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.started || r.closed || r.humansLocked() != 1 {
		return nil, ranked.Claim{}
	}
	for i, p := range r.peers {
		if p != nil {
			r.closed = true
			close(r.stop)
			r.peers[i] = nil
			return p, r.claims[i]
		}
	}
	return nil, ranked.Claim{}
}
