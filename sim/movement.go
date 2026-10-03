package sim

// movement.go: wave1 player movement/body ownership (docs/design/
// contact-v2.md §4 "Salto", §5 "Movimiento, sprint y stamina", §19.4
// "Separación de cápsulas entre compañeros"). movePlayers replaces the
// wave-0 instant-velocity stub that used to live in physics.go.

// movePlayers advances every active player (s.NumPlayers) by one tick:
// vertical (jump/gravity/landing), horizontal (acceleration toward a
// sprint/charge-aware target speed), stamina drain/regen/lock, same-team
// capsule separation (2v2) and clamping to the court. Returns ev with any
// EvJump events appended.
func movePlayers(s *State, inputs [MaxPlayers]PlayerInput, ev []Event) []Event {
	for i := int8(0); i < s.NumPlayers; i++ {
		p := &s.Players[i]
		in := inputs[i]

		ev = stepPlayerVertical(s, i, in, ev)
		stepPlayerHorizontal(p, in, &s.Cfg)
		stepPlayerStamina(p, in, &s.Cfg)
		clampPlayerToCourt(p)
	}

	separateTeammates(s)
	for i := int8(0); i < s.NumPlayers; i++ {
		clampPlayerToCourt(&s.Players[i])
	}
	return ev
}

// stepPlayerVertical handles the jump/gravity/landing block (§4): a player
// can only jump from the ground, on BtnJump's press edge, with enough
// stamina (JumpStaminaCost is paid up front; it does not by itself trigger
// StaminaLock unless it happens to drain the tank to exactly 0). Gravity
// then integrates every airborne tick, and landing clamps Y/Vel.Y/OnGround
// back to the floor.
func stepPlayerVertical(s *State, i int8, in PlayerInput, ev []Event) []Event {
	p := &s.Players[i]
	cfg := &s.Cfg

	if p.OnGround && pressed(p, in, BtnJump) && p.Stamina >= cfg.JumpStaminaCost {
		p.Vel.Y = cfg.JumpSpeed
		p.OnGround = false
		p.Stamina -= cfg.JumpStaminaCost
		ev = append(ev, Event{Kind: EvJump, Tick: s.Tick, Pos: p.Pos, Side: p.Team, Player: i})
	}
	if !p.OnGround {
		p.Vel.Y -= float64(cfg.PlayerGravity * Dt)
	}
	p.Pos.Y += float64(p.Vel.Y * Dt)
	if p.Pos.Y <= 0 {
		p.Pos.Y, p.Vel.Y, p.OnGround = 0, 0, true
	}
	return ev
}

// stepPlayerHorizontal moves p.Vel.X/Z toward a target velocity (direction *
// cruise/sprint speed, slowed to ChargeMoveFactor while charging a strike)
// by at most accel*Dt, where accel is PlayerAccel on the ground or the
// reduced PlayerAirAccel in the air (§5). Momentum, not instant velocity:
// |Δvel| per tick is bounded by accel*Dt.
func stepPlayerHorizontal(p *Player, in PlayerInput, cfg *Config) {
	dir := Vec3{X: in.MoveX, Z: in.MoveZ}
	if l := dir.Len(); l > 1 {
		dir = dir.Scale(1 / l)
	}

	speed := cfg.PlayerSpeed
	if in.Buttons&BtnSprint != 0 && p.Stamina > 0 {
		speed = cfg.SprintSpeed
	}
	if p.ChargeTicks > 0 {
		speed *= cfg.ChargeMoveFactor
	}
	target := dir.Scale(speed)

	accel := cfg.PlayerAccel
	if !p.OnGround {
		accel = cfg.PlayerAirAccel
	}

	horiz := approachVec(Vec3{X: p.Vel.X, Z: p.Vel.Z}, target, float64(accel*Dt))
	p.Vel.X, p.Vel.Z = horiz.X, horiz.Z
	p.Pos.X += float64(p.Vel.X * Dt)
	p.Pos.Z += float64(p.Vel.Z * Dt)
}

// approachVec moves current toward target by at most maxDelta (a scalar
// magnitude on the X/Z plane), snapping exactly to target once within reach
// so accel curves land on the target speed instead of oscillating around
// it.
func approachVec(current, target Vec3, maxDelta float64) Vec3 {
	diff := target.Sub(current)
	d := diff.Len()
	if d <= maxDelta || d == 0 {
		return target
	}
	return current.Add(diff.Scale(maxDelta / d))
}

// isMovingInput reports whether the player is requesting any horizontal
// movement this tick (sprint only drains stamina while held AND moving;
// standing still with Shift held costs nothing, §5).
func isMovingInput(in PlayerInput) bool {
	return in.MoveX != 0 || in.MoveZ != 0
}

// stepPlayerStamina applies the boost-model resource curve (§5): drains
// only while BtnSprint is held, stamina remains, and the player is actually
// moving; hitting exactly 0 starts StaminaLockTime ticks during which it
// cannot regenerate; otherwise it regenerates up to 1.
func stepPlayerStamina(p *Player, in PlayerInput, cfg *Config) {
	sprinting := in.Buttons&BtnSprint != 0 && p.Stamina > 0 && isMovingInput(in)
	switch {
	case sprinting:
		p.Stamina -= float64(cfg.StaminaDrain * Dt)
		if p.Stamina <= 0 {
			p.Stamina = 0
			p.StaminaLock = uint16(cfg.stLockTicks)
		}
	case p.StaminaLock > 0:
		p.StaminaLock--
	default:
		p.Stamina += float64(cfg.StaminaRegen * Dt)
		if p.Stamina > 1 {
			p.Stamina = 1
		}
	}
}

// clampPlayerToCourt confines p to their own team's half of the court and
// within the side walls (unchanged from the wave-0 behavior, just moved
// here alongside the rest of player movement).
func clampPlayerToCourt(p *Player) {
	minX, maxX := sideXRange(p.Team)
	p.Pos.X = clamp(p.Pos.X, minX, maxX)
	zLimit := CourtHalfWidth - PlayerRadius
	p.Pos.Z = clamp(p.Pos.Z, -zLimit, zLimit)
}

// separateTeammates resolves capsule overlap between same-team partners in
// 2v2 (§19.4): only teammates are pushed apart (opponents are kept apart by
// sideXRange/the net already). Fixed pair order (team 0 = {0,1}, team 1 =
// {2,3}) and a pure function of positions make this deterministic
// regardless of array iteration order.
func separateTeammates(s *State) {
	if s.NumPlayers < 4 {
		return
	}
	separatePair(&s.Players[0], &s.Players[1])
	separatePair(&s.Players[2], &s.Players[3])
}

// separatePair pushes a and b apart on the X/Z plane, evenly, until they are
// exactly 2*PlayerRadius apart. The degenerate case (exactly coincident
// positions) breaks the tie along a fixed +Z axis instead of dividing by
// zero, so the result stays deterministic.
func separatePair(a, b *Player) {
	minDist := 2 * PlayerRadius
	delta := Vec3{X: b.Pos.X - a.Pos.X, Z: b.Pos.Z - a.Pos.Z}
	dist := delta.Len()
	if dist != 0 && dist >= minDist {
		return
	}

	n := Vec3{Z: 1}
	if dist != 0 {
		n = delta.Scale(1 / dist)
	}
	push := n.Scale((minDist - dist) / 2)
	a.Pos.X -= push.X
	a.Pos.Z -= push.Z
	b.Pos.X += push.X
	b.Pos.Z += push.Z
}
