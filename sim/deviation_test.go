package sim

import (
	"math"
	"testing"
)

func TestDeviation_ZeroAtPerfectCenter_BoundedByR(t *testing.T) {
	cfg := DefaultConfig()
	ps := &cfg.Parts[PartFoot]
	target := Vec3{X: 5, Y: BallRadius, Z: 1}
	ball := Vec3{X: 0, Z: 0}

	for tick := uint32(0); tick < 50; tick++ {
		h := strikeHash(tick, 0, ball)
		u := 0.0
		got := deviateTarget(&cfg, target, ball, ps, u, h)

		r := ps.DevBase
		maxOffset := r * math.Sqrt(1*1+0.6*0.6)
		offset := got.Sub(target).Len()
		if offset > maxOffset+1e-9 {
			t.Errorf("tick %d: deviation offset %v exceeds bound %v (R=%v)", tick, offset, maxOffset, r)
		}
	}
}

func TestDeviation_ForcedContactDeviatesMoreThanPerfect(t *testing.T) {
	cfg := DefaultConfig()
	ps := &cfg.Parts[PartFoot]
	target := Vec3{X: 5, Y: BallRadius}
	ball := Vec3{}
	h := strikeHash(1, 0, ball)

	perfect := deviateTarget(&cfg, target, ball, ps, 0, h)
	forced := deviateTarget(&cfg, target, ball, ps, 1, h)

	if forced.Sub(target).Len() <= perfect.Sub(target).Len() {
		t.Errorf("forced contact (u=1) should deviate at least as much as perfect (u=0): forced=%v perfect=%v",
			forced.Sub(target).Len(), perfect.Sub(target).Len())
	}
}

func TestDeviation_ReclampedOnZOnly(t *testing.T) {
	cfg := DefaultConfig()
	ps := &cfg.Parts[PartKnee] // large DevMax so u=1 can push past the sideline
	target := Vec3{X: 5, Y: BallRadius, Z: CourtHalfWidth - cfg.TargetWallMargin - 0.01}
	ball := Vec3{Z: target.Z - 5}

	got := deviateTarget(&cfg, target, ball, ps, 1, strikeHash(1, 0, ball))

	limit := CourtHalfWidth - cfg.TargetWallMargin
	if got.Z > limit+1e-9 || got.Z < -limit-1e-9 {
		t.Errorf("Z: got %v, want within +/-%v", got.Z, limit)
	}
}

func TestShotSpeed_PerfectContactBarelyDeviatesPower(t *testing.T) {
	cfg := DefaultConfig()
	ps := &cfg.Parts[PartFoot]
	base := ps.SpeedMin + (ps.SpeedMax-ps.SpeedMin)*0.5

	got := shotSpeed(ps, 0.5, 0, strikeHash(1, 0, Vec3{}), &cfg)

	assertFloat(t, "shotSpeed at u=0", got, base)
}

func TestShotSpeed_InterpolatesChargeBetweenMinAndMax(t *testing.T) {
	cfg := DefaultConfig()
	ps := &cfg.Parts[PartFoot]

	min := shotSpeed(ps, 0, 0, strikeHash(1, 0, Vec3{}), &cfg)
	max := shotSpeed(ps, 1, 0, strikeHash(1, 0, Vec3{}), &cfg)

	assertFloat(t, "shotSpeed at charge=0", min, ps.SpeedMin)
	assertFloat(t, "shotSpeed at charge=1", max, ps.SpeedMax)
}
