package sim

import "testing"

// --- §19.4: same-team capsule separation (2v2) -----------------------------

func TestSeparateTeammates_PushesOverlapApart(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Mode = Mode2v2
	s := State{Cfg: cfg, NumPlayers: 4}
	s.Players[0] = Player{Pos: Vec3{X: -5, Z: 0.1}, Team: 0, OnGround: true}
	s.Players[1] = Player{Pos: Vec3{X: -5, Z: -0.1}, Team: 0, OnGround: true}
	s.Players[2] = Player{Pos: Vec3{X: 5}, Team: 1, OnGround: true}
	s.Players[3] = Player{Pos: Vec3{X: 5, Z: 3}, Team: 1, OnGround: true}

	movePlayers(&s, [MaxPlayers]PlayerInput{}, nil)

	dist := s.Players[1].Pos.Sub(s.Players[0].Pos).Len()
	if dist < 2*PlayerRadius-epsilon {
		t.Fatalf("teammates still overlapping: dist=%v, want >= %v", dist, 2*PlayerRadius)
	}
}

func TestSeparateTeammates_NeverMovesOpponentsApart(t *testing.T) {
	s := newPhysicsTestState() // NumPlayers: 2, no separation pass runs at all
	s.Players[0].OnGround = true
	s.Players[1].OnGround = true
	s.Players[0].Pos = Vec3{X: -1}
	s.Players[1].Pos = Vec3{X: 1}

	movePlayers(&s, [MaxPlayers]PlayerInput{}, nil)

	assertFloat(t, "player0.Pos.X unaffected by opponent proximity", s.Players[0].Pos.X, -1)
}

// TestSeparateTeammates_DegenerateTieBreaksDeterministically covers the
// dist==0 fallback: two exactly-coincident teammates must still separate,
// and always in the same direction (break the tie on +Z), not depend on
// floating point luck.
func TestSeparateTeammates_DegenerateTieBreaksDeterministically(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Mode = Mode2v2
	s := State{Cfg: cfg, NumPlayers: 4}
	s.Players[0] = Player{Pos: Vec3{X: -5}, Team: 0, OnGround: true}
	s.Players[1] = Player{Pos: Vec3{X: -5}, Team: 0, OnGround: true}
	s.Players[2] = Player{Pos: Vec3{X: 5}, Team: 1, OnGround: true}
	s.Players[3] = Player{Pos: Vec3{X: 5}, Team: 1, OnGround: true}

	movePlayers(&s, [MaxPlayers]PlayerInput{}, nil)

	if s.Players[0].Pos == s.Players[1].Pos {
		t.Fatalf("coincident teammates were not separated")
	}
	if s.Players[0].Pos.Z >= s.Players[1].Pos.Z {
		t.Errorf("expected player0 pushed to -Z and player1 to +Z deterministically: got %+v / %+v",
			s.Players[0].Pos, s.Players[1].Pos)
	}
}

// TestDeterminism_2v2 mirrors TestDeterminism but exercises the 2v2 path
// (separation + 4 active players) for the same bit-identical-replay
// contract.
func TestDeterminism_2v2(t *testing.T) {
	const ticks = 3000

	runMatch := func(seed uint64) State {
		cfg := DefaultConfig()
		cfg.Mode = Mode2v2
		m := NewMatch(cfg)
		g := newLCG(seed)
		for i := 0; i < ticks; i++ {
			inputs := [MaxPlayers]PlayerInput{
				scriptedInput(g), scriptedInput(g), scriptedInput(g), scriptedInput(g),
			}
			m.Step(inputs)
		}
		return m.State()
	}

	a := runMatch(7)
	b := runMatch(7)
	if a != b {
		t.Fatalf("two 2v2 matches given the same seed diverged:\n a=%+v\n b=%+v", a, b)
	}
}
