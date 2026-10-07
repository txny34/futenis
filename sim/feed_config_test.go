package sim

import "testing"

// --- docs/design/training.md §1: Config -------------------------------------

func TestFeedConfig_Defaults(t *testing.T) {
	c := DefaultConfig()
	f := []struct {
		name      string
		got, want float64
	}{
		{"MachineZMin", c.MachineZMin, -3}, {"MachineZMax", c.MachineZMax, 3},
		{"MachineXMin", c.MachineXMin, 8}, {"MachineXMax", c.MachineXMax, 9.5},
		{"FeedZMin", c.FeedZMin, -3.5}, {"FeedZMax", c.FeedZMax, 3.5},
		{"FeedXMin", c.FeedXMin, 4}, {"FeedXMax", c.FeedXMax, 8.5},
		{"FeedPowerMin", c.FeedPowerMin, 0.3}, {"FeedPowerMax", c.FeedPowerMax, 0.7},
		{"FeedQualityMin", c.FeedQualityMin, 0.7}, {"FeedQualityMax", c.FeedQualityMax, 1},
		{"FeedInterval", c.FeedInterval, 1.5},
	}
	for _, x := range f {
		if x.got != x.want {
			t.Errorf("%s: got %v, want %v", x.name, x.got, x.want)
		}
	}
	if c.FeedShots != FeedShotDrive || c.FeedOnDemand || c.FeedVolleyOnly || c.FeedValidBounce != 1 {
		t.Errorf("flags: shots=%d onDemand=%v volley=%v bounce=%d", c.FeedShots, c.FeedOnDemand, c.FeedVolleyOnly, c.FeedValidBounce)
	}
	if c.feedIntervalTicks != ticksOf(1.5) {
		t.Errorf("feedIntervalTicks: got %d, want %d", c.feedIntervalTicks, ticksOf(1.5))
	}
}

func TestFeedConfig_SanitizeClampsAndSwaps(t *testing.T) {
	c := DefaultConfig()
	c.MachineZMin, c.MachineZMax = 9, -9 // out of range AND swapped
	c.MachineXMin, c.MachineXMax = 0, 99
	c.FeedZMin, c.FeedZMax = 2, -1
	c.FeedXMin, c.FeedXMax = -5, 50
	c.FeedPowerMin, c.FeedPowerMax = 0.9, 0.1
	c.FeedQualityMin, c.FeedQualityMax = -1, 2
	c.FeedInterval = 99
	c.FeedShots = 0
	c.FeedValidBounce = 9
	c.Sanitize()

	checks := []struct {
		name      string
		got, want float64
	}{
		{"MachineZMin", c.MachineZMin, -4.7}, {"MachineZMax", c.MachineZMax, 4.7},
		{"MachineXMin", c.MachineXMin, 2}, {"MachineXMax", c.MachineXMax, 9.7},
		{"FeedZMin", c.FeedZMin, -1}, {"FeedZMax", c.FeedZMax, 2},
		{"FeedXMin", c.FeedXMin, 1}, {"FeedXMax", c.FeedXMax, 9.7},
		{"FeedPowerMin", c.FeedPowerMin, 0.1}, {"FeedPowerMax", c.FeedPowerMax, 0.9},
		{"FeedQualityMin", c.FeedQualityMin, 0}, {"FeedQualityMax", c.FeedQualityMax, 1},
		{"FeedInterval", c.FeedInterval, 5},
	}
	for _, x := range checks {
		if x.got != x.want {
			t.Errorf("%s: got %v, want %v", x.name, x.got, x.want)
		}
	}
	if c.FeedShots != FeedShotDrive {
		t.Errorf("FeedShots 0 -> got %d, want 1", c.FeedShots)
	}
	if c.FeedValidBounce != 2 {
		t.Errorf("FeedValidBounce 9 -> got %d, want 2", c.FeedValidBounce)
	}
	c.FeedShots, c.FeedValidBounce, c.FeedInterval = 0xFF, 0, 0
	c.Sanitize()
	if c.FeedShots != 7 || c.FeedValidBounce != 1 || c.FeedInterval != 0.5 {
		t.Errorf("shots=%d bounce=%d interval=%v, want 7/1/0.5", c.FeedShots, c.FeedValidBounce, c.FeedInterval)
	}
}

func TestFeedConfig_LayoutAppendedAfterFirstServer(t *testing.T) {
	if cfgMachineZMin != cfgFirstServer+1 || cfgFeedValidBounce != cfgLobbyHoldTime-1 {
		t.Fatalf("feed fields must be appended after cfgFirstServer and end the scalar block")
	}
	want := DefaultConfig()
	want.Mode = ModePlayground
	want.MachineZMin, want.MachineZMax = -1, 2
	want.MachineXMin, want.MachineXMax = 3, 4
	want.FeedZMin, want.FeedZMax = -2, 1
	want.FeedXMin, want.FeedXMax = 2, 5
	want.FeedShots = FeedShotLob | FeedShotSmash
	want.FeedPowerMin, want.FeedPowerMax = 0.2, 0.4
	want.FeedQualityMin, want.FeedQualityMax = 0.5, 0.6
	want.FeedOnDemand, want.FeedVolleyOnly = true, true
	want.FeedInterval = 2.5
	want.FeedValidBounce = 2
	want.Sanitize()

	buf := make([]float64, CfgLen)
	WriteConfig(&want, buf)
	if got := ReadConfig(buf); got != want {
		t.Fatalf("round trip mismatch:\n got %+v\nwant %+v", got, want)
	}
}
