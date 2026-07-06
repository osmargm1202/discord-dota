package dota

import "testing"

// Real chronological pick log from Stratz match 8883621088 (Fifrex, Viper),
// captured during the design research session. Q=viper_poison_attack,
// W=viper_nethertoxin, E=viper_corrosive_skin.
func viperMatchPicks() []AbilityPick {
	return []AbilityPick{
		{Name: "viper_poison_attack", Time: -89},
		{Name: "viper_corrosive_skin", Time: 197},
		{Name: "viper_corrosive_skin", Time: 420},
		{Name: "viper_nethertoxin", Time: 544},
		{Name: "viper_corrosive_skin", Time: 653},
		{Name: "viper_viper_strike", Time: 855},
		{Name: "viper_corrosive_skin", Time: 894},
		{Name: "viper_nethertoxin", Time: 980},
		{Name: "viper_poison_attack", Time: 1075},
		{Name: "viper_poison_attack", Time: 1369},
		{Name: "special_bonus_unique_viper_4", Time: 1423, IsTalent: true},
		{Name: "viper_nethertoxin", Time: 1598},
		{Name: "viper_viper_strike", Time: 1910},
		{Name: "viper_nethertoxin", Time: 2168},
		{Name: "viper_poison_attack", Time: 2353},
		{Name: "special_bonus_unique_viper_7", Time: 2353, IsTalent: true},
		{Name: "viper_viper_strike", Time: 2724},
	}
}

const (
	viperQ = "viper_poison_attack"
	viperW = "viper_nethertoxin"
	viperE = "viper_corrosive_skin"
)

func TestComputeBuildTuple_ViperLevel6(t *testing.T) {
	tuple, ok := ComputeBuildTuple(viperMatchPicks(), 6, viperQ, viperW, viperE)
	if !ok {
		t.Fatal("expected ok=true")
	}
	want := BuildTuple{Q: 1, W: 1, E: 3, R: 1}
	if tuple != want {
		t.Errorf("got %+v, want %+v", tuple, want)
	}
	if tuple.Label() != "1-1-3-1" {
		t.Errorf("Label() = %q, want 1-1-3-1", tuple.Label())
	}
}

func TestComputeBuildTuple_TalentsExcludedFromLevelCount(t *testing.T) {
	// A talent picked between skill points 2 and 3 must not count toward
	// the level index — level 6 should still require 6 non-talent picks.
	picks := []AbilityPick{
		{Name: viperQ, Time: 0},
		{Name: viperE, Time: 1},
		{Name: "special_bonus_unique_viper_4", Time: 2, IsTalent: true},
		{Name: viperE, Time: 3},
		{Name: viperW, Time: 4},
		{Name: viperE, Time: 5},
		{Name: "viper_viper_strike", Time: 6},
	}
	tuple, ok := ComputeBuildTuple(picks, 6, viperQ, viperW, viperE)
	if !ok {
		t.Fatal("expected ok=true")
	}
	want := BuildTuple{Q: 1, W: 1, E: 3, R: 1}
	if tuple != want {
		t.Errorf("got %+v, want %+v", tuple, want)
	}
}

func TestComputeBuildTuple_IncompleteMatch(t *testing.T) {
	picks := []AbilityPick{{Name: viperQ, Time: 0}}
	_, ok := ComputeBuildTuple(picks, 6, viperQ, viperW, viperE)
	if ok {
		t.Fatal("expected ok=false when fewer than `level` non-talent picks exist")
	}
}

func TestComputeBuildTuple_FifthDistinctNameBreaksElimination(t *testing.T) {
	picks := []AbilityPick{
		{Name: "hero_q", Time: 0},
		{Name: "hero_w", Time: 1},
		{Name: "hero_e", Time: 2},
		{Name: "hero_r", Time: 3},
		{Name: "hero_extra_shard_ability", Time: 4},
	}
	_, ok := ComputeBuildTuple(picks, 5, "hero_q", "hero_w", "hero_e")
	if ok {
		t.Fatal("expected ok=false when a 5th distinct non-talent ability name appears")
	}
}

func TestGroupBuildResults(t *testing.T) {
	build1113 := viperMatchPicks() // 1-1-3-1 at level 6
	build3111 := []AbilityPick{
		{Name: viperQ, Time: 0},
		{Name: viperQ, Time: 1},
		{Name: viperQ, Time: 2},
		{Name: viperW, Time: 3},
		{Name: viperE, Time: 4},
		{Name: "viper_viper_strike", Time: 5},
	}

	matches := []AbilityBuildMatch{
		{MatchID: 1, Win: true, Picks: build1113},
		{MatchID: 2, Win: false, Picks: build1113},
		{MatchID: 3, Win: true, Picks: build3111},
	}

	groups := GroupBuildResults(matches, 6, viperQ, viperW, viperE)
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d: %+v", len(groups), groups)
	}

	// Sorted by Total desc: the 1-1-3-1 group (2 matches) comes first.
	if groups[0].Tuple.Label() != "1-1-3-1" {
		t.Errorf("groups[0].Tuple.Label() = %q, want 1-1-3-1", groups[0].Tuple.Label())
	}
	if groups[0].Total != 2 || groups[0].Wins != 1 || groups[0].Losses != 1 {
		t.Errorf("groups[0] = %+v, want Total=2 Wins=1 Losses=1", groups[0])
	}

	if groups[1].Tuple.Label() != "3-1-1-1" {
		t.Errorf("groups[1].Tuple.Label() = %q, want 3-1-1-1", groups[1].Tuple.Label())
	}
	if groups[1].Total != 1 || groups[1].Wins != 1 || groups[1].Losses != 0 {
		t.Errorf("groups[1] = %+v, want Total=1 Wins=1 Losses=0", groups[1])
	}
}
