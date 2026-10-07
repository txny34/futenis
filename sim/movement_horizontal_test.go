package sim

import "testing"

// --- movement.go: horizontal acceleration / court clamp / inactive slots ---

func TestMovePlayers_DiagonalInputIsNormalized(t *testing.T) {
	s := newPhysicsTestState()
	s.Players[0].OnGround = true
	s.Players[0].Pos = Vec3{X: -5}
	movePlayers(&s, fullInputs(PlayerInput{MoveX: 1, MoveZ: 1}, PlayerInput{}), nil)

	assertFloat(t, "player0.Vel.X == player0.Vel.Z", s.Players[0].Vel.X, s.Players[0].Vel.Z)
	if s.Players[0].Vel.X <= 0 {
		t.Errorf("expected forward progress on the diagonal, got %v", s.Players[0].Vel.X)
	}
}

func TestMovePlayers_Side0ClampedBeforeCrossingNet(t *testing.T) {
	s := newPhysicsTestState()
	s.Players[0].OnGround = true
	s.Players[0].Pos = Vec3{X: -0.4}
	for i := 0; i < 2*TickRate; i++ {
		movePlayers(&s, fullInputs(PlayerInput{MoveX: 1}, PlayerInput{}), nil)
	}
	assertFloat(t, "player0.Pos.X", s.Players[0].Pos.X, -PlayerRadius)
}

func TestMovePlayers_Side1ClampedBeforeCrossingNet(t *testing.T) {
	s := newPhysicsTestState()
	s.Players[1].OnGround = true
	s.Players[1].Pos = Vec3{X: 0.4}
	for i := 0; i < 2*TickRate; i++ {
		movePlayers(&s, fullInputs(PlayerInput{}, PlayerInput{MoveX: -1}), nil)
	}
	assertFloat(t, "player1.Pos.X", s.Players[1].Pos.X, PlayerRadius)
}

func TestMovePlayers_Side0ClampedAtBackWall(t *testing.T) {
	s := newPhysicsTestState()
	s.Players[0].OnGround = true
	s.Players[0].Pos = Vec3{X: -9.6}
	for i := 0; i < 2*TickRate; i++ {
		movePlayers(&s, fullInputs(PlayerInput{MoveX: -1}, PlayerInput{}), nil)
	}
	assertFloat(t, "player0.Pos.X", s.Players[0].Pos.X, -(CourtHalfLength - PlayerRadius))
}

func TestMovePlayers_ClampedAtSideWallOnZ(t *testing.T) {
	s := newPhysicsTestState()
	s.Players[0].OnGround = true
	s.Players[0].Pos = Vec3{X: -5, Z: 4.9}
	for i := 0; i < 2*TickRate; i++ {
		movePlayers(&s, fullInputs(PlayerInput{MoveZ: 1}, PlayerInput{}), nil)
	}
	assertFloat(t, "player0.Pos.Z", s.Players[0].Pos.Z, CourtHalfWidth-PlayerRadius)
}

func TestMovePlayers_OnlyActiveSlotsMove(t *testing.T) {
	s := newPhysicsTestState() // NumPlayers: 2
	s.Players[2].Pos = Vec3{X: -5}
	movePlayers(&s, [MaxPlayers]PlayerInput{{}, {}, {MoveX: 1}, {MoveX: 1}}, nil)

	if s.Players[2].Pos.X != -5 {
		t.Errorf("player2 (inactive slot) moved: got %+v", s.Players[2].Pos)
	}
}

func TestMovePlayers_ZeroAllocs(t *testing.T) {
	s := newPhysicsTestState()
	s.Players[0].OnGround = true
	s.Players[1].OnGround = true
	in := [MaxPlayers]PlayerInput{{MoveX: 0.5}, {MoveX: -0.5}}
	var ev []Event

	allocs := testing.AllocsPerRun(100, func() {
		ev = movePlayers(&s, in, ev[:0])
	})
	if allocs != 0 {
		t.Errorf("movePlayers allocated %v times per call, want 0", allocs)
	}
}
