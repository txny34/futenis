package ai

import (
	"math"

	"github.com/txny34/futenis/sim"
)

const (
	// replanEvery is how often (ticks) a still-valid intercept is recomputed.
	replanEvery = 3
	// maxHoldTicks force-releases a charge that lost its intercept (2 s).
	maxHoldTicks = 2 * sim.TickRate
	// arriveRadius is the distance (m) under which the bot stops moving.
	arriveRadius = 0.05
	// slowRadius is the distance (m) under which the bot eases its speed.
	slowRadius = 0.5
	// sprintDistance is the minimum distance (m) worth sprinting for.
	sprintDistance = 2.0
	// homeDepth is the recovery distance (m) from the net.
	homeDepth = 7.0
	// jumpLeadTicks is when, before contact, the bot presses jump.
	jumpLeadTicks = 20
)

// Bot is a deterministic, allocation-free AI controller for one slot.
// The zero value is not useful: build it with New.
type Bot struct {
	Level Level
	// Seed decorrelates otherwise identical bots (harness, 2v2 partners).
	Seed uint64

	belief     sim.Ball // the ball as the bot currently believes it moves
	beliefTick uint32
	hasBelief  bool
	countdown  int // reaction ticks left before the belief resyncs

	plan        intercept
	tgt         sim.Vec3       // where the bot is heading for the planned contact
	prof        contactProfile // release lookahead scratch (no allocs)
	win         bounceWindow
	planAge     int
	holdTicks   int     // ticks the current charge has been held
	shot        uint8   // BtnStrike or BtnLob while charging, else 0
	aimed       bool    // target/shot latched for the coming contact
	mistimed    bool    // this shot will be released far too early (Level.MissChance)
	missed      bool    // a mistimed shot was released: no retry this crossing
	missedShot  int     // sim.RallyState.ShotCount when it was missed
	pendingShot uint8   // shot to press once the charge window opens
	chargeWant  int     // desired charge ticks for the latched shot
	cooldown    int     // ticks left of a released strike buffer (no re-press)
	aimX        float64 // latched target
	aimZ        float64
	spin        int8
	serve       servePlan // this serve attempt's variety rolls
}

// New returns a bot with the preset Level for difficulty, seeded by slot.
func New(difficulty Difficulty, slot int) Bot {
	return Bot{Level: LevelOf(difficulty), Seed: uint64(slot) + 1}
}

// Think returns the input for player index slot this tick. It only reads s
// and mutates the bot's own memory, so the same call sequence always yields
// the same inputs (deterministic, zero allocations).
func (b *Bot) Think(s *sim.State, slot int) sim.PlayerInput {
	if slot < 0 || slot >= int(s.NumPlayers) {
		return sim.PlayerInput{}
	}
	b.updateBelief(s)
	if b.cooldown > 0 {
		b.cooldown--
	}
	switch s.Phase {
	case sim.PhaseServe:
		b.missed = false
		return b.thinkServe(s, slot)
	case sim.PhaseRally:
		return b.thinkRally(s, slot)
	default:
		b.missed = false
		b.resetCharge()
		return b.recover(s, slot)
	}
}

// updateBelief advances the bot's ball belief and resyncs it with the real
// ball ReactionTicks after a divergence (the bot "notices" late).
func (b *Bot) updateBelief(s *sim.State) {
	staticServe := s.Phase == sim.PhaseServe && !s.Ball.Tossed
	if !b.hasBelief || s.Tick != b.beliefTick+1 || staticServe || s.Phase != sim.PhaseRally && s.Phase != sim.PhaseServe {
		b.syncBelief(s)
		return
	}
	b.belief, _ = stepBall(b.belief, &s.Cfg)
	b.beliefTick = s.Tick
	if b.countdown > 0 {
		b.countdown--
		if b.countdown == 0 {
			b.syncBelief(s)
		}
		return
	}
	if b.belief.Pos.Sub(s.Ball.Pos).Len() > beliefTolerance {
		if b.Level.ReactionTicks <= 0 {
			b.syncBelief(s)
			return
		}
		b.countdown = b.Level.ReactionTicks
	}
}

func (b *Bot) syncBelief(s *sim.State) {
	b.belief, b.beliefTick, b.hasBelief, b.countdown = s.Ball, s.Tick, true, 0
	b.planAge = replanEvery // force a replan
}

// hash returns deterministic per-tick, per-slot bits for "human error".
func (b *Bot) hash(s *sim.State, slot int, salt uint64) uint64 {
	return sim.Hash64(uint64(s.Tick)<<16 ^ uint64(slot)<<8 ^ salt ^ b.Seed*0x9E3779B97F4A7C15)
}

// unit maps hash bits to [0, 1).
func unit(h uint64) float64 { return float64(h>>11) * 0x1p-53 }

// signed maps hash bits to [-1, 1).
func signed(h uint64) float64 { return unit(h)*2 - 1 }

func (b *Bot) resetCharge() { b.shot, b.holdTicks, b.aimed, b.mistimed = 0, 0, false, false }

// release lets go of the charge and blocks re-pressing while the sim's
// strike buffer is live (a re-press would cancel it).
func (b *Bot) release(cfg *sim.Config) {
	b.resetCharge()
	b.cooldown = ticksOfSec(cfg.StrikeBufferTime) + 1
}

// forward is +1 when team attacks toward +X.
func forward(team int8) float64 {
	if team == 0 {
		return 1
	}
	return -1
}

// rightSign is the Z sign of a team's right-hand side (facing the net).
func rightSign(team int8) float64 { return forward(team) }

// speedFor is the planning speed: sprint if stamina management allows it.
func (b *Bot) speedFor(p *sim.Player, cfg *sim.Config) float64 {
	if p.Stamina > b.Level.MinSprintStamina && cfg.SprintSpeed > cfg.PlayerSpeed {
		return cfg.SprintSpeed
	}
	return cfg.PlayerSpeed
}

// moveTo steers p toward (x, z) with arrival easing; may sprint.
func (b *Bot) moveTo(p *sim.Player, x, z float64, urgent bool) sim.PlayerInput {
	dx, dz := x-p.Pos.X, z-p.Pos.Z
	d := math.Sqrt(float64(dx*dx) + float64(dz*dz))
	in := sim.PlayerInput{TargetX: b.aimX, TargetZ: b.aimZ, Spin: b.spin}
	if d < arriveRadius {
		return in
	}
	scale := math.Min(1, d/slowRadius) / d
	in.MoveX, in.MoveZ = float64(dx*scale), float64(dz*scale)
	if urgent && d > sprintDistance && p.Stamina > b.Level.MinSprintStamina {
		in.Buttons |= sim.BtnSprint
	}
	return in
}

// home returns the recovery spot for slot (2v2 partners split by Slot).
func home(s *sim.State, slot int) (x, z float64) {
	p := &s.Players[slot]
	x = -forward(p.Team) * homeDepth
	if s.NumPlayers >= 4 {
		side := 1.0
		if p.Slot == 1 {
			side = -1
		}
		z = side * rightSign(p.Team) * sim.CourtHalfWidth / 2
	}
	return x, z
}

// recover walks back to the home spot, drifting toward the ball's lane.
func (b *Bot) recover(s *sim.State, slot int) sim.PlayerInput {
	x, z := home(s, slot)
	if s.NumPlayers < 4 {
		z = float64(b.belief.Pos.Z * 0.4)
	}
	return b.moveTo(&s.Players[slot], x, z, false)
}
