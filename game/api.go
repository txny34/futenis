// Package game is the pure-Go match session shared by the browser bridge
// (wasm/) and the online server (cmd/server): mode + live config + bots, and
// the flat LE float64 frame (prev/curr snapshots + events) both sides emit.
package game

import "github.com/txny34/futenis/sim"

// Buffer sizes of the shared frame format (see frame.go).
const (
	InputPerPlayer = inputPerPlayer
	InputLen       = inputLen
	OutLen         = outLen
	CfgBytes       = cfgBytes
	MinSteps       = minSteps
	MaxSteps       = maxSteps
)

// Session is a configured match: mode, live config and bots for non-human slots.
type Session struct{ s session }

// NewSession returns the default session (1v1, default config, every slot human).
func NewSession() *Session { return &Session{s: defaultSession()} }

// Configure validates and rebuilds the match; on error the session is unchanged.
func (g *Session) Configure(mode, difficulty, pointsToWin, humanMask int) error {
	return g.s.configure(mode, difficulty, pointsToWin, humanMask)
}

// Reset restarts the match with the current config and bots.
func (g *Session) Reset() { g.s.reset() }

// SetHumanMask changes which slots the clients drive, keeping the match state
// (online: a player who leaves is replaced by the bot).
func (g *Session) SetHumanMask(mask int) error { return g.s.setHumanMask(mask) }

// ConfigureFirst is Configure with Config.FirstServer forced (replays).
func (g *Session) ConfigureFirst(mode, difficulty, pointsToWin, humanMask, firstServer int) error {
	return g.s.configureFirst(mode, difficulty, pointsToWin, humanMask, firstServer)
}

// FirstServer is the live config's first server (recorded into tapes).
func (g *Session) FirstServer() int { return int(g.s.cfg.FirstServer) }

// SetView makes Run output frames as seen from seat (online replays); 0 = server space.
func (g *Session) SetView(seat int, doubles bool) error { return g.s.setView(seat, doubles) }

// RespawnBall asks the ball machine for a feed (playground only).
func (g *Session) RespawnBall() error { return g.s.respawnBall() }

// WriteConfig encodes the live config into dst (CfgBytes).
func (g *Session) WriteConfig(dst []byte, scratch []float64) { g.s.writeConfig(dst, scratch) }

// ReadConfig decodes src and applies it between frames.
func (g *Session) ReadConfig(src []byte, scratch []float64) error {
	return g.s.readConfig(src, scratch)
}

// Run advances n ticks (clamped to MinSteps..MaxSteps) with held inputs, bots
// filling non-human slots, and writes the frame into out (len OutLen),
// rotated for SetView's seat. No allocs.
func (g *Session) Run(inputs [sim.MaxPlayers]sim.PlayerInput, n int, out []float64) {
	if g.s.viewSeat == 0 {
		runFrame(g.s.match, inputs, n, out, &g.s.drv)
		return
	}
	runFrame(g.s.match, inputs, n, g.s.raw[:], &g.s.drv)
	ViewOut(out, g.s.raw[:], g.s.scratch[:], g.s.viewSeat, g.s.viewDoubles)
}

// DecodeInputs reads InputLen LE float64s (NaN/Inf -> 0).
func DecodeInputs(src []byte) [sim.MaxPlayers]sim.PlayerInput { return decodeInputs(src) }

// EncodeFloats writes src as LE float64s into dst.
func EncodeFloats(dst []byte, src []float64) { encodeFloats(dst, src) }

// LayoutEntry is one numeric key of futenis.layout().
type LayoutEntry struct {
	Key string
	Val float64
}

// LayoutEntries lists every numeric layout key in a fixed order.
func LayoutEntries() []LayoutEntry {
	in := layoutEntries()
	out := make([]LayoutEntry, len(in))
	for i, e := range in {
		out[i] = LayoutEntry{Key: e.key, Val: e.val}
	}
	return out
}

// LobbyPadsFlat is futenis.layout().lobbyPads: [x0, z0, x1, z1, x2, z2].
func LobbyPadsFlat() []any { return lobbyPadsFlat() }
