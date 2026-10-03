package sim

import (
	"math"
	"testing"
)

func newPhysicsTestState() State {
	s := State{Cfg: DefaultConfig()}
	s.Players[0] = Player{Side: 0}
	s.Players[1] = Player{Side: 1}
	s.Rally.LastKicker = -1
	return s
}

func TestMovePlayers(t *testing.T) {
	cfg := DefaultConfig()

	t.Run("moves at configured speed along a single axis", func(t *testing.T) {
		s := newPhysicsTestState()
		s.Players[0].Pos = Vec3{X: -5}
		movePlayers(&s, [2]PlayerInput{{MoveX: 1}, {}})

		wantVel := cfg.PlayerSpeed
		assertFloat(t, "player0.Vel.X", s.Players[0].Vel.X, wantVel)
		assertFloat(t, "player0.Pos.X", s.Players[0].Pos.X, -5+wantVel*Dt)
		assertFloat(t, "player0.Pos.Y", s.Players[0].Pos.Y, 0)
	})

	t.Run("diagonal input is normalized before scaling by speed", func(t *testing.T) {
		s := newPhysicsTestState()
		s.Players[0].Pos = Vec3{X: -5}
		movePlayers(&s, [2]PlayerInput{{MoveX: 1, MoveZ: 1}, {}})

		diag := 1 / math.Sqrt2
		wantVelX := diag * cfg.PlayerSpeed
		wantVelZ := diag * cfg.PlayerSpeed
		assertFloat(t, "player0.Vel.X", s.Players[0].Vel.X, wantVelX)
		assertFloat(t, "player0.Vel.Z", s.Players[0].Vel.Z, wantVelZ)
	})

	t.Run("side0 is clamped before crossing the net", func(t *testing.T) {
		s := newPhysicsTestState()
		s.Players[0].Pos = Vec3{X: -0.4}
		movePlayers(&s, [2]PlayerInput{{MoveX: 1}, {}})

		assertFloat(t, "player0.Pos.X", s.Players[0].Pos.X, -PlayerRadius)
	})

	t.Run("side1 is clamped before crossing the net", func(t *testing.T) {
		s := newPhysicsTestState()
		s.Players[1].Pos = Vec3{X: 0.4}
		movePlayers(&s, [2]PlayerInput{{}, {MoveX: -1}})

		assertFloat(t, "player1.Pos.X", s.Players[1].Pos.X, PlayerRadius)
	})

	t.Run("side0 is clamped at the back wall", func(t *testing.T) {
		s := newPhysicsTestState()
		s.Players[0].Pos = Vec3{X: -9.6}
		movePlayers(&s, [2]PlayerInput{{MoveX: -1}, {}})

		assertFloat(t, "player0.Pos.X", s.Players[0].Pos.X, -(CourtHalfLength - PlayerRadius))
	})

	t.Run("player is clamped at the side wall on Z", func(t *testing.T) {
		s := newPhysicsTestState()
		s.Players[0].Pos = Vec3{X: -5, Z: 4.9}
		movePlayers(&s, [2]PlayerInput{{MoveZ: 1}, {}})

		assertFloat(t, "player0.Pos.Z", s.Players[0].Pos.Z, CourtHalfWidth-PlayerRadius)
	})
}

func TestApplyKicks(t *testing.T) {
	t.Run("non-server cannot kick during serve", func(t *testing.T) {
		s := newPhysicsTestState()
		s.Phase = PhaseServe
		s.Server = 0
		s.Players[1].Pos = Vec3{X: 0.5}
		s.Ball.Pos = Vec3{X: 0.5}

		ev := applyKicks(&s, [2]PlayerInput{{}, {Kick: true, AimX: 1}}, nil)

		if len(ev) != 0 {
			t.Fatalf("got %d events, want 0", len(ev))
		}
		if s.Phase != PhaseServe {
			t.Errorf("Phase: got %v, want PhaseServe", s.Phase)
		}
	})

	t.Run("server kick during serve switches to rally and emits EvKick", func(t *testing.T) {
		s := newPhysicsTestState()
		s.Phase = PhaseServe
		s.Server = 0
		s.Players[0].Pos = Vec3{X: -0.5}
		s.Ball.Pos = Vec3{X: -0.5}

		ev := applyKicks(&s, [2]PlayerInput{{Kick: true, AimX: 1, Power: 1}, {}}, nil)

		if s.Phase != PhaseRally {
			t.Fatalf("Phase: got %v, want PhaseRally", s.Phase)
		}
		if len(ev) != 1 || ev[0].Kind != EvKick || ev[0].Side != 0 {
			t.Fatalf("got %+v, want one EvKick with Side 0", ev)
		}
		if s.Ball.Vel.Len() == 0 {
			t.Error("ball velocity was not set by the kick")
		}
	})

	t.Run("out of reach does not kick", func(t *testing.T) {
		s := newPhysicsTestState()
		s.Phase = PhaseRally
		s.Rally.LastKicker = 1
		s.Players[0].Pos = Vec3{X: -5}
		s.Ball.Pos = Vec3{X: 0}

		ev := applyKicks(&s, [2]PlayerInput{{Kick: true, AimX: 1}, {}}, nil)

		if len(ev) != 0 {
			t.Fatalf("got %d events, want 0", len(ev))
		}
	})

	t.Run("either side may kick during rally", func(t *testing.T) {
		s := newPhysicsTestState()
		s.Phase = PhaseRally
		s.Players[1].Pos = Vec3{X: 0.5}
		s.Ball.Pos = Vec3{X: 0.5}

		ev := applyKicks(&s, [2]PlayerInput{{}, {Kick: true, AimX: -1, Power: 0.5}}, nil)

		if len(ev) != 1 || ev[0].Kind != EvKick || ev[0].Side != 1 {
			t.Fatalf("got %+v, want one EvKick with Side 1", ev)
		}
	})

	t.Run("simultaneous kicks: side 0 wins the tie-break", func(t *testing.T) {
		s := newPhysicsTestState()
		s.Phase = PhaseRally
		s.Players[0].Pos = Vec3{X: -0.35}
		s.Players[1].Pos = Vec3{X: 0.35}
		s.Ball.Pos = Vec3{X: 0}

		ev := applyKicks(&s, [2]PlayerInput{{Kick: true, AimX: 1}, {Kick: true, AimX: -1}}, nil)

		if len(ev) != 1 {
			t.Fatalf("got %d events, want exactly 1 (only one kick per tick)", len(ev))
		}
		if ev[0].Side != 0 {
			t.Errorf("Side: got %d, want 0", ev[0].Side)
		}
	})
}

func TestStepBallGravityAndFloor(t *testing.T) {
	cfg := DefaultConfig()

	t.Run("gravity integrates velocity and position when airborne", func(t *testing.T) {
		s := newPhysicsTestState()
		s.Cfg = cfg
		s.Ball = Ball{Pos: Vec3{Y: 1}}

		stepBall(&s, nil)

		wantVelY := -cfg.Gravity * Dt * (1 - cfg.AirDrag*Dt)
		assertFloat(t, "ball.Vel.Y", s.Ball.Vel.Y, wantVelY)
		assertFloat(t, "ball.Pos.Y", s.Ball.Pos.Y, 1+wantVelY*Dt)
	})

	t.Run("fast impact bounces with restitution and friction", func(t *testing.T) {
		s := newPhysicsTestState()
		s.Cfg = cfg
		s.Ball = Ball{Pos: Vec3{Y: 0.15}, Vel: Vec3{X: 2, Y: -10}}

		preVelY := (-10 - cfg.Gravity*Dt) * (1 - cfg.AirDrag*Dt)
		prePosY := 0.15 + preVelY*Dt
		if prePosY >= BallRadius {
			t.Fatalf("test setup invalid: ball does not cross the floor this tick (prePosY=%v)", prePosY)
		}
		incoming := -preVelY

		ev := stepBall(&s, nil)

		assertFloat(t, "ball.Pos.Y", s.Ball.Pos.Y, BallRadius)
		assertFloat(t, "ball.Vel.Y", s.Ball.Vel.Y, incoming*cfg.FloorRestitution)
		if len(ev) != 1 || ev[0].Kind != EvBounceFloor {
			t.Fatalf("got %+v, want one EvBounceFloor", ev)
		}
	})

	t.Run("slow impact rests and emits one edge-triggered EvBounceFloor", func(t *testing.T) {
		s := newPhysicsTestState()
		s.Cfg = cfg
		s.Ball = Ball{Pos: Vec3{Y: BallRadius + 0.0001}, Vel: Vec3{Y: -0.05}}

		ev := stepBall(&s, nil)

		assertFloat(t, "ball.Vel.Y", s.Ball.Vel.Y, 0)
		if !s.Ball.Resting {
			t.Error("ball.Resting: got false, want true after settling on the floor")
		}
		if len(ev) != 1 || ev[0].Kind != EvBounceFloor {
			t.Fatalf("got %+v, want exactly one EvBounceFloor (resting edge-trigger)", ev)
		}
	})

	t.Run("staying at rest does not re-emit EvBounceFloor every tick", func(t *testing.T) {
		s := newPhysicsTestState()
		s.Cfg = cfg
		s.Ball = Ball{Pos: Vec3{Y: BallRadius + 0.0001}, Vel: Vec3{Y: -0.05}}

		stepBall(&s, nil) // first tick: settles and emits once
		ev := stepBall(&s, nil)

		if len(ev) != 0 {
			t.Fatalf("got %d events, want 0 (already resting, no new episode)", len(ev))
		}
	})

	t.Run("resting flag clears once the ball is airborne again", func(t *testing.T) {
		s := newPhysicsTestState()
		s.Cfg = cfg
		s.Ball = Ball{Pos: Vec3{Y: 1}, Vel: Vec3{Y: 1}, Resting: true}

		stepBall(&s, nil)

		if s.Ball.Resting {
			t.Error("ball.Resting: got true, want false once the ball is airborne (kicked away)")
		}
	})
}

func TestStepBallWalls(t *testing.T) {
	cfg := DefaultConfig()

	t.Run("reflects off the far wall when inside the court", func(t *testing.T) {
		s := newPhysicsTestState()
		s.Cfg = cfg
		s.Ball = Ball{Pos: Vec3{X: CourtHalfLength - BallRadius - 0.01, Y: 0.1}, Vel: Vec3{X: 20}}

		ev := stepBall(&s, nil)

		assertFloat(t, "ball.Pos.X", s.Ball.Pos.X, CourtHalfLength-BallRadius)
		if s.Ball.Vel.X >= 0 {
			t.Errorf("ball.Vel.X: got %v, want negative (reflected)", s.Ball.Vel.X)
		}
		foundWall := false
		for _, e := range ev {
			if e.Kind == EvHitWall {
				foundWall = true
				if e.Side != 1 {
					t.Errorf("EvHitWall.Side: got %d, want 1", e.Side)
				}
			}
		}
		if !foundWall {
			t.Fatalf("got %+v, want an EvHitWall", ev)
		}
	})

	t.Run("flying above wall height skips reflection and goes out", func(t *testing.T) {
		s := newPhysicsTestState()
		s.Cfg = cfg
		s.Ball = Ball{Pos: Vec3{X: 9.5, Y: WallHeight + 0.5}, Vel: Vec3{X: 50}}

		ev := stepBall(&s, nil)

		for _, e := range ev {
			if e.Kind == EvHitWall {
				t.Errorf("got EvHitWall, want none (ball was above WallHeight)")
			}
		}
		if len(ev) != 1 || ev[0].Kind != EvOut {
			t.Fatalf("got %+v, want exactly one EvOut", ev)
		}
	})

	t.Run("EvOut is edge-triggered: only the first tick out of bounds emits it", func(t *testing.T) {
		s := newPhysicsTestState()
		s.Cfg = cfg
		s.Ball = Ball{Pos: Vec3{X: 9.5, Y: WallHeight + 0.5}, Vel: Vec3{X: 50}}

		first := stepBall(&s, nil)
		second := stepBall(&s, nil)

		outCount := func(ev []Event) int {
			n := 0
			for _, e := range ev {
				if e.Kind == EvOut {
					n++
				}
			}
			return n
		}
		if outCount(first) != 1 {
			t.Fatalf("first tick out: got %d EvOut, want 1", outCount(first))
		}
		if outCount(second) != 0 {
			t.Fatalf("second tick still out: got %d EvOut, want 0 (same episode)", outCount(second))
		}
		if !s.Ball.Out {
			t.Error("ball.Out: got false, want true while still outside the court")
		}
	})

	t.Run("Out flag clears once the ball is back inside the court", func(t *testing.T) {
		s := newPhysicsTestState()
		s.Cfg = cfg
		s.Ball = Ball{Pos: Vec3{X: 5, Y: 0.1}, Vel: Vec3{}, Out: true}

		stepBall(&s, nil)

		if s.Ball.Out {
			t.Error("ball.Out: got true, want false once the ball is back inside the court bounds")
		}
	})

	t.Run("own-side wall hit is legal, no event", func(t *testing.T) {
		s := newPhysicsTestState()
		s.Cfg = cfg
		s.Ball = Ball{Pos: Vec3{X: -(CourtHalfLength - BallRadius - 0.01), Y: 0.1}, Vel: Vec3{X: -20}}

		ev := stepBall(&s, nil)

		found := false
		for _, e := range ev {
			if e.Kind == EvHitWall {
				found = true
			}
		}
		if !found {
			t.Fatalf("got %+v, want an EvHitWall (walls bounce regardless of rules)", ev)
		}
	})
}

func TestStepBallNet(t *testing.T) {
	t.Run("low crossing clips the net", func(t *testing.T) {
		s := newPhysicsTestState()
		s.Cfg = DefaultConfig()
		s.Ball = Ball{Pos: Vec3{X: -0.05, Y: 0.2}, Vel: Vec3{X: 10}}

		ev := stepBall(&s, nil)

		if len(ev) == 0 || ev[0].Kind != EvHitNet {
			t.Fatalf("got %+v, want EvHitNet", ev)
		}
		if ev[0].Side != 0 {
			t.Errorf("EvHitNet.Side: got %d, want 0", ev[0].Side)
		}
		assertFloat(t, "ball.Pos.X", s.Ball.Pos.X, -BallRadius)
		if s.Ball.Vel.X >= 0 {
			t.Errorf("ball.Vel.X: got %v, want negative (bounced back toward side 0)", s.Ball.Vel.X)
		}
	})

	t.Run("high crossing clears the net without an event", func(t *testing.T) {
		s := newPhysicsTestState()
		s.Cfg = DefaultConfig()
		s.Ball = Ball{Pos: Vec3{X: -0.05, Y: NetHeight + 1}, Vel: Vec3{X: 10}}

		ev := stepBall(&s, nil)

		for _, e := range ev {
			if e.Kind == EvHitNet {
				t.Errorf("got EvHitNet, want the ball to clear the net")
			}
		}
	})
}

func TestClamp(t *testing.T) {
	cases := []struct {
		name      string
		v, lo, hi float64
		want      float64
	}{
		{"within range", 0.5, 0, 1, 0.5},
		{"below range", -1, 0, 1, 0},
		{"above range", 2, 0, 1, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := clamp(c.v, c.lo, c.hi)
			assertFloat(t, "clamp", got, c.want)
		})
	}
}

func TestSideOf(t *testing.T) {
	cases := []struct {
		name string
		x    float64
		want int8
	}{
		{"negative", -1, 0},
		{"positive", 1, 1},
		{"zero counts as side 1", 0, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := sideOf(c.x); got != c.want {
				t.Errorf("sideOf(%v): got %d, want %d", c.x, got, c.want)
			}
		})
	}
}
