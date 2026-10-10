package game

import "github.com/txny34/futenis/sim"

// Per-seat client views for online matches. Every client renders itself as
// slot 0 / team 0. Teams (sim/rules.go): 1v1 = slot; 2v2 = slot/2, i.e. {0,1}
// vs {2,3}. So in 2v2:
//   slot 0: as is              slot 1 (my partner): swap 0<->1, 2<->3
//   slot 2: RotateOut2v2       slot 3: RotateOut2v2, then swap 0<->1, 2<->3
// and in 1v1 slot 1 is RotateOut1v1. All functions are allocation free.

// SwapSlotsOut writes src with every slot i exchanged with i^k into dst (len
// OutLen, must not alias), keeping the court and the teams. Involution.
func SwapSlotsOut(dst, src []float64, k int) {
	copy(dst[:OutLen], src[:OutLen])
	swapSlotsSnap(dst[outPrev:outCurr], src[outPrev:outCurr], k)
	swapSlotsSnap(dst[outCurr:outEventCount], src[outCurr:outEventCount], k)
	n := clamp(int(src[outEventCount]), 0, maxEvents)
	for i := 0; i < n; i++ {
		e := outEvents + i*eventStride
		dst[e+6] = flipPlayer(src[e+6], k) // player
	}
}

func swapSlotsSnap(d, s []float64, k int) {
	for i := 0; i < sim.MaxPlayers; i++ {
		j := i ^ k
		copy(d[playerPos[i]:playerPos[i]+3], s[playerPos[j]:playerPos[j]+3])
		copy(d[playerExtra[i]:playerExtra[i]+extraLen], s[playerExtra[j]:playerExtra[j]+extraLen])
		copy(d[playerContact[i]:playerContact[i]+contactLen], s[playerContact[j]:playerContact[j]+contactLen])
		d[playerDash[i]] = s[playerDash[j]]
	}
	d[sim.SnapServerPlayer] = flipPlayer(s[sim.SnapServerPlayer], k)
}

// ViewOut writes frame src as seen by the client in slot (0..3) into dst.
// doubles: 2v2 room. scratch (len OutLen) is used by 2v2 slot 3. None may alias.
func ViewOut(dst, src, scratch []float64, slot int, doubles bool) {
	switch {
	case slot == 0:
		copy(dst[:OutLen], src[:OutLen])
	case !doubles:
		RotateOut1v1(dst, src)
	case slot == 1:
		SwapSlotsOut(dst, src, 1)
	case slot == 2:
		RotateOut2v2(dst, src)
	default:
		RotateOut2v2(scratch, src)
		SwapSlotsOut(dst, scratch, 1)
	}
}

// ViewInput turns an input from the client in slot (its own view) into server
// space: the far team's clients see the court rotated.
func ViewInput(in sim.PlayerInput, slot int, doubles bool) sim.PlayerInput {
	far := slot == 1
	if doubles {
		far = slot >= 2
	}
	if far {
		return RotateInput(in)
	}
	return in
}
