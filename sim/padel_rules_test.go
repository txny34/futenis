package sim

import "testing"

// Real-padel rule fixes (docs/design/contact-v2.md §7.5, §19.5, §21):
// side convention, diagonal serve, partner-hit fault, tiebreak scoring,
// tiebreak serve rotation and the 4-player 2v2 serve rotation.

func newDoublesPadelState(server int8) State {
	s := newPadelTestState()
	s.NumPlayers = 4
	s.Server = server
	return s
}

func countEv(ev []Event, k EventKind) int {
	n := 0
	for _, e := range ev {
		if e.Kind == k {
			n++
		}
	}
	return n
}

func TestRightSign(t *testing.T) {
	if rightSign(0) != 1 || rightSign(1) != -1 {
		t.Fatalf("rightSign: got (%v,%v), want (1,-1)", rightSign(0), rightSign(1))
	}
}

// Fix 1: slot 0 of each team starts on its OWN right (team 1 faces -X,
// so its right is -Z), slot 1 on its left -- for the receiving team.
func TestSetupDoublesPositions_SlotZeroOnOwnRight(t *testing.T) {
	s := newDoublesPadelState(0)
	resetForServe(&s, 0)
	if z := s.Players[2].Pos.Z; z != -servePlayerZOffset {
		t.Errorf("team1 slot0 Z: got %v, want %v (team 1's right is -Z)", z, -servePlayerZOffset)
	}
	if z := s.Players[3].Pos.Z; z != servePlayerZOffset {
		t.Errorf("team1 slot1 Z: got %v, want %v", z, servePlayerZOffset)
	}
	if z := s.Players[0].Pos.Z; z != servePlayerZOffset {
		t.Errorf("team0 slot0 Z: got %v, want %v", z, servePlayerZOffset)
	}
}

// Fix 1b: a serve that hits the server's own partner is a serve fault
// (first fault replays with a second serve), not a lost point.
func TestJudgeServeVolley_PartnerHitIsServeFault(t *testing.T) {
	s := newDoublesPadelState(0)
	resetForServe(&s, 0)
	s.Phase = PhaseRally
	sp := s.Rally.ServerPlayer
	partner := 1 - sp

	ev := judgeRally(&s, []Event{{Kind: EvKick, Side: 0, Player: sp}, {Kind: EvBodyHit, Side: 0, Player: partner}})

	if countEv(ev, EvServeFault) != 1 || countEv(ev, EvPoint) != 0 {
		t.Fatalf("got %+v, want exactly one EvServeFault and no EvPoint", ev)
	}
	if s.Rally.ServeAttempt != 2 || s.Phase != PhaseServe {
		t.Fatalf("got attempt %d phase %v, want second serve", s.Rally.ServeAttempt, s.Phase)
	}
	// Second serve hitting the partner again is a double fault.
	s.Phase = PhaseRally
	ev = judgeRally(&s, []Event{{Kind: EvKick, Side: 0, Player: sp}, {Kind: EvBodyHit, Side: 0, Player: partner}})
	if countEv(ev, EvPoint) != 1 || s.Score[1] != 1 {
		t.Fatalf("got %+v score %v, want the receiver to win on the double fault", ev, s.Score)
	}
}

// Fix 2: box 0 is the RECEIVER's right for both teams; box 1 its left.
func TestInServeBox_Diagonal(t *testing.T) {
	cfg := DefaultConfig()
	cases := []struct {
		name   string
		pos    Vec3
		server int8
		box    int8
		want   bool
	}{
		{"server0 box0 = team1's right (-Z)", Vec3{X: 3, Z: -2}, 0, 0, true},
		{"server0 box0 rejects +Z", Vec3{X: 3, Z: 2}, 0, 0, false},
		{"server0 box1 = team1's left (+Z)", Vec3{X: 3, Z: 2}, 0, 1, true},
		{"server1 box0 = team0's right (+Z)", Vec3{X: -3, Z: 2}, 1, 0, true},
		{"server1 box0 rejects -Z", Vec3{X: -3, Z: -2}, 1, 0, false},
		{"server1 box1 = team0's left (-Z)", Vec3{X: -3, Z: -2}, 1, 1, true},
	}
	for _, c := range cases {
		if got := inServeBox(c.pos, c.server, c.box, &cfg); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

// Fix 2b: the server stands on its own right on an even total (box 0) and
// on its left on an odd total, so the serve crosses diagonally.
func TestResetForServe_ServerSideMatchesBox(t *testing.T) {
	for _, server := range []int8{0, 1} {
		for total := 0; total < 2; total++ {
			s := newRulesTestState()
			s.Score = [2]int{total, 0}
			resetForServe(&s, server)
			want := rightSign(server) * servePlayerZOffset
			if total == 1 {
				want = -want
			}
			srv := s.Players[s.Rally.ServerPlayer]
			if srv.Pos.Z != want {
				t.Errorf("server %d total %d: Z got %v, want %v", server, total, srv.Pos.Z, want)
			}
			rcv := s.Players[1-server]
			if rcv.Pos.Z != -want {
				t.Errorf("server %d total %d: receiver Z got %v, want %v (diagonal)", server, total, rcv.Pos.Z, -want)
			}
			if ServeBoxZSign(server, s.Rally.ServeBox)*srv.Pos.Z >= 0 {
				t.Errorf("server %d total %d: box Z side must be opposite the server", server, total)
			}
		}
	}
}

func TestResetForServe_DoublesServerTakesBoxSide(t *testing.T) {
	s := newDoublesPadelState(1)
	s.Score = [2]int{1, 0}
	resetForServe(&s, 1)
	sp := s.Rally.ServerPlayer
	a, b := teamPlayers(1)
	partner := a
	if sp == a {
		partner = b
	}
	if z := s.Players[sp].Pos.Z; z != servePlayerZOffset {
		t.Errorf("team1 server on odd total: Z got %v, want %v (its left)", z, servePlayerZOffset)
	}
	if z := s.Players[partner].Pos.Z; z != -servePlayerZOffset {
		t.Errorf("team1 partner: Z got %v, want %v", z, -servePlayerZOffset)
	}
}

// Fix 3: tiebreak games are first to 7, win by 2.
func TestTiebreak_FirstToSevenWinByTwo(t *testing.T) {
	s := newPadelTestState()
	s.Games = [2]int{3, 3}
	s.Score = [2]int{5, 0}
	awardPoint(&s, 0, nil) // 6-0: not yet
	if s.Sets[0] != 0 {
		t.Fatalf("6-0 in a tiebreak must not win the set")
	}
	s.Score = [2]int{6, 6}
	awardPoint(&s, 0, nil) // 7-6
	if s.Sets[0] != 0 {
		t.Fatalf("7-6 in a tiebreak must not win the set (win by 2)")
	}
	awardPoint(&s, 0, nil) // 8-6
	if s.Sets[0] != 1 {
		t.Fatalf("8-6 must win the tiebreak/set, got sets %v", s.Sets)
	}
}

func TestTiebreak_GamePointWarningAtSix(t *testing.T) {
	s := newPadelTestState()
	s.Games = [2]int{3, 3}
	s.Server = 0
	s.Score = [2]int{2, 0}
	ev := awardPoint(&s, 0, nil) // 3-0: a normal game would warn here
	if countEv(ev, EvGamePoint)+countEv(ev, EvBreakPoint) != 0 {
		t.Fatalf("3-0 in a tiebreak must not warn: %+v", ev)
	}
	s.Score = [2]int{5, 0}
	ev = awardPoint(&s, 0, nil) // 6-0
	if countEv(ev, EvGamePoint)+countEv(ev, EvBreakPoint) != 1 {
		t.Fatalf("6-0 in a tiebreak must warn once: %+v", ev)
	}
}

// Fix 4: the team due to serve serves point 1, then serve alternates every
// 2 points; after the tiebreak the first receiver serves the next set.
func TestTiebreak_ServeRotation(t *testing.T) {
	s := newPadelTestState()
	s.Games = [2]int{3, 3}
	s.Server = 1
	want := []int8{0, 0, 1, 1, 0, 0} // server after points 1..6
	for i, w := range want {
		awardPoint(&s, int8(i%2), nil) // keep it level: 1-0,1-1,2-1,...
		if s.Server != w {
			t.Fatalf("after tiebreak point %d: Server got %d, want %d", i+1, s.Server, w)
		}
	}
	// Score 3-3; team 0 wins four straight -> 7-3, set over.
	for i := 0; i < 4; i++ {
		awardPoint(&s, 0, nil)
	}
	if s.Sets[0] != 1 {
		t.Fatalf("Sets: got %v, want team 0 to win the tiebreak", s.Sets)
	}
	if s.Server != 0 {
		t.Fatalf("next set first server: got %d, want 0 (received first in the tiebreak)", s.Server)
	}
}

func TestTiebreak_NextSetServerAfterOddLength(t *testing.T) {
	s := newPadelTestState()
	s.Cfg.SetsToWin = 2
	s.Games = [2]int{3, 3}
	s.Server = 0
	for i := 0; i < 7; i++ {
		awardPoint(&s, 1, nil)
	}
	if s.Sets[1] != 1 || s.Server != 1 {
		t.Fatalf("got sets %v server %d, want team 1 set and team 1 (first receiver) serving", s.Sets, s.Server)
	}
}

// Fix 5: one player serves a whole game; across games the order is
// A1, B1, A2, B2.
func TestServerRotation_2v2AcrossGames(t *testing.T) {
	s := newDoublesPadelState(0)
	s.LastServerPlayer = [2]int8{-1, -1}
	want := []int8{0, 2, 1, 3, 0}
	for g, w := range want {
		for p := 0; p < 4; p++ {
			resetForServe(&s, s.Server)
			if s.Rally.ServerPlayer != w {
				t.Fatalf("game %d point %d: ServerPlayer got %d, want %d", g, p, s.Rally.ServerPlayer, w)
			}
			awardPoint(&s, s.Server, nil)
		}
	}
}

func TestServerRotation_SimpleModeAlternatesOnServeChange(t *testing.T) {
	s := newRulesTestState()
	s.NumPlayers = 4
	resetForServe(&s, 0)
	resetForServe(&s, 0) // same team keeps serving: same player
	if s.Rally.ServerPlayer != 0 {
		t.Fatalf("got %d, want 0", s.Rally.ServerPlayer)
	}
	resetForServe(&s, 1)
	resetForServe(&s, 0) // team 0 regains serve: partner
	if s.Rally.ServerPlayer != 1 {
		t.Fatalf("got %d, want 1", s.Rally.ServerPlayer)
	}
}

func TestPadelRules_ZeroAllocs(t *testing.T) {
	s := newDoublesPadelState(0)
	s.Games = [2]int{3, 3}
	ev := make([]Event, 0, 64)
	allocs := testing.AllocsPerRun(100, func() {
		resetForServe(&s, s.Server)
		ev = awardPoint(&s, 0, ev[:0])
		if s.Phase == PhaseGameOver {
			s.Phase = PhasePoint
			s.Sets = [2]int{}
			s.Games = [2]int{3, 3}
		}
	})
	if allocs != 0 {
		t.Fatalf("allocs: got %v, want 0", allocs)
	}
}
