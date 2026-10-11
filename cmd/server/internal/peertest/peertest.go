// Package peertest is a recording connection for server tests: it satisfies
// the Peer interfaces of the match and lobby packages.
package peertest

import "sync"

// Peer records every message sent to it and whether it was closed.
type Peer struct {
	mu     sync.Mutex
	msgs   [][]byte
	closed bool
}

// Send records a copy of b.
func (p *Peer) Send(b []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.msgs = append(p.msgs, append([]byte{}, b...))
	return nil
}

// Close marks the peer closed.
func (p *Peer) Close(int, string) {
	p.mu.Lock()
	p.closed = true
	p.mu.Unlock()
}

// Snapshot returns the messages so far and whether the peer was closed.
func (p *Peer) Snapshot() ([][]byte, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([][]byte{}, p.msgs...), p.closed
}
