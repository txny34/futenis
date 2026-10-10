package game

import "github.com/txny34/futenis/sim"

// Per-slot client views for online 2v2 (1v1 uses slots 0/1 only). Every client
// renders itself as slot 0 / team 0. Slots are team = slot % 2, so:
//   slot 0: as is            slot 1: rotated (RotateOut1v1, swaps 0<->1, 2<->3)
//   slot 2: partners swapped slot 3: rotated, then partners swapped
// Swapping partners (0<->2, 1<->3) keeps the court and the teams; only who is
// "me" and who is "my partner" changes. All functions are allocation free.

// partnerDash are the dash-flag pairs exchanged with the partner swap.
var partnerDash = [...][2]int{
	{sim.SnapP0DashActive, sim.SnapP2DashActive}, {sim.SnapP1DashActive, sim.SnapP3DashActive},
}

// SwapPartnersOut writes src with slots 0<->2 and 1<->3 exchanged into dst
// (len OutLen, must not alias). Involution.
func SwapPartnersOut(dst, src []float64) {
	copy(dst[:OutLen], src[:OutLen])
	swapPartnersSnap(dst[outPrev:outCurr], src[outPrev:outCurr])
	swapPartnersSnap(dst[outCurr:outEventCount], src[outCurr:outEventCount])
	n := clamp(int(src[outEventCount]), 0, maxEvents)
	for i := 0; i < n; i++ {
		e := outEvents + i*eventStride
		dst[e+6] = partnerOf(src[e+6]) // player
	}
}

func swapPartnersSnap(d, s []float64) {
	for i := 0; i < sim.MaxPlayers; i++ {
		j := i ^ 2
		copy(d[playerPos[i]:playerPos[i]+3], s[playerPos[j]:playerPos[j]+3])
		copy(d[playerExtra[i]:playerExtra[i]+extraLen], s[playerExtra[j]:playerExtra[j]+extraLen])
		copy(d[playerContact[i]:playerContact[i]+contactLen], s[playerContact[j]:playerContact[j]+contactLen])
	}
	for _, p := range partnerDash {
		d[p[0]], d[p[1]] = s[p[1]], s[p[0]]
	}
	d[sim.SnapServerPlayer] = partnerOf(s[sim.SnapServerPlayer])
}

// partnerOf maps slot p to p^2 for 0..3 and keeps anything else (-1 = none).
func partnerOf(p float64) float64 {
	if p >= 0 && p < sim.MaxPlayers {
		return float64(int(p) ^ 2)
	}
	return p
}

// ViewOut writes frame src as seen by the client in slot (0..3) into dst.
// scratch (len OutLen) is used by slot 3. None of the three may alias.
func ViewOut(dst, src, scratch []float64, slot int) {
	switch slot {
	case 1:
		RotateOut1v1(dst, src)
	case 2:
		SwapPartnersOut(dst, src)
	case 3:
		RotateOut1v1(scratch, src)
		SwapPartnersOut(dst, scratch)
	default:
		copy(dst[:OutLen], src[:OutLen])
	}
}

// ViewInput turns an input from the client in slot (its own view) into server space.
func ViewInput(in sim.PlayerInput, slot int) sim.PlayerInput {
	if slot%2 == 1 {
		return RotateInput(in)
	}
	return in
}
