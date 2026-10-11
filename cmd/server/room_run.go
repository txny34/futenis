package main

import (
	"time"

	"github.com/txny34/futenis/ai"
	"github.com/txny34/futenis/game"
	"github.com/txny34/futenis/sim"
)

// leave frees slot when p still owns it (nil: whoever does; a replaced
// connection must not free its successor's seat) and reports whether it did.
// In 1v1 the room ends and the rival is told. In casual 2v2
// the seat is freed: a waiting room keeps waiting, a started one goes on with
// the bot in that seat; the room ends when no person is left. Leaving a
// started ranked match is a forfeit: the leaver's team loses and the match
// ends (no bot ever plays ranked). With a seat held for a dropped player the
// ranked match ends without a result: nobody loses to a shared network blip.
func (r *room) leave(slot int, p peer) bool {
	r.mu.Lock()
	if r.closed || (p != nil && r.peers[slot] != p) {
		r.mu.Unlock()
		return false
	}
	r.peers[slot] = nil
	r.inputs[slot] = sim.PlayerInput{}
	if r.mode == sim.Mode2v2 && !r.ranked && r.humansLocked() > 0 {
		r.mu.Unlock()
		return true // the bot takes the seat (started) or it is free again (waiting)
	}
	forfeit := r.ranked && r.started && !r.reported && r.onOver != nil && !r.anyAwayLocked()
	r.reported = r.reported || r.ranked // a no-contest is never reported later either
	winner, users, onOver := 1-r.teamOf(slot), r.users, r.onOver
	r.closed = true
	gone, release := r.stopHoldsLocked(), r.release
	others := r.peers
	r.peers = [sim.MaxPlayers]peer{}
	close(r.stop)
	r.mu.Unlock()
	for _, uid := range gone {
		if release != nil {
			release(uid)
		}
	}
	if !forfeit {
		tellLeft(others)
		return true
	}
	go func() { // report first so the players who stayed still get their RESULT
		onOver(winner, users)
		tellLeft(others)
	}()
	return true
}

func tellLeft(others [sim.MaxPlayers]peer) {
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
	mode, mask, onOver := r.mode, r.humanMaskLocked(), r.onOver
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
		in, peers, now, paused := r.inputs, r.peers, r.humanMaskLocked(), r.pausedLocked()
		r.mu.Unlock()
		if paused {
			continue // a dropped player may come back (room_rejoin.go); no frames: events would repeat
		}
		if now != mask {
			mask = now
			_ = s.SetHumanMask(mask) // someone left: the bot takes the seat
		}
		s.Run(in, ticksPerFrame, out)
		w, over := gameOverWinner(out)
		if over {
			r.markOver()
		}
		if over && onOver != nil && r.claimReport() {
			r.mu.Lock()
			users := r.users
			r.mu.Unlock()
			go onOver(w, users)
		}
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
