package main

import (
	"context"
	"regexp"
	"time"

	"github.com/coder/websocket"
	"github.com/txny34/futenis/cmd/server/internal/auth"
	"github.com/txny34/futenis/sim"
)

// Ranked queues (/ws?queue=ranked1v1|ranked2v2): the first message must be
// AUTH with the Supabase session token; the account (not the IP) is the
// queue identity; no bot fill; at game over the server computes and stores
// the new ELO and tells each player (RESULT).
const (
	msgAuth        = 0x03 // client: token bytes
	msgResult      = 0x86 // server: ELO before, after
	closeNoAuth    = 4006
	authWait       = 5 * time.Second
	authReadLimit  = 4096 // JWTs are larger than game messages
	resultMsgLen   = headerLen + 2*8
	ratingsTimeout = 10 * time.Second
)

var userID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// rankedModes maps a ranked queue to its ratings column.
var rankedModes = map[string]string{"ranked1v1": "1v1", "ranked2v2": "2v2"}

// authenticate reads the AUTH message and returns the verified account.
func (h *hub) authenticate(ctx context.Context, c *websocket.Conn) (auth.Account, bool) {
	if h.auth == nil || h.ratings == nil {
		return auth.Account{}, false // ranked not configured on this server
	}
	c.SetReadLimit(authReadLimit)
	defer c.SetReadLimit(readLimit)
	actx, cancel := context.WithTimeout(ctx, authWait)
	defer cancel()
	typ, b, err := c.Read(actx)
	if err != nil || typ != websocket.MessageBinary || len(b) <= headerLen || b[0] != msgAuth || b[1] != protoVersion {
		return auth.Account{}, false
	}
	acc, err := h.auth.Verify(string(b[headerLen:]))
	if err != nil || !userID.MatchString(acc.ID) {
		return auth.Account{}, false
	}
	return acc, true
}

// reportRanked stores the new ratings and sends each seated player a RESULT.
// Runs off the match loop (network I/O).
func (h *hub) reportRanked(mode string, r *room, users [sim.MaxPlayers]string, winner int) {
	teams := [2][]string{}
	for i := 0; i < r.seats; i++ {
		team := i
		if r.mode == sim.Mode2v2 {
			team = i / 2
		}
		teams[team] = append(teams[team], users[i])
	}
	ids := append(append([]string{}, teams[0]...), teams[1]...)
	if !distinct(ids) {
		h.logger.Printf("ranked %s: duplicate or missing player, result not rated", mode)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), ratingsTimeout)
	defer cancel()
	before, err := h.ratings.load(ctx, mode, ids)
	if err != nil {
		h.logger.Printf("ranked %s: %v", mode, err)
		return
	}
	after := applyElo(before, teams, winner)
	if err := h.ratings.save(ctx, mode, resultRows(r, users, after, winner)); err != nil {
		h.logger.Printf("ranked %s: %v", mode, err)
		if legacy, ok := err.(errLegacySave); !ok || legacy.err != nil {
			return // nothing saved: no RESULT
		}
	}
	r.mu.Lock()
	peers := r.peers
	r.mu.Unlock()
	for i, p := range peers {
		if p != nil && users[i] != "" {
			sendTo(p, encodeResult(ratingOf(before, users[i]), after[users[i]]))
		}
	}
}

// resultRows pairs every seated account with its new ELO, win flag and public name.
func resultRows(r *room, users [sim.MaxPlayers]string, after map[string]int, winner int) []rankedRow {
	r.mu.Lock()
	claims := r.claims
	r.mu.Unlock()
	rows := make([]rankedRow, 0, r.seats)
	for i := 0; i < r.seats; i++ {
		rows = append(rows, rankedRow{ID: users[i], Elo: after[users[i]], Won: r.teamOf(i) == winner, Name: claims[i].name})
	}
	return rows
}

func encodeResult(before, after int) []byte {
	b := make([]byte, resultMsgLen)
	putHeader(b, msgResult)
	putF64(b, 0, float64(before))
	putF64(b, 1, float64(after))
	return b
}

// gameOverWinner reports the winning team (server space) once the frame's
// current snapshot is in GameOver.
func gameOverWinner(out []float64) (int, bool) {
	cur := out[sim.SnapLen:]
	if int(cur[sim.SnapPhase]) != int(sim.PhaseGameOver) {
		return 0, false
	}
	if cur[sim.SnapSets1] > cur[sim.SnapSets0] {
		return 1, true
	}
	return 0, true
}

// claimRanked reserves the account's single ranked connection (queue through
// match end). It fails while another one is live: no self-matching, no
// parallel matches whose rating writes could overwrite each other.
// Each claim has its own generation: a late release from an old connection
// (replaced, or of a finished match) never frees the account's newer claim.
func (h *hub) claimRanked(acc auth.Account) (claim, bool) {
	uid := acc.ID
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, busy := h.active[uid]; busy {
		return claim{}, false
	}
	h.claimSeq++
	h.active[uid] = h.claimSeq
	return claim{uid: uid, gen: h.claimSeq, name: acc.Name}, true
}

// releaseRanked frees c if it is still the account's live claim ("" = none).
func (h *hub) releaseRanked(c claim) {
	h.mu.Lock()
	if c.uid != "" && h.active[c.uid] == c.gen {
		delete(h.active, c.uid)
	}
	h.mu.Unlock()
}

// claim is one ranked reservation of an account.
type claim struct {
	uid  string
	gen  uint64
	name string // public name for the leaderboard (cleaned; "" = keep the stored one)
}

// distinct reports whether every id appears once (defense in depth for reports).
func distinct(ids []string) bool {
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		if id == "" || seen[id] {
			return false
		}
		seen[id] = true
	}
	return true
}
