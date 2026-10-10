package main

import "math"

// Ranked ELO, computed only on the server. Teams play at the average of their
// players; every player of a team moves by the same delta.
const (
	eloK       = 32
	eloDefault = 1000
	eloFloor   = 100
)

// eloDelta is team a's rating change against team b.
func eloDelta(a, b float64, aWon bool) int {
	expected := 1 / (1 + math.Pow(10, (b-a)/400))
	score := 0.0
	if aWon {
		score = 1
	}
	return int(math.Round(eloK * (score - expected)))
}

// applyElo returns the new ratings of every player in teams after winner
// (0 or 1) won. ratings is not modified; missing players start at eloDefault.
func applyElo(ratings map[string]int, teams [2][]string, winner int) map[string]int {
	avg := func(ids []string) float64 {
		sum := 0
		for _, id := range ids {
			sum += ratingOf(ratings, id)
		}
		return float64(sum) / float64(len(ids))
	}
	a, b := avg(teams[0]), avg(teams[1])
	deltas := [2]int{eloDelta(a, b, winner == 0), eloDelta(b, a, winner == 1)}
	next := make(map[string]int, len(teams[0])+len(teams[1]))
	for team, ids := range teams {
		for _, id := range ids {
			next[id] = max(eloFloor, ratingOf(ratings, id)+deltas[team])
		}
	}
	return next
}

func ratingOf(r map[string]int, id string) int {
	if v, ok := r[id]; ok {
		return v
	}
	return eloDefault
}
