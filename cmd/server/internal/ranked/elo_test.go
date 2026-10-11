package ranked

import "testing"

func TestEloDelta(t *testing.T) {
	for _, c := range []struct {
		a, b float64
		aWon bool
		want int
	}{
		{1000, 1000, true, 16}, // even match: half of K
		{1000, 1000, false, -16},
		{1400, 1000, true, 3},  // favourite wins: small gain
		{1000, 1400, true, 29}, // upset: big gain
		{1400, 1000, false, -29},
	} {
		if got := eloDelta(c.a, c.b, c.aWon); got != c.want {
			t.Errorf("%v vs %v won=%v: got %d want %d", c.a, c.b, c.aWon, got, c.want)
		}
	}
}

func TestApplyElo2v2UsesTeamAverages(t *testing.T) {
	r := map[string]int{"a": 1200, "b": 800, "c": 1000, "d": 1000}
	next := ApplyElo(r, [2][]string{{"a", "b"}, {"c", "d"}}, 0)
	if next["a"]-1200 != 16 || next["b"]-800 != 16 || next["c"]-1000 != -16 {
		t.Fatalf("got %v", next)
	}
	if r["a"] != 1200 {
		t.Fatal("input mutated")
	}
}
