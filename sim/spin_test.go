package sim

import "testing"

// --- floor bounce spin model (resolveFloor, §10) ----------------------------

func TestFloorBounce_TopspinKeepsMoreHorizontal_LowerBounce(t *testing.T) {
	cfg := DefaultConfig()

	flat := newStrikeTestState()
	flat.Cfg = cfg
	flat.Ball = Ball{Pos: Vec3{Y: 0.15}, Vel: Vec3{X: 5, Y: -10}, Spin: 0}
	stepBall(&flat, nil)

	top := newStrikeTestState()
	top.Cfg = cfg
	top.Ball = Ball{Pos: Vec3{Y: 0.15}, Vel: Vec3{X: 5, Y: -10}, Spin: 1}
	stepBall(&top, nil)

	if top.Ball.Vel.X <= flat.Ball.Vel.X {
		t.Errorf("topspin horizontal speed after bounce: got %v, want > flat's %v", top.Ball.Vel.X, flat.Ball.Vel.X)
	}
	if top.Ball.Vel.Y >= flat.Ball.Vel.Y {
		t.Errorf("topspin bounce height (Vel.Y): got %v, want lower than flat's %v (lower bounce)", top.Ball.Vel.Y, flat.Ball.Vel.Y)
	}
}

func TestFloorBounce_BackspinReducesHorizontalSpeed(t *testing.T) {
	cfg := DefaultConfig()

	flat := newStrikeTestState()
	flat.Cfg = cfg
	flat.Ball = Ball{Pos: Vec3{Y: 0.15}, Vel: Vec3{X: 5, Y: -10}, Spin: 0}
	stepBall(&flat, nil)

	back := newStrikeTestState()
	back.Cfg = cfg
	back.Ball = Ball{Pos: Vec3{Y: 0.15}, Vel: Vec3{X: 5, Y: -10}, Spin: -1}
	stepBall(&back, nil)

	if back.Ball.Vel.X >= flat.Ball.Vel.X {
		t.Errorf("backspin horizontal speed after bounce: got %v, want < flat's %v", back.Ball.Vel.X, flat.Ball.Vel.X)
	}
}

func TestSpin_DecaysPerFloorBounce(t *testing.T) {
	cfg := DefaultConfig()
	s := newStrikeTestState()
	s.Cfg = cfg
	s.Ball = Ball{Pos: Vec3{Y: 0.15}, Vel: Vec3{X: 5, Y: -10}, Spin: 1}

	stepBall(&s, nil)

	assertFloat(t, "Ball.Spin after one floor bounce", s.Ball.Spin, cfg.SpinDecay)
}

// --- wall bounce spin model (reflectWalls, §10) -----------------------------

func TestWall_BackspinDiesOnGlass(t *testing.T) {
	cfg := DefaultConfig()

	flat := newStrikeTestState()
	flat.Cfg = cfg
	flat.Ball = Ball{Pos: Vec3{X: CourtHalfLength - BallRadius - 0.01, Y: 0.1}, Vel: Vec3{X: 20}, Spin: 0}
	stepBall(&flat, nil)

	back := newStrikeTestState()
	back.Cfg = cfg
	back.Ball = Ball{Pos: Vec3{X: CourtHalfLength - BallRadius - 0.01, Y: 0.1}, Vel: Vec3{X: 20}, Spin: -1}
	stepBall(&back, nil)

	if -back.Ball.Vel.X >= -flat.Ball.Vel.X {
		t.Errorf("backspin bounce-back speed: got %v, want weaker (less negative) than flat's %v", back.Ball.Vel.X, flat.Ball.Vel.X)
	}
	if back.Ball.Vel.Y >= flat.Ball.Vel.Y {
		t.Errorf("backspin should push Vel.Y down on a wall hit: got %v, want < flat's %v", back.Ball.Vel.Y, flat.Ball.Vel.Y)
	}
}

func TestSpin_DecaysPerWallBounce(t *testing.T) {
	cfg := DefaultConfig()
	s := newStrikeTestState()
	s.Cfg = cfg
	s.Ball = Ball{Pos: Vec3{X: CourtHalfLength - BallRadius - 0.01, Y: 0.1}, Vel: Vec3{X: 20}, Spin: 1}

	stepBall(&s, nil)

	assertFloat(t, "Ball.Spin after one wall bounce", s.Ball.Spin, cfg.SpinDecay)
}

func TestWall_TopspinUnaffectedByBackspinDrop(t *testing.T) {
	cfg := DefaultConfig()

	flat := newStrikeTestState()
	flat.Cfg = cfg
	flat.Ball = Ball{Pos: Vec3{X: CourtHalfLength - BallRadius - 0.01, Y: 0.1}, Vel: Vec3{X: 20, Y: 1}, Spin: 0}
	stepBall(&flat, nil)

	top := newStrikeTestState()
	top.Cfg = cfg
	top.Ball = Ball{Pos: Vec3{X: CourtHalfLength - BallRadius - 0.01, Y: 0.1}, Vel: Vec3{X: 20, Y: 1}, Spin: 1}
	stepBall(&top, nil)

	// Topspin only affects the restitution's sign-gated term (clamped to
	// min(s,0) == 0 for s>0) and never SpinWallDrop (backspin-only): Vel.Y
	// should end up exactly where gravity+drag alone would have put it.
	assertFloat(t, "Ball.Vel.Y with topspin vs. no spin", top.Ball.Vel.Y, flat.Ball.Vel.Y)
}
