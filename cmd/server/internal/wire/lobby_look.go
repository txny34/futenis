package wire

// Lobby looks and emotes (mirrors client/src/net/lobby-proto.ts): a client
// sends its own look (LOBBY_STYLE, same body as STYLE) and emotes; the server
// relays them with the sender's id. Bytes are opaque to the server except the
// emote kind, which is bounded.
const (
	MsgLobbyStyle = 0x13 // client: look (StyleLen bytes, v1 8)
	MsgEmote      = 0x14 // client: emote kind byte
	MsgLobbyLook  = 0x94 // server: f64 id, then the look
	MsgLobbyEmote = 0x95 // server: f64 id, then the emote kind byte

	EmoteKinds    = 4 // wave, cheer, dance, sad (client/src/lobby/emotes.ts)
	EmoteLen      = HeaderLen + 1
	LobbyLookLen  = HeaderLen + 8 + StyleLen
	LobbyEmoteLen = HeaderLen + 8 + 1
)

// DecodeLobbyStyle parses a LOBBY_STYLE (marked present).
func DecodeLobbyStyle(b []byte) (Style, bool) {
	if len(b) < HeaderLen || b[0] != MsgLobbyStyle {
		return Style{}, false
	}
	return styleBody(b)
}

// DecodeEmote parses an EMOTE; ok is false for an unknown kind.
func DecodeEmote(b []byte) (kind byte, ok bool) {
	if len(b) != EmoteLen || b[0] != MsgEmote || b[1] != Version || b[HeaderLen] >= EmoteKinds {
		return 0, false
	}
	return b[HeaderLen], true
}

func EncodeLobbyLook(id int, st Style) []byte {
	b := make([]byte, LobbyLookLen)
	PutHeader(b, MsgLobbyLook)
	PutF64(b, 0, float64(id))
	copy(b[HeaderLen+8:], st[:])
	return b
}

func EncodeLobbyEmote(id int, kind byte) []byte {
	b := make([]byte, LobbyEmoteLen)
	PutHeader(b, MsgLobbyEmote)
	PutF64(b, 0, float64(id))
	b[HeaderLen+8] = kind
	return b
}
