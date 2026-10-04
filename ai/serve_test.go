package ai

import (
	"testing"

	"github.com/txny34/futenis/sim"
)

// Side convention: team 0's right is +Z, team 1's right is -Z. Box 0 is the
// receiver's right; the server stands diagonally opposite (crosscourt).
func TestServeFollowsSideConvention(t *testing.T) {
	cases := []struct {
		server, box  int8
		standZ, aimZ float64 // signs
	}{
		{0, 0, 1, -1}, // team 0 serves from its right (+Z) to team 1's right (-Z)
		{0, 1, -1, 1}, // odd: both on their left
		{1, 0, -1, 1}, // team 1's right is -Z, receiver (team 0) right is +Z
		{1, 1, 1, -1},
	}
	for _, c := range cases {
		s := sim.NewMatch(sim.DefaultConfig()).State()
		s.Server, s.Rally.ServerPlayer, s.Rally.ServeBox = c.server, c.server, c.box
		_, z := serveSpot(&s, c.server)
		bot := New(Pro, int(c.server))
		bot.aimServe(&s, int(c.server))
		if z*c.standZ <= 0 || bot.aimZ*c.aimZ <= 0 {
			t.Errorf("server %d box %d: stand z=%.2f aim z=%.2f, want signs %v/%v", c.server, c.box, z, bot.aimZ, c.standZ, c.aimZ)
		}
	}
}

func TestReceiverWaitsInTargetBox1v1(t *testing.T) {
	s := sim.NewMatch(sim.DefaultConfig()).State()
	s.Server, s.Rally.ServerPlayer, s.Rally.ServeBox = 0, 0, 0
	s.Players[1].Pos = sim.Vec3{X: 7, Z: 1} // standing on +Z (its left)
	bot := New(Pro, 1)
	in := bot.serveWait(&s, 1)
	if in.MoveZ >= 0 {
		t.Fatalf("receiver must walk to its right (-Z) box, MoveZ=%.2f", in.MoveZ)
	}
}

func TestHomeSlotsStandOnOwnRight(t *testing.T) {
	cfg := sim.DefaultConfig()
	cfg.Mode = sim.Mode2v2
	s := sim.NewMatch(cfg).State()
	for i := 0; i < int(s.NumPlayers); i++ {
		p := &s.Players[i]
		_, z := home(&s, i)
		want := 1.0
		if p.Team == 1 {
			want = -1
		}
		if p.Slot == 1 {
			want = -want
		}
		if z*want <= 0 {
			t.Errorf("slot %d team %d (Slot %d): home z=%.2f, want sign %v", i, p.Team, p.Slot, z, want)
		}
	}
}
