package game

import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/txny34/futenis/ai"
	"github.com/txny34/futenis/sim"
)

// cfgBytes is the byte size of the live-tuning buffer (sim.CfgLen LE float64s).
const cfgBytes = sim.CfgLen * 8

// maxPointsToWin bounds futenis.configure's pointsToWin argument.
const maxPointsToWin = 99

// humanMaskAll is every slot human (no bots).
const humanMaskAll = 1<<sim.MaxPlayers - 1

// aiDriver fills the non-human slots with ai.Bot inputs, tick by tick.
type aiDriver struct {
	bots      [sim.MaxPlayers]ai.Bot
	humanMask uint8 // bit i set = slot i is driven by the client
}

func newDriver(d ai.Difficulty, humanMask uint8) aiDriver {
	drv := aiDriver{humanMask: humanMask}
	for i := range drv.bots {
		drv.bots[i] = ai.New(d, i)
	}
	return drv
}

// fill overwrites the inputs of AI slots from the current state (no allocs).
func (d *aiDriver) fill(m *sim.Match, in *[sim.MaxPlayers]sim.PlayerInput) {
	if d == nil || d.humanMask == humanMaskAll {
		return
	}
	s := m.State()
	if s.Cfg.Mode == sim.ModePlayground || s.Cfg.Mode == sim.ModeLobby {
		return // Entrenamiento (ball machine) and lobby (no ball): no bot
	}
	for i := 0; i < int(s.NumPlayers); i++ {
		if d.humanMask&(1<<i) == 0 {
			in[i] = d.bots[i].Think(&s, i)
		}
	}
}

// session is the configured match: mode, live config and bots.
type session struct {
	match *sim.Match
	cfg   sim.Config
	drv   aiDriver
	// configured counts successful configure calls; its parity picks
	// Config.FirstServer so consecutive matches alternate who opens serve
	// (deterministic, no RNG).
	configured int
}

func defaultSession() session {
	cfg := sim.DefaultConfig()
	return session{match: sim.NewMatch(cfg), cfg: cfg, drv: aiDriver{humanMask: humanMaskAll}}
}

// configure validates the JS arguments and rebuilds the match; on error the
// session is left untouched.
func (ss *session) configure(mode, difficulty, pointsToWin, humanMask int) error {
	if mode < int(sim.Mode1v1) || mode > int(sim.ModeLobby) {
		return fmt.Errorf("futenis.configure: mode %d not in 0..3", mode)
	}
	if difficulty < int(ai.Easy) || difficulty > int(ai.Pro) {
		return fmt.Errorf("futenis.configure: difficulty %d not in 0..3", difficulty)
	}
	if pointsToWin < 1 || pointsToWin > maxPointsToWin {
		return fmt.Errorf("futenis.configure: pointsToWin %d not in 1..%d", pointsToWin, maxPointsToWin)
	}
	if humanMask < 0 || humanMask > humanMaskAll {
		return fmt.Errorf("futenis.configure: humanMask %d not in 0..%d", humanMask, humanMaskAll)
	}
	cfg := ss.cfg
	cfg.Mode = sim.Mode(mode)
	cfg.PointsToWin = pointsToWin
	applyMatchFormat(&cfg, ss.configured)
	ss.configured++
	ss.cfg = cfg
	ss.match = sim.NewMatch(cfg)
	ss.drv = newDriver(ai.Difficulty(difficulty), uint8(humanMask))
	return nil
}

// applyMatchFormat sets the scoring model: every mode plays the flat point
// tally (first to PointsToWin, win by 2, capped; sim EvMatchPoint), padel
// scoring stays in the sim but is no longer used. The first server
// alternates with the parity of n (the configure call count).
func applyMatchFormat(cfg *sim.Config, n int) {
	cfg.FirstServer = int8(n % 2)
	cfg.ScoringMode = sim.ScoringSimple
}

func (ss *session) setHumanMask(mask int) error {
	if mask < 0 || mask > humanMaskAll {
		return fmt.Errorf("setHumanMask: %d not in 0..%d", mask, humanMaskAll)
	}
	d := ss.drv.bots[0].Difficulty()
	for i := range ss.drv.bots {
		if ss.drv.humanMask&(1<<i) != 0 && mask&(1<<i) == 0 {
			ss.drv.bots[i] = ai.New(d, i) // fresh bot for a seat a person just left
		}
	}
	ss.drv.humanMask = uint8(mask)
	return nil
}

// reset restarts the match with the current config and bots.
func (ss *session) reset() {
	ss.match = sim.NewMatch(ss.cfg)
	ss.drv = newDriver(ss.drv.bots[0].Difficulty(), ss.drv.humanMask)
}

// respawnBall is Playground-only: it asks the ball machine for a feed now
// (sim.Match.Feed, docs/design/training.md §4). JS exposes it as both
// respawnBall() and feed().
func (ss *session) respawnBall() error {
	if ss.cfg.Mode != sim.ModePlayground {
		return fmt.Errorf("futenis.respawnBall: only available in playground mode (2)")
	}
	ss.match.Feed()
	return nil
}

// writeConfig encodes the live config into dst (cfgBytes LE float64s).
func (ss *session) writeConfig(dst []byte, scratch []float64) {
	sim.WriteConfig(&ss.cfg, scratch)
	encodeFloats(dst, scratch)
}

// readConfig decodes src and applies it between frames. Mode and
// PointsToWin stay as configured (they need configure, not live tuning).
func (ss *session) readConfig(src []byte, scratch []float64) error {
	for i := range scratch {
		f := math.Float64frombits(binary.LittleEndian.Uint64(src[i*8:]))
		if math.IsNaN(f) || math.IsInf(f, 0) {
			return fmt.Errorf("futenis.setConfig: field %d is not finite", i)
		}
		scratch[i] = f
	}
	cfg := sim.ReadConfig(scratch)
	cfg.Mode, cfg.PointsToWin = ss.cfg.Mode, ss.cfg.PointsToWin
	ss.cfg = cfg
	st := ss.match.State()
	st.Cfg = cfg
	ss.match.Restore(st)
	return nil
}
