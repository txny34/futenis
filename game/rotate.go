package game

import "github.com/txny34/futenis/sim"

// Guest view for online 1v1 (docs/design/online-spike.md §3): the guest is
// slot 1 / team 1 on the server, but every client renders itself as slot 0 /
// team 0. Rotating the court 180° around Y (X -> -X, Z -> -Z) and swapping
// slot 0<->1 (2<->3) and team 0<->1 turns one view into the other. A rotation
// keeps handedness, so spin and part/quality are unchanged. Involution:
// applying it twice gives back the input.

// snapNeg are per-snapshot indices whose value is negated.
var snapNeg = [...]int{
	sim.SnapBallX, sim.SnapBallZ, sim.SnapLandingX, sim.SnapLandingZ, sim.SnapMachineX, sim.SnapMachineZ,
}

// snapSwap are per-snapshot index pairs whose values are exchanged.
var snapSwap = [...][2]int{
	{sim.SnapScore0, sim.SnapScore1}, {sim.SnapPoints0, sim.SnapPoints1},
	{sim.SnapGames0, sim.SnapGames1}, {sim.SnapSets0, sim.SnapSets1},
	{sim.SnapP0DashActive, sim.SnapP1DashActive}, {sim.SnapP2DashActive, sim.SnapP3DashActive},
}

// playerPos/playerExtra/playerContact are each slot's block starts.
var (
	playerPos     = [sim.MaxPlayers]int{sim.SnapP0X, sim.SnapP1X, sim.SnapP2X, sim.SnapP3X}
	playerExtra   = [sim.MaxPlayers]int{sim.SnapP0VelX, sim.SnapP1VelX, sim.SnapP2VelX, sim.SnapP3VelX}
	playerContact = [sim.MaxPlayers]int{sim.SnapP0ContactTicks, sim.SnapP1ContactTicks, sim.SnapP2ContactTicks, sim.SnapP3ContactTicks}
)

const (
	extraLen   = 7 // VelX, VelZ, Stamina, Charge, OnGround, Buffer, Flags
	contactLen = 4 // Ticks, Part, Quality, Ok
)

// RotateOut1v1 writes the guest view of frame src into dst (both len OutLen,
// must not alias). No allocations.
func RotateOut1v1(dst, src []float64) {
	copy(dst[:OutLen], src[:OutLen])
	rotateSnap(dst[outPrev:outCurr], src[outPrev:outCurr])
	rotateSnap(dst[outCurr:outEventCount], src[outCurr:outEventCount])
	n := clamp(int(src[outEventCount]), 0, maxEvents)
	for i := 0; i < n; i++ {
		rotateEvent(dst[outEvents+i*eventStride:], src[outEvents+i*eventStride:])
	}
}

// RotateInput turns a guest input (its own rotated view) into server space.
func RotateInput(in sim.PlayerInput) sim.PlayerInput {
	in.MoveX, in.MoveZ = -in.MoveX, -in.MoveZ
	in.TargetX, in.TargetZ = -in.TargetX, -in.TargetZ
	return in
}

func rotateSnap(d, s []float64) {
	for _, i := range snapNeg {
		d[i] = -s[i]
	}
	for _, p := range snapSwap {
		d[p[0]], d[p[1]] = s[p[1]], s[p[0]]
	}
	d[sim.SnapServer] = flipTeam(s[sim.SnapServer])
	d[sim.SnapLastTeam] = flipTeam(s[sim.SnapLastTeam])
	d[sim.SnapServerPlayer] = flipPlayer(s[sim.SnapServerPlayer])
	for i := 0; i < sim.MaxPlayers; i++ {
		j := i ^ 1
		rotatePos(d[playerPos[i]:], s[playerPos[j]:])
		copy(d[playerExtra[i]:playerExtra[i]+extraLen], s[playerExtra[j]:playerExtra[j]+extraLen])
		d[playerExtra[i]] = -s[playerExtra[j]]     // VelX
		d[playerExtra[i]+1] = -s[playerExtra[j]+1] // VelZ
		copy(d[playerContact[i]:playerContact[i]+contactLen], s[playerContact[j]:playerContact[j]+contactLen])
	}
}

func rotatePos(d, s []float64) {
	d[0], d[1], d[2] = -s[0], s[1], -s[2]
}

// rotateEvent mirrors one event record. side is a team for every event except
// the EvPad* lobby events, where it is a pad index; those never occur online
// (1v1 only), so flipping it as a team is safe.
func rotateEvent(d, s []float64) {
	rotatePos(d[2:], s[2:]) // posX, posY, posZ
	rotatePos(d[9:], s[9:]) // velX, velY, velZ
	d[5] = flipTeam(s[5])   // side
	d[6] = flipPlayer(s[6]) // player
}

// flipTeam maps team 0<->1 and keeps anything else (-1 = none).
func flipTeam(t float64) float64 {
	if t == 0 || t == 1 {
		return 1 - t
	}
	return t
}

// flipPlayer maps slot p to p^1 for 0..3 and keeps anything else (-1 = none).
func flipPlayer(p float64) float64 {
	if p >= 0 && p < sim.MaxPlayers {
		return float64(int(p) ^ 1)
	}
	return p
}
