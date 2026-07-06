package dota

import "strings"

// PlayerLaneSlot maps a Stratz lane enum ("SAFE_LANE", "OFF_LANE",
// "MID_LANE") plus the player's team to which physical lane (top/mid/bottom)
// they played, mirroring Dota's map layout (Radiant's safe lane is bottom,
// Dire's safe lane is top). Returns "" for jungle/roam/unknown.
func PlayerLaneSlot(lane string, isRadiant bool) string {
	switch strings.ToUpper(lane) {
	case "SAFE_LANE":
		if isRadiant {
			return "bottom"
		}
		return "top"
	case "OFF_LANE":
		if isRadiant {
			return "top"
		}
		return "bottom"
	case "MID_LANE":
		return "mid"
	default:
		return ""
	}
}

// LaneOutcomeForPlayer determines whether the player's team won, lost, or
// tied the lane they played, given the match's per-lane outcome strings
// (Stratz's topLaneOutcome/midLaneOutcome/bottomLaneOutcome, e.g.
// "RADIANT_VICTORY", "DIRE_STOMP", "TIE"). Returns "won", "lost", "tied", or
// "" when the player didn't play a fixed lane (jungle/roam) or the outcome
// is unknown.
func LaneOutcomeForPlayer(lane string, isRadiant bool, topOutcome, midOutcome, bottomOutcome string) string {
	var outcome string
	switch PlayerLaneSlot(lane, isRadiant) {
	case "top":
		outcome = topOutcome
	case "mid":
		outcome = midOutcome
	case "bottom":
		outcome = bottomOutcome
	default:
		return ""
	}

	switch strings.ToUpper(outcome) {
	case "TIE":
		return "tied"
	case "RADIANT_VICTORY", "RADIANT_STOMP":
		if isRadiant {
			return "won"
		}
		return "lost"
	case "DIRE_VICTORY", "DIRE_STOMP":
		if isRadiant {
			return "lost"
		}
		return "won"
	default:
		return ""
	}
}
