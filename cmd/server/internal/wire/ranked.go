package wire

// Ranked queues (cmd/server/ranked.go): AUTH carries the Supabase session
// token; RESULT tells each player the ELO before and after the match.
const (
	MsgAuth   = 0x03 // client: token bytes
	MsgResult = 0x86 // server: ELO before, after
	ResultLen = HeaderLen + 2*8
)

func EncodeResult(before, after int) []byte {
	b := make([]byte, ResultLen)
	PutHeader(b, MsgResult)
	PutF64(b, 0, float64(before))
	PutF64(b, 1, float64(after))
	return b
}
