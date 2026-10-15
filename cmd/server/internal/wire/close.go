package wire

import "strconv"

// WebSocket close codes the server sends; the client maps them to what it
// shows (client/src/net/net-status.ts). Frozen like the messages: only append.
const (
	CloseNormal   = 1000 // the match ended for this peer (rival left)
	CloseFull     = 4001 // room full, already queued, or same account/network in a ranked match
	CloseBusy     = 4002 // server at its room cap
	CloseIdle     = 4003 // nothing received for the idle timeout (AFK, hidden tab)
	CloseRate     = 4004 // too many messages
	CloseNoRival  = 4005 // nobody joined the room in time
	CloseNoAuth   = 4006 // ranked without a valid sign-in
	CloseGone     = 4007 // rejoin: the held seat is gone (match over or hold expired)
	CloseReplace  = 4008 // old connection of a player who rejoined
	CloseRequeue  = 4009 // ranked matchmaking moved a lone waiter: queue again at once
	CloseOutdated = 4010 // the client speaks another protocol version: reload the page
)

// ClientProtoOK checks a connection's ?proto= query. Clients from before the
// param send none and speak Version 1; when Version is bumped, require it.
func ClientProtoOK(v string) bool { return v == "" || v == strconv.Itoa(Version) }
