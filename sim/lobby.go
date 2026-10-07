package sim

// lobby.go: the open-world hub (docs/design/lobby.md). ModeLobby has one
// human, no ball, no net/walls and three match pads on the floor; standing
// on a pad for Config.LobbyHoldTime emits EvPadStart. The sim detects, the
// client decides (it reconfigures the match on EvPadStart).

// Vec2 is a point on the X/Z floor plane (meters).
type Vec2 struct {
	X, Z float64
}

// LobbyPadCount is how many match pads the lobby has.
const LobbyPadCount = 3

// LobbyPadRadius is every pad's radius (m). A sim constant, not Config.
const LobbyPadRadius = 1.5

// LobbyPads are the pad centers, in pad index order: 0 = Entrenamiento,
// 1 = 1v1 vs bot, 2 = 2v2 vs bots. Mirrored by client/src/lobby/pads.ts.
var LobbyPads = [LobbyPadCount]Vec2{{X: -6, Z: -4}, {X: 0, Z: -6}, {X: 6, Z: -4}}

// LobbySpawn is where the player appears (facing -Z, toward the pads).
var LobbySpawn = Vec3{X: 0, Y: 0, Z: 6}

// Exported Config layout indices of the lobby fields (wasm layout()).
const (
	CfgLobbyHoldTime = cfgLobbyHoldTime
	CfgLobbyHalfX    = cfgLobbyHalfX
	CfgLobbyHalfZ    = cfgLobbyHalfZ
)

// Lobby Config bounds (docs/design/lobby.md §3).
const (
	minLobbyHoldTime, maxLobbyHoldTime = 0.3, 5
	minLobbyHalfX, maxLobbyHalfX       = 10, 30
	minLobbyHalfZ, maxLobbyHalfZ       = 8, 25
)

// LobbyState is the pad-hold bookkeeping of ModeLobby.
type LobbyState struct {
	Pad     int8   // pad under the player: -1 none, 0..LobbyPadCount-1
	Hold    uint32 // ticks accumulated on Pad
	Started bool   // EvPadStart already emitted; cleared when Pad changes
}

// setupLobby places the lone human at the spawn with no ball in play.
func setupLobby(s *State) {
	s.Phase = PhaseRally
	s.Rally = RallyState{LastTeam: -1, LastPlayer: -1}
	s.Players[0] = Player{Pos: LobbySpawn, Team: 0, OnGround: true, Stamina: 1}
	s.Ball = Ball{}
	s.Lobby = LobbyState{Pad: -1}
}

// padUnder returns the pad index p stands on (grounded, horizontal distance
// <= LobbyPadRadius), or -1. Pads never overlap, so at most one matches.
func padUnder(p *Player) int8 {
	if !p.OnGround {
		return -1
	}
	for i := int8(0); i < LobbyPadCount; i++ {
		c := LobbyPads[i]
		dx := float64(p.Pos.X - c.X)
		dz := float64(p.Pos.Z - c.Z)
		if float64(dx*dx+dz*dz) <= LobbyPadRadius*LobbyPadRadius {
			return i
		}
	}
	return -1
}

// clampPlayerToLobby confines p to the symmetric lobby rectangle.
func clampPlayerToLobby(p *Player, cfg *Config) {
	xLimit := float64(cfg.LobbyHalfX - PlayerRadius)
	zLimit := float64(cfg.LobbyHalfZ - PlayerRadius)
	p.Pos.X = clamp(p.Pos.X, -xLimit, xLimit)
	p.Pos.Z = clamp(p.Pos.Z, -zLimit, zLimit)
}

// stepLobby runs the pad detection after movement (docs/design/lobby.md §4).
func stepLobby(s *State, ev []Event) []Event {
	l := &s.Lobby
	pad := padUnder(&s.Players[0])
	if pad != l.Pad {
		if l.Pad >= 0 {
			ev = append(ev, padEvent(s, EvPadLeave, l.Pad))
		}
		if pad >= 0 {
			ev = append(ev, padEvent(s, EvPadEnter, pad))
		}
		*l = LobbyState{Pad: pad}
	}
	if pad < 0 || l.Started {
		return ev
	}
	l.Hold++
	if int(l.Hold) >= s.Cfg.lobbyHoldTicks {
		l.Started = true
		ev = append(ev, padEvent(s, EvPadStart, pad))
	}
	return ev
}

// padEvent builds a lobby event: Player 0, Side = pad index, Pos = pad center.
func padEvent(s *State, kind EventKind, pad int8) Event {
	c := LobbyPads[pad]
	return Event{Kind: kind, Tick: s.Tick, Pos: Vec3{X: c.X, Z: c.Z}, Side: pad, Player: 0}
}

// sanitizeLobby bounds the lobby Config fields (lobbyHoldTicks is derived in
// recomputeTickCaches).
func (c *Config) sanitizeLobby() {
	c.LobbyHoldTime = clamp(c.LobbyHoldTime, minLobbyHoldTime, maxLobbyHoldTime)
	c.LobbyHalfX = clamp(c.LobbyHalfX, minLobbyHalfX, maxLobbyHalfX)
	c.LobbyHalfZ = clamp(c.LobbyHalfZ, minLobbyHalfZ, maxLobbyHalfZ)
}

// writeLobbyBlock flattens the lobby Config fields.
func writeLobbyBlock(c *Config, dst []float64) {
	dst[cfgLobbyHoldTime] = c.LobbyHoldTime
	dst[cfgLobbyHalfX] = c.LobbyHalfX
	dst[cfgLobbyHalfZ] = c.LobbyHalfZ
}

// readLobbyBlock is writeLobbyBlock's inverse (Sanitize applies bounds).
func readLobbyBlock(c *Config, src []float64) {
	c.LobbyHoldTime = src[cfgLobbyHoldTime]
	c.LobbyHalfX = src[cfgLobbyHalfX]
	c.LobbyHalfZ = src[cfgLobbyHalfZ]
}

// writeLobby fills the lobby snapshot block; -1 / 0 outside ModeLobby.
func writeLobby(s *State, dst []float64) {
	dst[SnapLobbyPad] = -1
	dst[SnapLobbyHold] = 0
	if s.Cfg.Mode != ModeLobby {
		return
	}
	dst[SnapLobbyPad] = float64(s.Lobby.Pad)
	if s.Lobby.Pad < 0 {
		return
	}
	hold := 1.0
	if n := s.Cfg.lobbyHoldTicks; n > 0 {
		hold = min(float64(s.Lobby.Hold)/float64(n), 1)
	}
	dst[SnapLobbyHold] = hold
}
