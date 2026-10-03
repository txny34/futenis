package sim

// OWNER: physics agent. Stubs define the contract; replace the bodies.

// movePlayers moves both players from inputs, clamped to their own half (cannot cross the net).
func movePlayers(s *State, inputs [2]PlayerInput) {}

// applyKicks handles Kick inputs: if the ball is within KickReach, set its velocity and emit EvKick.
// In PhaseServe only the server may kick, and the kick switches Phase to PhaseRally.
// Max 1 touch per side is enforced by rules (applyRules), not here.
func applyKicks(s *State, inputs [2]PlayerInput, ev []Event) []Event { return ev }

// stepBall integrates the ball one tick and resolves floor, wall and net collisions,
// appending EvBounceFloor / EvHitWall / EvHitNet events.
func stepBall(s *State, ev []Event) []Event { return ev }
