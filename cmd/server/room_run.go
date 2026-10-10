package main

import (
	"time"

	"github.com/txny34/futenis/ai"
	"github.com/txny34/futenis/game"
	"github.com/txny34/futenis/sim"
)

// leave frees slot. In 1v1 the room ends and the rival is told. In 2v2 the
// seat is freed: a waiting room keeps waiting, a started one goes on with the
// bot in that seat; the room ends when no person is left.
func (r *room) leave(slot int) {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return
	}
	r.peers[slot] = nil
	r.inputs[slot] = sim.PlayerInput{}
	if r.mode == sim.Mode2v2 && r.humansLocked() > 0 {
		r.mu.Unlock()
		return // the bot takes the seat (started) or it is free again (waiting)
	}
	r.closed = true
	others := r.peers
	r.peers = [sim.MaxPlayers]peer{}
	close(r.stop)
	r.mu.Unlock()
	for _, o := range others {
		if o != nil {
			_ = o.send(encodeSignal(msgPeerLeft))
			o.close(closeNormal, "peer left")
		}
	}
}

// run plays the match until the room stops. tick is injectable for tests.
func (r *room) run(tick <-chan time.Time) {
	r.mu.Lock()
	mode, mask := r.mode, r.humanMaskLocked()
	r.mu.Unlock()
	s := game.NewSession()
	if err := s.Configure(int(mode), int(ai.Normal), roomPointsToWin, mask); err != nil {
		panic(err) // arguments are validated constants/masks: a failure is a programming error
	}
	out := make([]float64, game.OutLen)
	scratch := make([]float64, game.OutLen)
	view := make([]float64, game.OutLen)
	msgs := [sim.MaxPlayers][]byte{}
	for i := range msgs {
		msgs[i] = make([]byte, frameMsgLen)
	}
	for {
		select {
		case <-r.stop:
			return
		case <-tick:
		}
		r.mu.Lock()
		in, peers, now := r.inputs, r.peers, r.humanMaskLocked()
		r.mu.Unlock()
		if now != mask {
			mask = now
			_ = s.SetHumanMask(mask) // someone left: the bot takes the seat
		}
		s.Run(in, ticksPerFrame, out)
		for i, p := range peers {
			if p == nil {
				continue
			}
			game.ViewOut(view, out, scratch, i, mode == sim.Mode2v2)
			encodeFrame(msgs[i], view)
			sendTo(p, msgs[i])
		}
	}
}

func sendTo(p peer, b []byte) {
	if p != nil {
		_ = p.send(b) // a dead peer is detected by its read loop, which calls leave
	}
}
