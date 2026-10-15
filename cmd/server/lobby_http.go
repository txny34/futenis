package main

import (
	"context"
	"net/http"
	"time"

	"github.com/coder/websocket"
	"github.com/txny34/futenis/cmd/server/internal/lobby"
	"github.com/txny34/futenis/cmd/server/internal/names"
	"github.com/txny34/futenis/cmd/server/internal/netlimit"
	"github.com/txny34/futenis/cmd/server/internal/wire"
)

// ServeLobby upgrades /lobby: same origin check and per-client caps as /ws.
func (h *hub) ServeLobby(w http.ResponseWriter, req *http.Request) {
	ip := netlimit.LimitKey(netlimit.ClientIP(req, h.proxied))
	if !h.conns.Acquire(ip) {
		http.Error(w, "too many connections", http.StatusTooManyRequests)
		return
	}
	defer h.conns.Release(ip)
	if !h.lobbyConns.Acquire(ip) {
		http.Error(w, "too many lobby connections", http.StatusTooManyRequests)
		return
	}
	defer h.lobbyConns.Release(ip)
	c, err := websocket.Accept(w, req, &websocket.AcceptOptions{OriginPatterns: h.origins})
	if err != nil {
		return // Accept already wrote the HTTP error
	}
	defer c.CloseNow()
	c.SetReadLimit(readLimit)
	if !wire.ClientProtoOK(req.URL.Query().Get("proto")) {
		_ = c.Close(websocket.StatusCode(wire.CloseOutdated), "reload the game")
		return
	}
	p := wsPeer{c}
	l, id, ok := h.joinLobby(p)
	if !ok {
		p.Close(wire.CloseFull, "lobby full")
		return
	}
	defer l.Leave(id)
	l.SetClient(id, ip, req.URL.Query().Get("v") == wire.CompactLobbyFlag)
	h.lobbyLoop(req.Context(), l, id, c)
}

// joinLobby seats p in the first lobby with room, opening another (with its
// own broadcast loop) when all are full, up to lobby.MaxShards. The join itself
// (which sends) runs outside the hub lock; a lobby that filled meanwhile is retried.
func (h *hub) joinLobby(p lobby.Peer) (l *lobby.Lobby, id int, ok bool) {
	for range 3 {
		if l = h.pickLobby(); l == nil {
			return nil, 0, false
		}
		if id, ok = l.Join(p); ok {
			return l, id, true
		}
	}
	return nil, 0, false
}

func (h *hub) pickLobby() *lobby.Lobby {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, l := range h.lobbies {
		if !l.Full() {
			return l
		}
	}
	if len(h.lobbies) >= lobby.MaxShards {
		return nil
	}
	l := lobby.New()
	h.lobbies = append(h.lobbies, l)
	tick, stop := h.lobbyTicker()
	go func() { defer stop(); l.Run(tick, h.done) }()
	return l
}

// lobbyLoop reads PRESENCE / NAME / LOBBY_STYLE / EMOTE / PING until the peer leaves, floods, sends
// garbage or stays silent for lobbyIdle (hidden tab).
func (h *hub) lobbyLoop(ctx context.Context, l *lobby.Lobby, id int, c *websocket.Conn) {
	p := wsPeer{c}
	var rate netlimit.RateWindow
	pong := make([]byte, wire.PingLen)
	for {
		rctx, cancel := context.WithTimeout(ctx, h.lobbyIdle)
		typ, b, err := c.Read(rctx)
		cancel()
		if err != nil {
			return
		}
		if !rate.AllowN(time.Now(), lobby.MaxMsgs) {
			p.Close(wire.CloseRate, "rate limit")
			return
		}
		if typ != websocket.MessageBinary {
			_ = c.Close(websocket.StatusUnsupportedData, "bad message")
			return
		}
		switch {
		case wire.IsPing(b):
			_ = p.Send(wire.EncodePong(pong, b))
		case wire.IsReportMsg(b):
			if name, hidden := l.Report(id, int(wire.F64(b, 0))); name != "" {
				h.logger.Printf("lobby report: %q (player %d) by player %d, hidden=%v", name, int(wire.F64(b, 0)), id, hidden)
			}
		case wire.IsNameMsg(b):
			l.SetName(id, names.Clean(string(b[wire.HeaderLen:])), time.Now())
		case len(b) > 0 && b[0] == wire.MsgLobbyStyle:
			if st, ok := wire.DecodeLobbyStyle(b); ok {
				l.SetLook(id, st, time.Now())
			}
		case len(b) > 0 && b[0] == wire.MsgEmote:
			if kind, ok := wire.DecodeEmote(b); ok {
				l.Emote(id, kind, time.Now())
			}
		default:
			pos, err := wire.DecodePresence(b)
			if err != nil {
				_ = c.Close(websocket.StatusUnsupportedData, "bad message")
				return
			}
			l.SetPos(id, pos)
		}
	}
}

func realLobbyTicker() (<-chan time.Time, func()) {
	t := time.NewTicker(lobby.Interval)
	return t.C, t.Stop
}
