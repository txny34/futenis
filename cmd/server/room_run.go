package main

import (
	"time"

	"github.com/txny34/futenis/ai"
	"github.com/txny34/futenis/game"
	"github.com/txny34/futenis/sim"
)

// leave frees slot. In 1v1 the room ends and the rival is told. In casual 2v2
// the seat is freed: a waiting room keeps waiting, a started one goes on with
// the bot in that seat; the room ends when no person is left. Leaving a
// started ranked match is a forfeit: the leaver's team loses and the match
// ends (no bot ever plays ranked).
func (r *room) leave(slot int) {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return
	}
	r.peers[slot] = nil
	r.inputs[slot] = sim.PlayerInput{}
	if r.mode == sim.Mode2v2 && !r.ranked && r.humansLocked() > 0 {
		r.mu.Unlock()
		return // the bot takes the seat (started) or it is free again (waiting)
	}
	forfeit := r.ranked && r.started && !r.reported && r.onOver != nil
	r.reported = r.reported || forfeit
	winner, users, onOver := 1-r.teamOf(slot), r.users, r.onOver
	r.closed = true
	r.stopHoldsLocked()
	others := r.peers
	r.peers = [sim.MaxPlayers]peer{}
	close(r.stop)
	r.mu.Unlock()
	if !forfeit {
		tellLeft(others)
		return
	}
	go func() { // report first so the players who stayed still get their RESULT
		onOver(winner, users)
		tellLeft(others)
	}()
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
		if w, over := gameOverWinner(out); over && onOver != nil && r.claimReport() {
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
