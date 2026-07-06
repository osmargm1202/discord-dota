package dota

import "testing"

func TestParseHeroAbilitiesJSON(t *testing.T) {
	data := []byte(`{
		"npc_dota_hero_viper": {
			"abilities": ["viper_poison_attack", "viper_nethertoxin", "viper_corrosive_skin", "viper_nose_dive", "viper_predator", "viper_viper_strike"]
		},
		"npc_dota_hero_axe": {
			"abilities": ["axe_berserkers_call", "axe_battle_hunger", "axe_counter_helix", "generic_hidden", "generic_hidden", "axe_culling_blade", "axe_one_man_army"]
		}
	}`)

	parsed, err := parseHeroAbilitiesJSON(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	viper, ok := parsed["npc_dota_hero_viper"]
	if !ok {
		t.Fatal("expected npc_dota_hero_viper to be present")
	}
	want := [3]string{"viper_poison_attack", "viper_nethertoxin", "viper_corrosive_skin"}
	if viper != want {
		t.Errorf("viper QWE = %v, want %v", viper, want)
	}

	axe, ok := parsed["npc_dota_hero_axe"]
	if !ok {
		t.Fatal("expected npc_dota_hero_axe to be present")
	}
	wantAxe := [3]string{"axe_berserkers_call", "axe_battle_hunger", "axe_counter_helix"}
	if axe != wantAxe {
		t.Errorf("axe QWE = %v, want %v", axe, wantAxe)
	}
}

// TestParseHeroAbilitiesJSON_SkipsMalformedHeroWithoutLosingOthers
// reproduces the real OpenDota payload bug found in production: Monkey
// King's abilities array contains a nested array (a facet/transformation
// swap pair) instead of a plain string, which breaks a naive whole-file
// unmarshal into map[string]heroAbilitiesRaw and previously discarded ALL
// 127 heroes' data (including Viper), not just Monkey King's.
func TestParseHeroAbilitiesJSON_SkipsMalformedHeroWithoutLosingOthers(t *testing.T) {
	data := []byte(`{
		"npc_dota_hero_viper": {
			"abilities": ["viper_poison_attack", "viper_nethertoxin", "viper_corrosive_skin", "viper_viper_strike"]
		},
		"npc_dota_hero_monkey_king": {
			"abilities": ["monkey_king_boundless_strike", "monkey_king_jingu_mastery", ["monkey_king_untransform", "monkey_king_transfiguration"], "monkey_king_wukongs_command"]
		}
	}`)

	parsed, err := parseHeroAbilitiesJSON(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	viper, ok := parsed["npc_dota_hero_viper"]
	if !ok {
		t.Fatal("expected npc_dota_hero_viper to still be present despite monkey_king's malformed entry")
	}
	want := [3]string{"viper_poison_attack", "viper_nethertoxin", "viper_corrosive_skin"}
	if viper != want {
		t.Errorf("viper QWE = %v, want %v", viper, want)
	}

	if _, ok := parsed["npc_dota_hero_monkey_king"]; ok {
		t.Error("expected npc_dota_hero_monkey_king to be skipped (malformed abilities entry), not present")
	}
}

func TestGetHeroQWE(t *testing.T) {
	c := NewClient()
	c.heroSlugCache[47] = "viper"
	c.heroAbilitiesCache = map[string][3]string{
		"npc_dota_hero_viper": {"viper_poison_attack", "viper_nethertoxin", "viper_corrosive_skin"},
	}

	q, w, e, ok := c.GetHeroQWE(47)
	if !ok {
		t.Fatal("expected ok=true for hero 47")
	}
	if q != "viper_poison_attack" || w != "viper_nethertoxin" || e != "viper_corrosive_skin" {
		t.Errorf("got q=%q w=%q e=%q", q, w, e)
	}

	_, _, _, ok = c.GetHeroQWE(9999)
	if ok {
		t.Error("expected ok=false for unknown hero id")
	}
}
