package main

import "time"

// Public-exposure limits (docs/design/online-spike.md §7).
const (
	maxConns      = 2 * maxRooms
	maxConnsPerIP = 4
	// idleTimeout closes a peer that sent nothing (no INPUT, no PING): a hidden
	// tab stops its game loop, so this is also the AFK kick.
	idleTimeout = 20 * time.Second
	// rivalTimeout closes a host still alone in its room.
	rivalTimeout = 5 * time.Minute
	// Window/maxMsgsPerWindow: INPUT is sent at <= 120 Hz plus 1 PING/s;
	// twice that leaves room for jitter bursts.
	maxMsgsPerWindow = 2 * (120 + 1)
	closeIdle        = 4003
	closeRate        = 4004
	closeNoRival     = 4005
	closeGone        = 4007 // rejoin: the held seat is gone (match over or hold expired)
)
