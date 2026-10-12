package ai

import (
	"math"

	"github.com/txny34/futenis/sim"
)

// 2v2 play (futvóley): a bot that may pass (sim.PassReady) sets its partner
// up with a quick Lob tap PassChance of the time; the partner may set it
// back (A-B-A) and the last touch finishes with a full-power smash to the
// open corner. A lob that is meant to cross is charged past the tap.

// passTapTicks is the bot's Lob tap for a pass: well under sim.PassMaxCharge.
const passTapTicks = 2

// choosePass decides a pass for this touch and latches it (aim is ignored by
// the sim's set). Reports whether it did.
func (b *Bot) choosePass(s *sim.State, slot int) bool {
	if !sim.PassReady(s, int8(slot)) || unit(b.hash(s, slot, 11)) >= b.Level.PassChance {
		return false
	}
	b.spin = 0
	b.mistimed = false // a set is a soft, easy touch
	b.pendingShot, b.aimed, b.chargeWant = sim.BtnLob, true, passTapTicks
	return true
}

// lobCharge is how long a crossing lob charges: a quarter of the max, but
// past the pass tap while a release could still be a 2v2 pass.
func lobCharge(s *sim.State, slot, maxT int) int {
	if sim.PassReady(s, int8(slot)) {
		return max(maxT/4, int(math.Ceil(sim.PassMaxCharge*float64(maxT)))+2)
	}
	return maxT / 4
}

// finishing reports whether this bot's touch finishes its partner's set
// (its team already holds the ball): the smash of the play.
func finishing(s *sim.State, slot int) bool {
	return s.Rally.LastTeam == s.Players[slot].Team
}
