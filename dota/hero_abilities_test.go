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
