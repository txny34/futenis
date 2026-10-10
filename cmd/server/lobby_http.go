package main

import (
	"context"
	"net/http"
	"time"

	"github.com/coder/websocket"
)

// ServeLobby upgrades /lobby: same origin check and per-client caps as /ws.
func (h *hub) ServeLobby(w http.ResponseWriter, req *http.Request) {
	ip := limitKey(clientIP(req, h.proxied))
	if !h.conns.acquire(ip) {
		http.Error(w, "too many connections", http.StatusTooManyRequests)
		return
	}
	defer h.conns.release(ip)
	c, err := websocket.Accept(w, req, &websocket.AcceptOptions{OriginPatterns: h.origins})
	if err != nil {
		return // Accept already wrote the HTTP error
	}
	defer c.CloseNow()
	c.SetReadLimit(readLimit)
	l := h.lobbyRoom()
	p := wsPeer{c}
	id, ok := l.join(p)
	if !ok {
		p.close(closeFull, "lobby full")
		return
	}
	defer l.leave(id)
	h.lobbyLoop(req.Context(), l, id, c)
}

// lobbyRoom returns the shared lobby, starting its broadcast loop once.
func (h *hub) lobbyRoom() *lobby {
	h.lobbyOnce.Do(func() {
		h.lobby = newLobby()
		tick, stop := h.lobbyTicker()
		go func() { defer stop(); h.lobby.run(tick, h.done) }()
	})
	return h.lobby
}

// lobbyLoop reads PRESENCE / NAME / PING until the peer leaves, floods, sends
// garbage or stays silent for lobbyIdle (hidden tab).
func (h *hub) lobbyLoop(ctx context.Context, l *lobby, id int, c *websocket.Conn) {
	p := wsPeer{c}
	var rate rateWindow
	pong := make([]byte, pingMsgLen)
	for {
		rctx, cancel := context.WithTimeout(ctx, h.lobbyIdle)
		typ, b, err := c.Read(rctx)
		cancel()
		if err != nil {
			return
		}
		if !rate.allow(time.Now()) {
			p.close(closeRate, "rate limit")
			return
		}
		if typ != websocket.MessageBinary {
			_ = c.Close(websocket.StatusUnsupportedData, "bad message")
			return
		}
		switch {
		case isPing(b):
			_ = p.send(encodePong(pong, b))
		case isNameMsg(b):
			l.setName(id, sanitizeName(string(b[headerLen:])))
		default:
			pos, err := decodePresence(b)
			if err != nil {
				_ = c.Close(websocket.StatusUnsupportedData, "bad message")
				return
			}
			l.setPos(id, pos)
		}
	}
}

func realLobbyTicker() (<-chan time.Time, func()) {
	t := time.NewTicker(lobbyInterval)
	return t.C, t.Stop
}
