// Package match is one online match room: seats, the authoritative sim loop,
// ranked bookkeeping, held seats for reconnection (rejoin.go) and the
// matchmaking moves of a waiting room (queue.go).
package match

import (
	"sync"
	"time"

	"github.com/txny34/futenis/cmd/server/internal/ranked"
	"github.com/txny34/futenis/cmd/server/internal/wire"
	"github.com/txny34/futenis/game"
	"github.com/txny34/futenis/sim"
)

const (
	FrameInterval   = time.Second / 60
	ticksPerFrame   = sim.TickRate / 60
	roomPointsToWin = 5
	// MinHumans2v2: a 2v2 room may start early (bots fill the rest) once this
	// many people are in; see hub.fillWait.
	MinHumans2v2 = 2
)

// Seat fill order. 2v2 teams are {0,1} vs {2,3} (sim/rules.go): the first two
// people become rivals, each with a bot partner until more arrive.
var (
	seats1v1 = []int{0, 1}
	seats2v2 = []int{0, 2, 1, 3}
)

// Peer is one connected browser; send must be safe for concurrent use.
type Peer interface {
	Send(b []byte) error
	Close(code int, reason string)
}

// Room is one online match: 1v1 (2 seats) or 2v2 (4 seats). Seat = sim slot;
// team = slot in 1v1, slot/2 in 2v2. Every client gets its own view
// (game.ViewOut). In 2v2 an empty seat is played by the bot, also when
// someone leaves mid-match.
type Room struct {
	mu      sync.Mutex
	mode    sim.Mode
	code    string // hub key (given to NewRoom)
	seats   int
	order   []int
	peers   [sim.MaxPlayers]Peer
	inputs  [sim.MaxPlayers]sim.PlayerInput
	stop    chan struct{}
	started bool
	closed  bool
	startAt time.Time // when the match began; the idle clock starts here

	ranked   bool                                           // ranked queue: no bot fill, ELO at game over
	users    [sim.MaxPlayers]string                         // verified account per seat (ranked only)
	nets     [sim.MaxPlayers]string                         // client limit key per seat (ranked only)
	onOver   func(winner int, users [sim.MaxPlayers]string) // set before run (ranked)
	reported bool                                           // ranked result already sent (game over or forfeit)

	tokens  [sim.MaxPlayers]string       // rejoin secret per seat (rejoin.go)
	away    [sim.MaxPlayers]*time.Timer  // seat held for a dropped connection; fires the leave
	holds   int                          // holds used in this room (capped: nobody stalls a match forever)
	over    bool                         // game over reached: no more holds
	claims  [sim.MaxPlayers]ranked.Claim // ranked reservation per seat
	release func(c ranked.Claim)         // frees a held seat's ranked account when the room closes (set by announce)
}

// claimReport returns true exactly once: the caller reports the ranked result.
func (r *Room) claimReport() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.reported {
		return false
	}
	r.reported = true
	return true
}

// TeamOf maps a seat to its team (1v1: the seat; 2v2: seat/2, sim/rules.go).
func (r *Room) TeamOf(slot int) int {
	if r.mode == sim.Mode2v2 {
		return slot / 2
	}
	return slot
}

// MarkRanked records slot's verified account and network (limit key); the
// room becomes ranked. It refuses an account or a network already seated:
// nobody plays against themselves, and two accounts from one network can't
// trade wins.
func (r *Room) MarkRanked(slot int, c ranked.Claim, network string) bool {
	user := c.UID
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.users {
		if i != slot && (r.users[i] == user || (r.nets[i] != "" && r.nets[i] == network)) {
			return false
		}
	}
	r.ranked, r.users[slot], r.nets[slot], r.claims[slot] = true, user, network, c
	return true
}

// NewRoom is an empty room of mode, known to the hub as code.
func NewRoom(mode sim.Mode, code string) *Room {
	order := seats1v1
	if mode == sim.Mode2v2 {
		order = seats2v2
	}
	return &Room{mode: mode, code: code, seats: len(order), order: order, stop: make(chan struct{})}
}

// SeatOrder is the fill order of mode's seats (a copy: rooms share the original).
func SeatOrder(mode sim.Mode) []int {
	if mode == sim.Mode2v2 {
		return append([]int(nil), seats2v2...)
	}
	return append([]int(nil), seats1v1...)
}

// Code is the hub key of the room; Mode and SeatCount never change either.
func (r *Room) Code() string   { return r.code }
func (r *Room) Mode() sim.Mode { return r.mode }
func (r *Room) SeatCount() int { return r.seats }

// Peers is a snapshot of who is seated (nil = empty or held seat).
func (r *Room) Peers() [sim.MaxPlayers]Peer {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.peers
}

// Claims is a snapshot of the seats' ranked reservations.
func (r *Room) Claims() [sim.MaxPlayers]ranked.Claim {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.claims
}

// SetRankedHooks wires the ELO report at game over and the release of held
// accounts when the room closes; a casual room ignores them. Call before run.
func (r *Room) SetRankedHooks(onOver func(winner int, users [sim.MaxPlayers]string), release func(ranked.Claim)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.ranked {
		r.onOver, r.release = onOver, release
	}
}

// Join adds p and returns its slot (-1 if full, closed or already started).
// start is true for exactly one caller: the one whose Join fills the room.
// humans is the head count after joining. A waiting joiner gets WAITING while
// the lock is held, so a concurrent WELCOME can never precede it.
func (r *Room) Join(p Peer) (slot int, start bool, humans int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed || r.started {
		return -1, false, 0
	}
	slot = -1
	for _, i := range r.order {
		if r.peers[i] == nil {
			r.peers[i] = p
			slot = i
			break
		}
	}
	if slot < 0 {
		return -1, false, 0
	}
	humans = r.humansLocked()
	if humans < r.seats {
		_ = p.Send(wire.EncodeSignal(wire.MsgWaiting))
		return slot, false, humans
	}
	return slot, r.startLocked(), humans
}

// StartEarly starts a 2v2 room that is not full yet (bots take the empty
// seats). It reports true for exactly one caller.
func (r *Room) StartEarly() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed || r.ranked || r.humansLocked() < MinHumans2v2 {
		return false
	}
	return r.startLocked()
}

func (r *Room) startLocked() bool {
	if r.started {
		return false
	}
	r.started = true
	r.startAt = time.Now()
	return true
}

func (r *Room) humansLocked() int {
	n := 0
	for _, p := range r.peers {
		if p != nil {
			n++
		}
	}
	return n
}

// humanMaskLocked has bit i set for every seated person.
func (r *Room) humanMaskLocked() int {
	m := 0
	for i, p := range r.peers {
		if p != nil {
			m |= 1 << i
		}
	}
	return m
}

// StartedAt reports when the match began (ok false while waiting).
func (r *Room) StartedAt() (t time.Time, ok bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.startAt, r.started
}

// ExpireIfWaiting closes the room iff the match never started, atomically
// with join (a rival arriving now is rejected instead of meeting a dead host).
func (r *Room) ExpireIfWaiting() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.started || r.closed || (r.mode == sim.Mode2v2 && r.humansLocked() >= MinHumans2v2) {
		return false // 2v2 with enough people: the fill timer starts it with bots
	}
	r.closed = true
	close(r.stop)
	return true
}

// IsClosed reports that the room ended (nobody can join or come back).
func (r *Room) IsClosed() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.closed
}

// SetInput stores slot's latest input from p (ignored once p was replaced),
// turned from the client's view into server space.
func (r *Room) SetInput(slot int, p Peer, in sim.PlayerInput) {
	in = game.ViewInput(in, slot, r.mode == sim.Mode2v2)
	r.mu.Lock()
	if r.peers[slot] == p {
		r.inputs[slot] = in
	}
	r.mu.Unlock()
}
