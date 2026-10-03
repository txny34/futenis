package sim

import "math"

// partError is the normalized contact error for one body part against a
// relative ball height/horizontal distance: 0 = perfect center, >=1 = out
// of that part's reach (docs/design/contact-v2.md §7.2).
func partError(ps *PartSpec, hRel, dH float64) float64 {
	if hRel < ps.HMin || hRel > ps.HMax || dH > ps.Reach {
		return 2
	}
	half := ps.HMax - ps.HIdeal
	if hRel < ps.HIdeal {
		half = ps.HIdeal - ps.HMin
	}
	if half == 0 {
		half = 1e-9 // degenerate table entry: avoid a division by zero
	}
	eh := math.Abs(hRel-ps.HIdeal) / half
	er := dH / ps.Reach
	return math.Max(eh, er)
}

// bestPart returns the body part with the lowest contact error for the
// given relative ball height/horizontal distance, and that error. Ties go
// to the lower-indexed part (deterministic). PartNone (err 2) means no part
// reaches (docs/design/contact-v2.md §6, §7.2).
func bestPart(cfg *Config, hRel, dH float64) (BodyPart, float64) {
	return bestPartConstrained(cfg, hRel, dH, true, false)
}

// bestPartConstrained is bestPart with the two eligibility gates strike.go's
// applyStrikes needs (docs/design/contact-v2.md §7.2, §7.5):
//
//   - bicycleEligible: PartBicycle only counts as reachable when the
//     player is airborne with a (near-)full charge; otherwise it is
//     skipped entirely so a lower-error grounded part wins instead of the
//     table simply reporting PartNone when only the bicycle's height band
//     would have matched.
//   - serveOnly: during a serve the padel rules restrict contact to foot
//     or knee ("golpear con pie o rodilla", below the waist); every part
//     above PartKnee is skipped.
func bestPartConstrained(cfg *Config, hRel, dH float64, bicycleEligible, serveOnly bool) (BodyPart, float64) {
	best := PartNone
	bestErr := 2.0
	for i := PartFoot; i < PartCount; i++ {
		if i == PartBicycle && !bicycleEligible {
			continue
		}
		if serveOnly && i > PartKnee {
			continue
		}
		u := partError(&cfg.Parts[i], hRel, dH)
		if u < bestErr {
			bestErr = u
			best = i
		}
	}
	return best, bestErr
}

// qualityOf buckets a normalized contact error into Perfect/Good/Forced
// (docs/design/contact-v2.md §8).
func qualityOf(c *Config, u float64) Quality {
	switch {
	case u <= c.QualPerfectMax:
		return QualPerfect
	case u <= c.QualGoodMax:
		return QualGood
	default:
		return QualForced
	}
}
