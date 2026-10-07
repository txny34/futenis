package sim

import (
	"math"
	"testing"
)

// lobby_test.go: docs/design/lobby.md §9 (Agente A) acceptance criteria.

func newLobby(t *testing.T) *Match {
	t.Helper()
	cfg := DefaultConfig()
	cfg.Mode = ModeLobby
	cfg.Sanitize()
	return NewMatch(cfg)
}

// placeAt teleports the lobby player (grounded, at rest) to (x, z).
func placeAt(m *Match, x, z float64) {
	s := m.State()
	s.Players[0].Pos = Vec3{X: x, Z: z}
	s.Players[0].Vel = Vec3{}
	s.Players[0].OnGround = true
	m.Restore(s)
}

func stepIdle(m *Match) []Event {
	return m.Step([MaxPlayers]PlayerInput{})
}

func TestLobby_LayoutIndices(t *testing.T) {
	checks := []struct {
		name      string
		got, want int
	}{
		{"ModeLobby", int(ModeLobby), 3},
		{"cfgLobbyHoldTime", cfgLobbyHoldTime, 72},
		{"cfgLobbyHalfX", cfgLobbyHalfX, 73},
		{"cfgLobbyHalfZ", cfgLobbyHalfZ, 74},
		{"cfgFeedValidBounce", cfgFeedValidBounce, 71},
		{"CfgScalarLen", CfgScalarLen, 75},
		{"CfgPartsStart", CfgPartsStart, 75},
		{"CfgLen", CfgLen, 152},
		{"SnapMachineZ", SnapMachineZ, 90},
		{"SnapLobbyPad", SnapLobbyPad, 91},
		{"SnapLobbyHold", SnapLobbyHold, 92},
		{"SnapLen", SnapLen, 93},
		{"EvReturnFail", int(EvReturnFail), 23},
		{"EvPadEnter", int(EvPadEnter), 24},
		{"EvPadLeave", int(EvPadLeave), 25},
		{"EvPadStart", int(EvPadStart), 26},
		{"CfgLobbyHoldTime", CfgLobbyHoldTime, cfgLobbyHoldTime},
		{"CfgLobbyHalfX", CfgLobbyHalfX, cfgLobbyHalfX},
		{"CfgLobbyHalfZ", CfgLobbyHalfZ, cfgLobbyHalfZ},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %d, want %d", c.name, c.got, c.want)
		}
	}
}

// TestLobby_PadContract prints and pins the pad constants mirrored by
// client/src/lobby/pads.ts (docs/design/lobby.md §2).
func TestLobby_PadContract(t *testing.T) {
	want := [LobbyPadCount]Vec2{{X: -6, Z: -4}, {X: 0, Z: -6}, {X: 6, Z: -4}}
	if LobbyPads != want || LobbyPadRadius != 1.5 || LobbyPadCount != 3 {
		t.Fatalf("pads=%v radius=%v", LobbyPads, LobbyPadRadius)
	}
	if LobbySpawn != (Vec3{Z: 6}) {
		t.Fatalf("spawn=%v", LobbySpawn)
	}
	for i, p := range LobbyPads {
		t.Logf("pad %d: x=%v z=%v r=%v", i, p.X, p.Z, LobbyPadRadius)
	}
}

func TestLobby_ConfigRoundTripAndSanitize(t *testing.T) {
	want := DefaultConfig()
	if want.LobbyHoldTime != 1.5 || want.LobbyHalfX != 16 || want.LobbyHalfZ != 12 || want.lobbyHoldTicks != 180 {
		t.Fatalf("defaults: %v %v %v %d", want.LobbyHoldTime, want.LobbyHalfX, want.LobbyHalfZ, want.lobbyHoldTicks)
	}
	want.Mode = ModeLobby
	want.LobbyHoldTime, want.LobbyHalfX, want.LobbyHalfZ = 2.25, 20, 9
	want.Sanitize()
	buf := make([]float64, CfgLen)
	WriteConfig(&want, buf)
	if buf[cfgLobbyHoldTime] != 2.25 || buf[cfgLobbyHalfX] != 20 || buf[cfgLobbyHalfZ] != 9 {
		t.Fatalf("written: %v", buf[cfgLobbyHoldTime:CfgScalarLen])
	}
	if got := ReadConfig(buf); got != want || got.lobbyHoldTicks != 270 {
		t.Fatalf("round trip mismatch:\n got %+v\nwant %+v", got, want)
	}

	lo := DefaultConfig()
	lo.LobbyHoldTime, lo.LobbyHalfX, lo.LobbyHalfZ = 0, 1, -3
	lo.Sanitize()
	if lo.LobbyHoldTime != 0.3 || lo.LobbyHalfX != 10 || lo.LobbyHalfZ != 8 || lo.lobbyHoldTicks != 36 {
		t.Errorf("low clamp: %v %v %v %d", lo.LobbyHoldTime, lo.LobbyHalfX, lo.LobbyHalfZ, lo.lobbyHoldTicks)
	}
	hi := DefaultConfig()
	hi.LobbyHoldTime, hi.LobbyHalfX, hi.LobbyHalfZ = 99, 99, 99
	hi.Sanitize()
	if hi.LobbyHoldTime != 5 || hi.LobbyHalfX != 30 || hi.LobbyHalfZ != 25 || hi.lobbyHoldTicks != 600 {
		t.Errorf("high clamp: %v %v %v %d", hi.LobbyHoldTime, hi.LobbyHalfX, hi.LobbyHalfZ, hi.lobbyHoldTicks)
	}
}

func TestLobby_ModeSanitizeRange(t *testing.T) {
	c := DefaultConfig()
	c.Mode = ModeLobby
	c.Sanitize()
	if c.Mode != ModeLobby {
		t.Fatalf("ModeLobby must survive Sanitize, got %d", c.Mode)
	}
	c.Mode = ModeLobby + 1
	c.Sanitize()
	if c.Mode != Mode1v1 {
		t.Fatalf("mode 4 must clamp to 1v1, got %d", c.Mode)
	}
}

func TestLobby_NewMatch(t *testing.T) {
	s := newLobby(t).State()
	if s.NumPlayers != 1 || numPlayersForMode(ModeLobby) != 1 {
		t.Fatalf("NumPlayers=%d", s.NumPlayers)
	}
	p := s.Players[0]
	if p.Pos != LobbySpawn || !p.OnGround || p.Stamina != 1 || p.Team != 0 {
		t.Fatalf("player=%+v", p)
	}
	if s.Lobby != (LobbyState{Pad: -1}) || s.Phase != PhaseRally || s.Ball != (Ball{}) {
		t.Fatalf("lobby=%+v phase=%d ball=%+v", s.Lobby, s.Phase, s.Ball)
	}
}

// TestLobby_WalkToPadStartsExactlyOnce walks from the spawn onto pad 1 and
// stays there.
func TestLobby_WalkToPadStartsExactlyOnce(t *testing.T) {
	m := newLobby(t)
	var enterTick, startTick uint32
	var enters, starts, leaves int
	for i := 0; i < 20*TickRate; i++ {
		s := m.State()
		in := PlayerInput{}
		if s.Players[0].Pos.Z > LobbyPads[1].Z {
			in.MoveZ = -1
		}
		for _, e := range m.Step([MaxPlayers]PlayerInput{in}) {
			switch e.Kind {
			case EvPadEnter:
				enters++
				enterTick = e.Tick
				assertPadEvent(t, e, 1)
			case EvPadStart:
				starts++
				startTick = e.Tick
				assertPadEvent(t, e, 1)
			case EvPadLeave:
				leaves++
			}
		}
	}
	if enters != 1 || starts != 1 || leaves != 0 {
		t.Fatalf("enters=%d starts=%d leaves=%d", enters, starts, leaves)
	}
	// The enter tick is the first held tick (Hold=1), so the start lands on
	// the lobbyHoldTicks-th tick on the pad.
	if got := startTick - enterTick + 1; got != 180 {
		t.Fatalf("start after %d ticks on the pad, want 180", got)
	}
	if s := m.State(); !s.Lobby.Started || s.Lobby.Pad != 1 {
		t.Fatalf("lobby=%+v", s.Lobby)
	}
}

func assertPadEvent(t *testing.T, e Event, pad int8) {
	t.Helper()
	c := LobbyPads[pad]
	if e.Side != pad || e.Player != 0 || e.Pos != (Vec3{X: c.X, Z: c.Z}) {
		t.Fatalf("event=%+v, want pad %d", e, pad)
	}
}

func TestLobby_LeaveResetsHoldAndReentryStartsOver(t *testing.T) {
	m := newLobby(t)
	placeAt(m, LobbyPads[0].X, LobbyPads[0].Z)
	if ev := stepIdle(m); countKind(ev, EvPadEnter) != 1 || ev[0].Side != 0 {
		t.Fatalf("enter: %+v", ev)
	}
	for i := 0; i < 100; i++ {
		stepIdle(m)
	}
	if h := m.State().Lobby.Hold; h != 101 {
		t.Fatalf("hold=%d want 101", h)
	}
	placeAt(m, 0, 6)
	ev := stepIdle(m)
	if countKind(ev, EvPadLeave) != 1 || ev[0].Side != 0 || ev[0].Pos.X != LobbyPads[0].X {
		t.Fatalf("leave: %+v", ev)
	}
	if s := m.State(); s.Lobby.Hold != 0 || s.Lobby.Pad != -1 {
		t.Fatalf("lobby after leave=%+v", s.Lobby)
	}
	placeAt(m, LobbyPads[0].X, LobbyPads[0].Z)
	total := 0
	for i := 0; i < 179; i++ {
		total += countKind(stepIdle(m), EvPadStart)
	}
	if total != 0 || m.State().Lobby.Hold != 179 {
		t.Fatalf("re-entry must start from zero: starts=%d hold=%d", total, m.State().Lobby.Hold)
	}
	if countKind(stepIdle(m), EvPadStart) != 1 {
		t.Fatal("start expected on the 180th tick after re-entry")
	}
}

func TestLobby_AirborneDoesNotCount(t *testing.T) {
	m := newLobby(t)
	placeAt(m, LobbyPads[2].X, LobbyPads[2].Z)
	stepIdle(m) // enter
	ev := m.Step([MaxPlayers]PlayerInput{{Buttons: BtnJump}})
	if countKind(ev, EvJump) != 1 || countKind(ev, EvPadLeave) != 1 {
		t.Fatalf("jumping off the pad must leave it: %+v", ev)
	}
	airborne := 0
	for i := 0; i < TickRate; i++ {
		ev = m.Step([MaxPlayers]PlayerInput{{Buttons: BtnJump}}) // held: no re-jump
		if !m.State().Players[0].OnGround {
			airborne++
			if m.State().Lobby.Pad != -1 || countKind(ev, EvPadEnter) != 0 {
				t.Fatal("airborne must not count as on the pad")
			}
		}
		if countKind(ev, EvJump) != 0 {
			t.Fatal("held jump must not re-trigger (Held must update in the lobby)")
		}
	}
	if airborne == 0 {
		t.Fatal("player never left the ground")
	}
	if s := m.State(); s.Lobby.Pad != 2 || !s.Players[0].OnGround {
		t.Fatalf("landing on the pad must re-enter it: %+v", s.Lobby)
	}
}

func TestLobby_StartedResetsAfterLeaving(t *testing.T) {
	m := newLobby(t)
	placeAt(m, LobbyPads[1].X, LobbyPads[1].Z)
	for i := 0; i < 400; i++ {
		stepIdle(m)
	}
	if !m.State().Lobby.Started {
		t.Fatal("expected Started")
	}
	placeAt(m, 0, 6)
	stepIdle(m)
	if m.State().Lobby.Started {
		t.Fatal("leaving the pad must reset Started")
	}
}

func TestLobby_ClampNeverExceedsBounds(t *testing.T) {
	dirs := [][2]float64{{1, 0}, {-1, 0}, {0, 1}, {0, -1}, {1, 1}, {-1, -1}}
	for _, d := range dirs {
		m := newLobby(t)
		limX := m.State().Cfg.LobbyHalfX - PlayerRadius
		limZ := m.State().Cfg.LobbyHalfZ - PlayerRadius
		reachedX := false
		for i := 0; i < 10*TickRate; i++ {
			btn := BtnSprint
			if i%60 == 0 {
				btn |= BtnDash
			}
			m.Step([MaxPlayers]PlayerInput{{MoveX: d[0], MoveZ: d[1], Buttons: btn}})
			p := m.State().Players[0].Pos
			if math.Abs(p.X) > limX || math.Abs(p.Z) > limZ {
				t.Fatalf("dir %v: escaped to %+v", d, p)
			}
			reachedX = reachedX || math.Abs(p.X) == limX || math.Abs(p.Z) == limZ
		}
		if !reachedX {
			t.Fatalf("dir %v never reached a border (outside the court?)", d)
		}
	}
}

func TestLobby_ClampDispatchKeepsCourtClampElsewhere(t *testing.T) {
	s := State{Cfg: DefaultConfig()}
	p := Player{Pos: Vec3{X: 15, Z: 15}}
	want := p
	clampPlayerToCourt(&want)
	clampPlayer(&s, &p)
	if p.Pos != want.Pos || p.Pos.Z != CourtHalfWidth-PlayerRadius {
		t.Fatalf("court clamp changed: %+v", p.Pos)
	}
	s.Cfg.Mode = ModeLobby
	p.Pos = Vec3{X: -40, Z: 40}
	clampPlayer(&s, &p)
	if p.Pos.X != -(16-PlayerRadius) || p.Pos.Z != 12-PlayerRadius {
		t.Fatalf("lobby clamp: %+v", p.Pos)
	}
}

func TestLobby_StrikeLobIgnoredBallStill(t *testing.T) {
	m := newLobby(t)
	for i := 0; i < 2*TickRate; i++ {
		btn := uint8(0)
		if i%20 < 10 {
			btn = BtnStrike
		} else if i%40 < 30 {
			btn = BtnLob
		}
		ev := m.Step([MaxPlayers]PlayerInput{{MoveX: 0.2, Buttons: btn}})
		if countKind(ev, EvKick) != 0 || countKind(ev, EvWhiff) != 0 || countKind(ev, EvBodyHit) != 0 {
			t.Fatalf("tick %d: strike events in lobby: %+v", i, ev)
		}
		if b := m.State().Ball; b != (Ball{}) {
			t.Fatalf("ball moved: %+v", b)
		}
	}
	if p := m.State().Players[0]; p.ChargeTicks != 0 || p.Buffer != 0 {
		t.Fatalf("no charge in lobby: %+v", p)
	}
}

func TestLobby_Snapshot(t *testing.T) {
	m := newLobby(t)
	dst := make([]float64, SnapLen)
	s := m.State()
	WriteSnapshot(&s, dst)
	if dst[SnapLobbyPad] != -1 || dst[SnapLobbyHold] != 0 || dst[SnapMode] != 3 || dst[SnapNumPlayers] != 1 {
		t.Fatalf("idle lobby snapshot: pad=%v hold=%v", dst[SnapLobbyPad], dst[SnapLobbyHold])
	}
	placeAt(m, LobbyPads[1].X, LobbyPads[1].Z)
	for i := 0; i < 90; i++ {
		stepIdle(m)
	}
	s = m.State()
	WriteSnapshot(&s, dst)
	if dst[SnapLobbyPad] != 1 || dst[SnapLobbyHold] != 0.5 {
		t.Fatalf("half hold: pad=%v hold=%v", dst[SnapLobbyPad], dst[SnapLobbyHold])
	}
	s.Lobby.Hold = 9999
	WriteSnapshot(&s, dst)
	if dst[SnapLobbyHold] != 1 {
		t.Fatalf("hold must clamp to 1, got %v", dst[SnapLobbyHold])
	}
	s.Cfg.lobbyHoldTicks = 0
	WriteSnapshot(&s, dst)
	if dst[SnapLobbyHold] != 1 {
		t.Fatalf("zero hold ticks: got %v", dst[SnapLobbyHold])
	}

	other := NewMatch(DefaultConfig()).State()
	WriteSnapshot(&other, dst)
	if dst[SnapLobbyPad] != -1 || dst[SnapLobbyHold] != 0 {
		t.Fatalf("non-lobby: pad=%v hold=%v", dst[SnapLobbyPad], dst[SnapLobbyHold])
	}
}

func TestLobby_StepAllocatesZero(t *testing.T) {
	m := newLobby(t)
	g := newLCG(7)
	allocs := testing.AllocsPerRun(1000, func() {
		m.Step([MaxPlayers]PlayerInput{scriptedInput(g)})
	})
	if allocs != 0 {
		t.Errorf("lobby Step allocated %v times per call, want 0", allocs)
	}
}
