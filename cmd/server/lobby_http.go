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
	if !h.lobbyConns.acquire(ip) {
		http.Error(w, "too many lobby connections", http.StatusTooManyRequests)
		return
	}
	defer h.lobbyConns.release(ip)
	c, err := websocket.Accept(w, req, &websocket.AcceptOptions{OriginPatterns: h.origins})
	if err != nil {
		return // Accept already wrote the HTTP error
	}
	defer c.CloseNow()
	c.SetReadLimit(readLimit)
	p := wsPeer{c}
	l, id, ok := h.joinLobby(p)
	if !ok {
		p.close(closeFull, "lobby full")
		return
	}
	defer l.leave(id)
	h.lobbyLoop(req.Context(), l, id, c)
}

// joinLobby seats p in the first lobby with room, opening another (with its
// own broadcast loop) when all are full, up to maxLobbyShard. The join itself
// (which sends) runs outside the hub lock; a lobby that filled meanwhile is retried.
func (h *hub) joinLobby(p peer) (l *lobby, id int, ok bool) {
	for range 3 {
		if l = h.pickLobby(); l == nil {
			return nil, 0, false
		}
		if id, ok = l.join(p); ok {
			return l, id, true
		}
	}
	return nil, 0, false
}

func (h *hub) pickLobby() *lobby {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, l := range h.lobbies {
		if !l.full() {
			return l
		}
	}
	if len(h.lobbies) >= maxLobbyShard {
		return nil
	}
	l := newLobby()
	h.lobbies = append(h.lobbies, l)
	tick, stop := h.lobbyTicker()
	go func() { defer stop(); l.run(tick, h.done) }()
	return l
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
		if !rate.allowN(time.Now(), maxLobbyMsgs) {
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
			l.setName(id, sanitizeName(string(b[headerLen:])), time.Now())
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
