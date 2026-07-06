package dota

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type heroAbilitiesRaw struct {
	Abilities []string `json:"abilities"`
}

// parseHeroAbilitiesJSON extracts each hero's Q/W/E ability names (the
// first 3 entries of its "abilities" array) from OpenDota's
// constants/hero_abilities payload. R is deliberately not extracted here —
// see docs/superpowers/specs/2026-07-06-mystats-ability-build-design.md
// for why "last array entry" is not a reliable way to find the ultimate.
func parseHeroAbilitiesJSON(data []byte) (map[string][3]string, error) {
	var rawEntries map[string]json.RawMessage
	if err := json.Unmarshal(data, &rawEntries); err != nil {
		return nil, err
	}
	out := make(map[string][3]string, len(rawEntries))
	for heroName, entry := range rawEntries {
		var v heroAbilitiesRaw
		// Some heroes (e.g. Monkey King) have a non-string entry in their
		// abilities array (a facet/transformation swap pair encoded as a
		// nested array). Skip just that hero rather than failing the
		// whole batch — see docs/superpowers/specs/2026-07-06-mystats-ability-build-design.md.
		if err := json.Unmarshal(entry, &v); err != nil {
			continue
		}
		if len(v.Abilities) >= 3 {
			out[heroName] = [3]string{v.Abilities[0], v.Abilities[1], v.Abilities[2]}
		}
	}
	return out, nil
}

func (c *Client) loadHeroAbilitiesLocal() {
	if len(c.heroAbilitiesCache) > 0 {
		return
	}
	data, err := os.ReadFile(filepath.Join("dota", "hero_abilities.json"))
	if err != nil {
		return
	}
	parsed, err := parseHeroAbilitiesJSON(data)
	if err != nil {
		return
	}
	c.heroAbilitiesCache = parsed
}

// GetHeroQWE returns the internal Stratz/OpenDota ability names for a
// hero's Q, W, and E slots. ok is false if the hero id or its ability
// data isn't known.
func (c *Client) GetHeroQWE(heroID int) (q, w, e string, ok bool) {
	c.loadHeroesLocal()
	c.loadHeroAbilitiesLocal()
	slug, exists := c.heroSlugCache[heroID]
	if !exists || slug == "" {
		return "", "", "", false
	}
	qwe, exists2 := c.heroAbilitiesCache["npc_dota_hero_"+slug]
	if !exists2 {
		return "", "", "", false
	}
	return qwe[0], qwe[1], qwe[2], true
}
