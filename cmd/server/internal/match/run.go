package match

import (
	"time"

	"github.com/txny34/futenis/ai"
	"github.com/txny34/futenis/cmd/server/internal/wire"
	"github.com/txny34/futenis/game"
	"github.com/txny34/futenis/sim"
)

// Leave frees slot when p still owns it (nil: whoever does; a replaced
// connection must not free its successor's seat) and reports whether it did.
// In 1v1 the room ends and the rival is told. In casual 2v2
// the seat is freed: a waiting room keeps waiting, a started one goes on with
// the bot in that seat; the room ends when no person is left. Leaving a
// started ranked match is a forfeit: the leaver's team loses and the match
// ends (no bot ever plays ranked). A ranked match whose last connected player
// drops while another seat is held ends without a result (shared outage);
// leaving on purpose is still a forfeit.
// dropped: p's connection was lost too (no hold possible): with another seat
// already held that is a shared outage, not a forfeit by p.
func (r *Room) Leave(slot int, p Peer, dropped bool) bool {
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
	forfeit := r.ranked && r.started && !r.reported && r.onOver != nil && !(dropped && r.anyAwayLocked())
	r.reported = r.reported || r.ranked // a no-contest is never reported later either
	winner, users, onOver := 1-r.TeamOf(slot), r.users, r.onOver
	r.closed = true
	gone, release := r.stopHoldsLocked(), r.release
	others := r.peers
	r.peers = [sim.MaxPlayers]Peer{}
	close(r.stop)
	r.mu.Unlock()
	for _, c := range gone {
		if release != nil {
			release(c)
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

func tellLeft(others [sim.MaxPlayers]Peer) {
	for _, o := range others {
		if o != nil {
			_ = o.Send(wire.EncodeSignal(wire.MsgPeerLeft))
			o.Close(wire.CloseNormal, "peer left")
		}
	}
}

// Run plays the match until the room stops. tick is injectable for tests.
func (r *Room) Run(tick <-chan time.Time) {
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
		msgs[i] = make([]byte, wire.FrameLen)
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
			continue // a dropped player may come back (rejoin.go); no frames: events would repeat
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
			wire.EncodeFrame(msgs[i], view)
			SendTo(p, msgs[i])
		}
	}
}

// SendTo sends b to p when the seat is taken; a send error is left to p's read loop.
func SendTo(p Peer, b []byte) {
	if p != nil {
		_ = p.Send(b) // a dead peer is detected by its read loop, which calls leave
	}
}

// gameOverWinner reports the winning team (server space) once the frame's
// current snapshot is in GameOver.
func gameOverWinner(out []float64) (int, bool) {
	cur := out[sim.SnapLen:]
	if int(cur[sim.SnapPhase]) != int(sim.PhaseGameOver) {
		return 0, false
	}
	if cur[sim.SnapScore1] > cur[sim.SnapScore0] {
		return 1, true
	}
	return 0, true
}
