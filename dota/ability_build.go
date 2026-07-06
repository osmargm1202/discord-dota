package dota

import (
	"fmt"
	"sort"
)

// AbilityPick is one skill-point event from Stratz's match ability log.
type AbilityPick struct {
	Name     string
	Time     int
	IsTalent bool
}

// AbilityBuildMatch is one match's ability pick log plus its outcome for
// the target player.
type AbilityBuildMatch struct {
	MatchID int64
	Win     bool
	// LaneOutcome is "won", "lost", "tied", or "" (no fixed lane / unknown).
	// See LaneOutcomeForPlayer.
	LaneOutcome string
	Picks       []AbilityPick
}

// BuildTuple counts skill points spent on Q, W, E, R.
type BuildTuple struct {
	Q, W, E, R int
}

// Label renders the tuple as "Q-W-E-R", e.g. "1-1-3-1".
func (t BuildTuple) Label() string {
	return fmt.Sprintf("%d-%d-%d-%d", t.Q, t.W, t.E, t.R)
}

// ComputeBuildTuple computes the Q/W/E/R skill-point tuple at the given
// hero level (valid for level 1-9 only). It returns ok=false when the
// match doesn't have enough non-talent picks to reach that level, or when
// a 5th distinct non-talent ability name appears (meaning the elimination
// rule used to detect R broke down for this hero).
func ComputeBuildTuple(picks []AbilityPick, level int, qName, wName, eName string) (BuildTuple, bool) {
	nonTalent := make([]AbilityPick, 0, len(picks))
	for _, p := range picks {
		if !p.IsTalent {
			nonTalent = append(nonTalent, p)
		}
	}
	sort.Slice(nonTalent, func(i, j int) bool { return nonTalent[i].Time < nonTalent[j].Time })

	if len(nonTalent) < level {
		return BuildTuple{}, false
	}

	var t BuildTuple
	rName := ""
	for _, p := range nonTalent[:level] {
		switch p.Name {
		case qName:
			t.Q++
		case wName:
			t.W++
		case eName:
			t.E++
		default:
			if rName == "" {
				rName = p.Name
				t.R++
			} else if p.Name == rName {
				t.R++
			} else {
				return BuildTuple{}, false
			}
		}
	}
	return t, true
}

// BuildGroup is one ability-build tuple's aggregated match record.
type BuildGroup struct {
	Tuple  BuildTuple
	Wins   int
	Losses int
	Total  int
	// Lane outcome counts contrast the build against how often the
	// player's own lane was won/lost/tied (independent of match result).
	LaneWins   int
	LaneLosses int
	LaneTies   int
}

// GroupBuildResults computes each match's build tuple and groups matches
// by tuple, sorted by Total descending (most-played build first).
func GroupBuildResults(matches []AbilityBuildMatch, level int, qName, wName, eName string) []BuildGroup {
	groups := make(map[BuildTuple]*BuildGroup)
	var order []BuildTuple

	for _, m := range matches {
		tuple, ok := ComputeBuildTuple(m.Picks, level, qName, wName, eName)
		if !ok {
			continue
		}
		g, exists := groups[tuple]
		if !exists {
			g = &BuildGroup{Tuple: tuple}
			groups[tuple] = g
			order = append(order, tuple)
		}
		g.Total++
		if m.Win {
			g.Wins++
		} else {
			g.Losses++
		}
		switch m.LaneOutcome {
		case "won":
			g.LaneWins++
		case "lost":
			g.LaneLosses++
		case "tied":
			g.LaneTies++
		}
	}

	result := make([]BuildGroup, 0, len(order))
	for _, t := range order {
		result = append(result, *groups[t])
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Total > result[j].Total })
	return result
}
