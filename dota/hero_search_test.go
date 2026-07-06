package dota

import "testing"

func TestFindHeroName(t *testing.T) {
	heroes := map[int]string{
		47: "Viper",
		8:  "Juggernaut",
		5:  "Crystal Maiden",
		20: "Vengeful Spirit",
	}

	t.Run("exact match", func(t *testing.T) {
		id, name, _, err := FindHeroName("viper", heroes)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if id != 47 || name != "Viper" {
			t.Errorf("got id=%d name=%q, want id=47 name=Viper", id, name)
		}
	})

	t.Run("substring match single result", func(t *testing.T) {
		id, name, _, err := FindHeroName("jugger", heroes)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if id != 8 || name != "Juggernaut" {
			t.Errorf("got id=%d name=%q, want id=8 name=Juggernaut", id, name)
		}
	})

	t.Run("ambiguous match", func(t *testing.T) {
		_, _, candidates, err := FindHeroName("v", heroes)
		if err == nil {
			t.Fatal("expected an error for an ambiguous query")
		}
		if len(candidates) < 2 {
			t.Fatalf("expected multiple candidates, got %v", candidates)
		}
	})

	t.Run("no match", func(t *testing.T) {
		_, _, candidates, err := FindHeroName("nonexistenthero", heroes)
		if err == nil {
			t.Fatal("expected an error for no match")
		}
		if len(candidates) != 0 {
			t.Errorf("expected no candidates, got %v", candidates)
		}
	})
}
