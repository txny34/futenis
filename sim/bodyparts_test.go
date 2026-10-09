package sim

import "testing"

func TestBestPartMatchesConstrainedWithoutBicycle(t *testing.T) {
	cfg := DefaultConfig()
	for _, serveOnly := range []bool{false, true} {
		for h := -0.2; h <= 2.6; h += 0.05 {
			for dH := 0.0; dH <= 1.4; dH += 0.1 {
				gp, gu := BestPart(&cfg, h, dH, serveOnly)
				wp, wu := bestPartConstrained(&cfg, h, dH, false, serveOnly)
				if gp != wp || gu != wu {
					t.Fatalf("h=%v dH=%v serveOnly=%v: got (%v,%v) want (%v,%v)", h, dH, serveOnly, gp, gu, wp, wu)
				}
			}
		}
	}
}
