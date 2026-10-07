package sim

import "testing"

// serveUntilBounce drives the server (slot 0) through the real padel serve:
// Strike drops the toss, extra (BtnLob) is added from the given tick after
// the toss, and the match is stepped until the toss bounced legally (the
// release tick). It returns the match ready to release.
func serveUntilBounce(t *testing.T, extra uint8, extraFrom int) *Match {
	t.Helper()
	m := NewMatch(DefaultConfig())
	in := PlayerInput{TargetX: 4, TargetZ: 1.5, Buttons: BtnStrike}
	for i := 0; i < 10*TickRate; i++ {
		if i >= extraFrom {
			in.Buttons = BtnStrike | extra
		}
		m.Step([MaxPlayers]PlayerInput{in})
		if m.State().Rally.ServeTossBounced {
			return m
		}
	}
	t.Fatal("toss never bounced")
	return nil
}

// releaseServe releases every button and steps until the serve kick,
// then returns the ball state right after it and the max flight height
// until it next touches the floor.
func releaseServe(t *testing.T, m *Match) (Ball, float64) {
	t.Helper()
	in := PlayerInput{TargetX: 4, TargetZ: 1.5}
	for i := 0; i < TickRate; i++ {
		for _, e := range m.Step([MaxPlayers]PlayerInput{in}) {
			if e.Kind != EvKick {
				continue
			}
			kicked := m.State().Ball
			top := kicked.Pos.Y
			for j := 0; j < 5*TickRate; j++ {
				m.Step([MaxPlayers]PlayerInput{})
				b := m.State().Ball
				if b.Pos.Y > top {
					top = b.Pos.Y
				}
				if b.Vel.Y > 0 && b.Pos.Y < 0.2 && j > 0 {
					break
				}
			}
			return kicked, top
		}
	}
	t.Fatal("serve never struck")
	return Ball{}, 0
}

// A Lob latched on the serve strikes the regular serve: the same
// trajectory as Strike with the same aim and timing (padel: the serve is
// struck at/below the waist, there is no lob serve).
func TestServeLobLatchStrikesRegularServe(t *testing.T) {
	drive := serveUntilBounce(t, 0, 0)
	lob := serveUntilBounce(t, 0, 0)
	s := lob.State()
	s.Players[0].ShotType = ShotLob
	lob.Restore(s)

	wantBall, _ := releaseServe(t, drive)
	gotBall, _ := releaseServe(t, lob)
	if gotBall != wantBall {
		t.Fatalf("lob-latched serve = %+v, want the drive serve %+v", gotBall, wantBall)
	}
}

// Pressing Lob (right click) during the toss drop never produces a lob:
// the serve stays far below the lob apex range.
func TestServeLobPressNeverLobs(t *testing.T) {
	m := serveUntilBounce(t, BtnLob, 20)
	if m.State().Players[0].ShotType != ShotLob {
		t.Fatal("setup: Lob press did not latch ShotLob")
	}
	_, top := releaseServe(t, m)
	if cfg := DefaultConfig(); top >= cfg.LobApexMin {
		t.Fatalf("serve apex %.2f m >= LobApexMin %.2f: a lob serve", top, cfg.LobApexMin)
	}
}
