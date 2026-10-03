package ai

import (
	"testing"

	"github.com/txny34/futenis/sim"
)

// stepBall must report every floor bounce the sim's prediction resolves,
// including the edge where the ball comes to rest (sim emits EvBounceFloor).
func TestStepBallReportsBounces(t *testing.T) {
	cfg := sim.DefaultConfig()
	ball := sim.Ball{Pos: sim.Vec3{X: -5, Y: 1.5}}
	bounces := 0
	for k := 0; k < 4*sim.TickRate; k++ {
		var b bool
		ball, b = stepBall(ball, &cfg)
		if b {
			bounces++
		}
	}
	if bounces < 2 {
		t.Fatalf("dropped ball reported %d bounces, want >= 2", bounces)
	}
	if !ball.Resting {
		t.Fatalf("ball should be resting after 4 s: %+v", ball)
	}
	// Resting is edge-triggered: no more bounces once at rest.
	if _, b := stepBall(ball, &cfg); b {
		t.Fatal("a resting ball must not report another bounce")
	}
}

func TestStepBallMatchesSimPrediction(t *testing.T) {
	cfg := sim.DefaultConfig()
	ball := sim.Ball{Pos: sim.Vec3{X: -3, Y: 1}, Vel: sim.Vec3{X: -6, Y: 2, Z: 3}}
	want := sim.PredictBall(ball, &cfg, 200)
	for k := 0; k < 200; k++ {
		ball, _ = stepBall(ball, &cfg)
	}
	if ball.Pos != want.Pos || ball.Vel != want.Vel {
		t.Fatalf("stepBall diverged from sim.PredictBall: %+v vs %+v", ball, want)
	}
}
