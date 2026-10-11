package ranked

// Claim is one ranked reservation of an account (the hub keeps one live
// claim per account); Gen tells a stale release from the live one.
type Claim struct {
	UID  string
	Gen  uint64
	Name string // public name for the leaderboard (cleaned; "" = keep the stored one)
}
