package sim

import (
	"math"
	"testing"
)

// passState is a 2v2 rally where team 1 just shot and team 0's player 0 is
// about to touch the ball with the Lob button latched.
func passState() State {
	s := State{Cfg: DefaultConfig(), NumPlayers: 4}
	resetForServe(&s, 1)
	s.Phase = PhaseRally
	s.Rally.LastTeam, s.Rally.LastPlayer, s.Rally.TeamTouches = 1, 2, 1
	s.Players[0].Pos = Vec3{X: -6, Z: -2}
	s.Players[1].Pos = Vec3{X: -3, Z: 2.5}
	s.Ball.Pos = Vec3{X: -6, Y: 0.4, Z: -1.6}
	s.Ball.Tossed = true
	p := &s.Players[0]
	p.ShotType = ShotLob
	p.BufTarget = Vec3{X: 8, Y: BallRadius, Z: 3} // aiming at the rival court
	p.BufCharge = 1
	return s
}

func horizontalDir(v Vec3) Vec3 { return Vec3{X: v.X, Z: v.Z}.Normalized() }

func TestStrike_LobWithPassReadySetsUpThePartner(t *testing.T) {
	s := passState()
	if !PassReady(&s, 0) {
		t.Fatal("first touch of team 0 in a 2v2 rally must be pass-ready")
	}
	strike(&s, 0, PartFoot, 0, nil)
	want := horizontalDir(Vec3{X: s.Players[1].Pos.X - s.Ball.Pos.X, Z: s.Players[1].Pos.Z - s.Ball.Pos.Z})
	if got := horizontalDir(s.Ball.Vel); got.Dot(want) < 0.99 {
		t.Fatalf("pass heads %+v, want toward the partner %+v", got, want)
	}
	if s.Ball.Vel.Y <= 0 {
		t.Fatalf("a pass is a soft set upward, got Vel.Y=%.2f", s.Ball.Vel.Y)
	}
	land := landingOf(&s)
	if d := math.Hypot(land.X-s.Players[1].Pos.X, land.Z-s.Players[1].Pos.Z); d > 0.6 {
		t.Fatalf("the set comes down %.2f m from the partner (at %+v), want within 0.6 m", d, land)
	}
}

func TestStrike_LobWithoutPassGoesToTheRivalCourt(t *testing.T) {
	cases := []struct {
		name  string
		setup func(*State)
	}{
		{"1v1", func(s *State) { s.Rally.TouchLimit = 1 }},
		{"last touch left must cross", func(s *State) {
			s.Rally.LastTeam, s.Rally.LastPlayer, s.Rally.TeamTouches = 0, 1, 2
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := passState()
			c.setup(&s)
			if PassReady(&s, 0) {
				t.Fatal("PassReady must be false here")
			}
			strike(&s, 0, PartFoot, 0, nil)
			if s.Ball.Vel.X <= 0 {
				t.Fatalf("a normal lob crosses the net (+X), got Vel=%+v", s.Ball.Vel)
			}
		})
	}
}

func TestPassReady(t *testing.T) {
	cases := []struct {
		name                 string
		phase                Phase
		lastTeam, lastPlayer int8
		touches              uint8
		boxPending           bool
		player               int8
		want                 bool
	}{
		{"receiving team, first touch", PhaseRally, 1, 2, 1, false, 0, true},
		{"partner after one pass", PhaseRally, 0, 0, 1, false, 1, true},
		{"partner after two touches: must cross", PhaseRally, 0, 0, 2, false, 1, false},
		{"same player twice", PhaseRally, 0, 0, 1, false, 0, false},
		{"serve still in flight", PhaseRally, 1, 2, 1, true, 0, false},
		{"serve phase", PhaseServe, -1, -1, 0, false, 0, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := passState()
			s.Phase = c.phase
			s.Rally.LastTeam, s.Rally.LastPlayer, s.Rally.TeamTouches = c.lastTeam, c.lastPlayer, c.touches
			s.Rally.ServeBoxPending = c.boxPending
			if got := PassReady(&s, c.player); got != c.want {
				t.Fatalf("PassReady=%v, want %v", got, c.want)
			}
		})
	}
}

// landingOf integrates the ball (no events) until it comes back down to
// BallRadius + 1 m (the partner's chest height the set aims at).
func landingOf(s *State) Vec3 {
	b := s.Ball
	for i := 0; i < 4*TickRate; i++ {
		prevY := b.Pos.Y
		b = PredictBall(b, &s.Cfg, 1)
		if b.Vel.Y < 0 && prevY >= passHeight && b.Pos.Y < passHeight {
			return b.Pos
		}
	}
	return b.Pos
}

func TestSnapshot_PassReadyFlag(t *testing.T) {
	s := passState()
	dst := make([]float64, SnapLen)
	WriteSnapshot(&s, dst)
	if flags := int(dst[SnapP0Flags]); flags&FlagPassReady == 0 {
		t.Fatalf("P0 flags=%b, want the pass-ready bit", flags)
	}
	if flags := int(dst[SnapP2Flags]); flags&FlagPassReady != 0 {
		t.Fatalf("P2 (team in possession, last toucher) flags=%b, want no pass-ready bit", flags)
	}
}
