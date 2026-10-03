package sim

import "testing"

// clearsRealNet reports whether a ball launched with v from `from` passes
// over the physical net (no margin) when it crosses sides.
func clearsRealNet(cfg Config, from, v Vec3) bool {
	cfg.DriveNetClearance = 0
	return netClearanceOK(&cfg, from, v)
}

// TestStrike_AimAssistAlwaysClearsNet: any drive aimed at the rival half
// passes over the net, whatever the body part, timing error, charge or
// distance (the Roblox-style aim assist), never above the part's speed cap. Only aiming at your own side may
// end in the net.
func TestStrike_AimAssistAlwaysClearsNet(t *testing.T) {
	parts := []BodyPart{PartFoot, PartKnee, PartChest, PartShoulder, PartHead}
	n := 0
	for _, part := range parts {
		for _, u := range []float64{0, 0.4, 0.8, 1} {
			for tx := 1.0; tx <= 9.5; tx += 0.5 {
				for px := -9.5; px <= -1; px++ {
					for _, ch := range []float64{0, 0.5, 1} {
						s := newStrikeTestState()
						s.Tick = uint32(n * 7)
						n++
						from := Vec3{X: px, Y: s.Cfg.Parts[part].HIdeal}
						s.Ball.Pos = from
						p := &s.Players[0]
						p.Pos = Vec3{X: px}
						p.BufCharge = ch
						p.BufTarget = Vec3{X: tx, Y: BallRadius, Z: 1}
						strike(&s, 0, part, u, nil)
						if cap := s.Cfg.Parts[part].SpeedMax + s.Cfg.MomentumMax; s.Ball.Vel.Len() > cap+1e-9 {
							t.Fatalf("part=%d u=%.1f from x=%.1f target x=%.1f charge=%.1f: speed %.2f over cap %.2f", part, u, px, tx, ch, s.Ball.Vel.Len(), cap)
						}
						if !clearsRealNet(s.Cfg, from, s.Ball.Vel) {
							t.Fatalf("part=%d u=%.1f from x=%.1f target x=%.1f charge=%.1f: hit the net (v=%+v)", part, u, px, tx, ch, s.Ball.Vel)
						}
					}
				}
			}
		}
	}
}

// TestStrike_OwnSideTargetIsNotAssisted: aiming at your own half (the
// "muy bajo" aim) gets no net assist, so the ball stays on your side.
func TestStrike_OwnSideTargetIsNotAssisted(t *testing.T) {
	s := newStrikeTestState()
	part := PartFoot
	from := Vec3{X: -3, Y: s.Cfg.Parts[part].HIdeal}
	s.Ball.Pos = from
	s.Players[0].Pos = Vec3{X: -3}
	s.Players[0].BufTarget = Vec3{X: -1, Y: BallRadius}
	strike(&s, 0, part, 0, nil)
	if clearsRealNet(s.Cfg, from, s.Ball.Vel) {
		b := Ball{Pos: from, Vel: s.Ball.Vel}
		for i := 0; i < maxSolveTicks && sideOf(b.Pos.X) == 0; i++ {
			b.Vel.Y -= float64(s.Cfg.Gravity * Dt)
			b.Pos = b.Pos.AddScaled(b.Vel, Dt)
		}
		if sideOf(b.Pos.X) == 1 {
			t.Fatalf("own-side target crossed the net: v=%+v", s.Ball.Vel)
		}
	}
}

// TestDeviation_StaysOnRivalSide: a big deviation on a short shot can never
// move the target back onto the striker's own half.
func TestDeviation_StaysOnRivalSide(t *testing.T) {
	cfg := DefaultConfig()
	ps := &cfg.Parts[PartFoot]
	for h := uint64(0); h < 2000; h++ {
		got := deviateTarget(&cfg, Vec3{X: 0.6, Y: BallRadius}, Vec3{X: -5}, ps, 1, Hash64(h))
		if got.X < rivalMinTargetX {
			t.Fatalf("h=%d: deviated X=%.3f, want >= %.2f", h, got.X, rivalMinTargetX)
		}
	}
}
