package sim

import "testing"

// newBodyTestState is a minimal 1v1 state for ball-vs-capsule tests: player
// 1 starts far away so it never spuriously participates.
func newBodyTestState() State {
	s := State{Cfg: DefaultConfig(), NumPlayers: 2}
	s.Players[0] = Player{Team: 0, OnGround: true}
	s.Players[1] = Player{Team: 1, Pos: Vec3{X: 100}, OnGround: true}
	return s
}

// TestBodyHit_BouncesOffCapsule_Restitution is test plan #31b.
func TestBodyHit_BouncesOffCapsule_Restitution(t *testing.T) {
	s := newBodyTestState()
	s.Ball.Pos = Vec3{X: PlayerRadius + BallRadius - 0.01, Y: 1}
	s.Ball.Vel = Vec3{X: -5}

	ev := applyBodyCollisions(&s, nil)

	found := false
	for _, e := range ev {
		if e.Kind == EvBodyHit {
			found = true
			if e.Side != 0 || e.Player != 0 {
				t.Errorf("EvBodyHit: got Side=%d Player=%d, want 0,0", e.Side, e.Player)
			}
		}
	}
	if !found {
		t.Fatalf("got %+v, want an EvBodyHit", ev)
	}

	// n = (1,0,0), vn = -5, BodyRestitution = 0.3:
	// Vel' = Vel - n*vn*(1+rest) = -5 - (1*-5*1.3) = 1.5
	assertFloat(t, "ball.Vel.X after bouncing off the capsule", s.Ball.Vel.X, 1.5)

	dist := s.Ball.Pos.Sub(Vec3{X: 0, Y: 1}).Len()
	assertFloat(t, "post-collision distance from the capsule", dist, PlayerRadius+BallRadius)
}

// TestBodyHit_SeparatingContactIsIgnored: if the ball is already moving
// away from the player along the contact normal, there is nothing to
// resolve (avoids re-triggering on a depenetrated contact next tick).
func TestBodyHit_SeparatingContactIsIgnored(t *testing.T) {
	s := newBodyTestState()
	s.Ball.Pos = Vec3{X: PlayerRadius + BallRadius - 0.01, Y: 1}
	s.Ball.Vel = Vec3{X: 5} // moving away from the player, not into it

	ev := applyBodyCollisions(&s, nil)

	if len(ev) != 0 {
		t.Fatalf("got %+v, want no EvBodyHit for a separating contact", ev)
	}
}

// TestBodyHit_GraceSkipsStriker is test plan #31b: the player who just
// struck the ball is immune to their own body for graceTicks.
func TestBodyHit_GraceSkipsStriker(t *testing.T) {
	s := newBodyTestState()
	s.Tick = 5
	s.Players[0].StruckAt = 4 // 1 tick ago, well within the default grace window
	s.Ball.Pos = Vec3{X: PlayerRadius + BallRadius - 0.01, Y: 1}
	s.Ball.Vel = Vec3{X: -5}

	ev := applyBodyCollisions(&s, nil)

	if len(ev) != 0 {
		t.Fatalf("got %+v, want no EvBodyHit while the striker's grace is active", ev)
	}
	if s.Ball.Vel.X != -5 {
		t.Errorf("ball velocity should be untouched during grace: got %v", s.Ball.Vel.X)
	}
}

// TestBodyHit_GraceExpiresAfterGraceTicks is the complement of the previous
// test: once enough ticks have passed, the same player's body collides
// normally again.
func TestBodyHit_GraceExpiresAfterGraceTicks(t *testing.T) {
	cfg := DefaultConfig()
	s := newBodyTestState()
	s.Tick = uint32(cfg.graceTicks) + 10
	s.Players[0].StruckAt = 1 // long enough ago for grace to have expired
	s.Ball.Pos = Vec3{X: PlayerRadius + BallRadius - 0.01, Y: 1}
	s.Ball.Vel = Vec3{X: -5}

	ev := applyBodyCollisions(&s, nil)

	found := false
	for _, e := range ev {
		if e.Kind == EvBodyHit {
			found = true
		}
	}
	if !found {
		t.Fatalf("got %+v, want an EvBodyHit once the grace window has passed", ev)
	}
}

// TestBodyHit_NoTunnelingAtHighSpeed is test plan's explicit "no tunneling
// at 22 m/s" requirement: a ball closing at 22 m/s, starting just outside
// the capsule, must still be detected and bounced on the tick it enters the
// capsule, not pass clean through to the player's far side.
func TestBodyHit_NoTunnelingAtHighSpeed(t *testing.T) {
	s := newBodyTestState()
	s.Cfg.Gravity = 0
	s.Cfg.AirDrag = 0

	const speed = 22.0
	minDist := PlayerRadius + BallRadius
	s.Ball.Pos = Vec3{X: minDist + speed*Dt/2, Y: 1}
	s.Ball.Vel = Vec3{X: -speed}

	stepBall(&s, nil)
	ev := applyBodyCollisions(&s, nil)

	found := false
	for _, e := range ev {
		if e.Kind == EvBodyHit {
			found = true
		}
	}
	if !found {
		t.Fatalf("ball tunneled through the capsule without a body hit: ball.Pos=%+v", s.Ball.Pos)
	}
	if s.Ball.Pos.X <= 0 {
		t.Errorf("ball ended up on/past the player's side (tunneled): Pos.X=%v", s.Ball.Pos.X)
	}
	if s.Ball.Vel.X <= 0 {
		t.Errorf("ball.Vel.X: got %v, want positive (bounced away)", s.Ball.Vel.X)
	}
}

// TestBodyHit_DegenerateNormalFallsBackToForward covers dist==0 (ball
// center exactly on the capsule's segment): the normal falls back to the
// player's forward direction instead of dividing by zero.
func TestBodyHit_DegenerateNormalFallsBackToForward(t *testing.T) {
	s := newBodyTestState()
	s.Players[0].Pos = Vec3{X: 2}
	s.Ball.Pos = Vec3{X: 2, Y: 1} // exactly on the capsule's axis
	s.Ball.Vel = Vec3{X: -1}      // moving toward team 0's back wall (into forward(0) = +1... )

	ev := applyBodyCollisions(&s, nil)

	found := false
	for _, e := range ev {
		if e.Kind == EvBodyHit {
			found = true
		}
	}
	if !found {
		t.Fatalf("got %+v, want an EvBodyHit even in the degenerate dist==0 case", ev)
	}
	dist := s.Ball.Pos.Sub(Vec3{X: 2, Y: 1}).Len()
	assertFloat(t, "post-collision distance (degenerate case)", dist, PlayerRadius+BallRadius)
}

// TestApplyBodyCollisions_OnlyActivePlayers: an overlapping but inactive
// player slot (index >= s.NumPlayers) must never participate.
func TestApplyBodyCollisions_OnlyActivePlayers(t *testing.T) {
	s := newBodyTestState() // NumPlayers: 2
	s.Players[2] = Player{Team: 0, Pos: Vec3{X: 0}}
	s.Ball.Pos = Vec3{X: PlayerRadius + BallRadius - 0.01, Y: 1}
	s.Ball.Vel = Vec3{X: -5}
	s.Players[0].Pos = Vec3{X: 100} // move the one *active* player out of range

	ev := applyBodyCollisions(&s, nil)

	if len(ev) != 0 {
		t.Fatalf("got %+v, want no collisions: inactive player2 must be ignored even though it overlaps the ball", ev)
	}
}

// TestClosestPointOnCapsule_ClampsToSegment checks the capsule's Y band
// independent of the collision resolution, so a failure here is easy to
// localize.
func TestClosestPointOnCapsule_ClampsToSegment(t *testing.T) {
	p := &Player{Pos: Vec3{X: 1, Y: 0, Z: 2}}
	cases := []struct {
		name  string
		ballY float64
		wantY float64
	}{
		{"below the capsule, clamps to PlayerRadius", -10, PlayerRadius},
		{"within the band, passes through", 1.0, 1.0},
		{"above the capsule, clamps to PlayerHeight-PlayerRadius", 10, PlayerHeight - PlayerRadius},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := closestPointOnCapsule(p, Vec3{X: 1, Y: c.ballY, Z: 2})
			assertFloat(t, "closest point Y", got.Y, c.wantY)
			assertFloat(t, "closest point X", got.X, 1)
			assertFloat(t, "closest point Z", got.Z, 2)
		})
	}
}

// TestBodyHit_PendingStrikeTakesTheBall: while the player has a strike
// buffered (waiting for its best contact tick), the incoming ball must not
// bounce off their body -- the strike resolves it. Otherwise the body touch
// plus the strike one tick later is judged a double touch (the "crazy
// referee" bug: ~70% of bot-vs-bot points ended that way).
func TestBodyHit_PendingStrikeTakesTheBall(t *testing.T) {
	s := newBodyTestState()
	s.Tick = 500
	s.Players[0].Buffer = 3
	s.Ball.Pos = Vec3{X: PlayerRadius + BallRadius - 0.01, Y: 1}
	s.Ball.Vel = Vec3{X: -5}

	ev := applyBodyCollisions(&s, nil)

	if len(ev) != 0 {
		t.Fatalf("got %+v, want no EvBodyHit while a strike is buffered", ev)
	}
	if s.Ball.Vel.X != -5 {
		t.Errorf("ball velocity should be untouched: got %v", s.Ball.Vel.X)
	}
}
