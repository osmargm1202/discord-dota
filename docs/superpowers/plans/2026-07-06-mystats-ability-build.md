# /dota mystats — Ability Build Win-Rate Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `/dota mystats hero:<texto> level:<1-9> jugador:<@opcional>` — a Discord slash command that renders a PNG card grouping a player's matches with a given hero by the exact Q/W/E/R skill-point allocation at a chosen hero level (1-9), with win/loss record per group.

**Architecture:** New pure Go helpers in the `dota` package compute the Q/W/E/R tuple per match from Stratz's ability pick log (elimination rule — see spec) and group results; a new Stratz query method fetches all of a player's matches for one hero since `BaseYear` in one paginated call; a new PNG renderer in `internal/ranking` follows the existing `RenderPlayerStats` visual style; a new Discord subcommand wires it together.

**Tech Stack:** Go 1.25, discordgo, `github.com/fogleman/gg` (canvas), Stratz GraphQL API, OpenDota REST constants (vendored JSON).

**Spec:** `docs/superpowers/specs/2026-07-06-mystats-ability-build-design.md` (read this first — it documents the verified Stratz/OpenDota facts and the elimination rule this plan implements).

---

### Task 1: Vendor OpenDota's hero_abilities.json

**Files:**
- Create: `dota/hero_abilities.json`

- [ ] **Step 1: Download the file**

Run:
```bash
curl -s "https://api.opendota.com/api/constants/hero_abilities" -o /home/osmarg/Code/discord-dota/dota/hero_abilities.json
```

- [ ] **Step 2: Verify it has the expected shape**

Run:
```bash
python3 -c "
import json
d = json.load(open('/home/osmarg/Code/discord-dota/dota/hero_abilities.json'))
assert d['npc_dota_hero_viper']['abilities'][:3] == ['viper_poison_attack', 'viper_nethertoxin', 'viper_corrosive_skin']
assert d['npc_dota_hero_juggernaut']['abilities'][:3] == ['juggernaut_blade_fury', 'juggernaut_healing_ward', 'juggernaut_blade_dance']
print('OK', len(d), 'heroes')
"
```
Expected: `OK 126 heroes` (or similar count), no assertion error.

- [ ] **Step 3: Commit**

```bash
cd /home/osmarg/Code/discord-dota
git add dota/hero_abilities.json
git commit -m "Vendor OpenDota hero_abilities.json for Q/W/E slot mapping"
```

---

### Task 2: Hero Q/W/E slot lookup (`dota/api.go`)

**Files:**
- Modify: `dota/api.go`
- Test: `dota/hero_abilities_test.go` (new)

- [ ] **Step 1: Write the failing test**

Create `dota/hero_abilities_test.go`:

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/osmarg/Code/discord-dota && go test ./dota/... -run 'TestParseHeroAbilitiesJSON|TestGetHeroQWE' -v`
Expected: FAIL — `parseHeroAbilitiesJSON`, `c.heroAbilitiesCache`, and `GetHeroQWE` are undefined.

- [ ] **Step 3: Add `heroAbilitiesCache` field to the `Client` struct**

In `dota/api.go`, find the `Client` struct (around line 22-29):

```go
type Client struct {
	httpClient      *http.Client
	heroesCache     map[int]string
	heroImages      map[int]string
	heroSlugCache   map[int]string
	gameModes       map[int]string
	lobbyTypes      map[int]string
```

Add `heroAbilitiesCache map[string][3]string` as a new field right after `heroSlugCache`:

```go
type Client struct {
	httpClient         *http.Client
	heroesCache        map[int]string
	heroImages         map[int]string
	heroSlugCache      map[int]string
	heroAbilitiesCache map[string][3]string
	gameModes          map[int]string
	lobbyTypes         map[int]string
```

Find `NewClient` (around line 36-41) and add the initialization:

```go
		heroesCache:   make(map[int]string),
		heroImages:    make(map[int]string),
		heroSlugCache: make(map[int]string),
		gameModes:     make(map[int]string),
		lobbyTypes:    make(map[int]string),
```

becomes:

```go
		heroesCache:        make(map[int]string),
		heroImages:         make(map[int]string),
		heroSlugCache:      make(map[int]string),
		heroAbilitiesCache: make(map[string][3]string),
		gameModes:          make(map[int]string),
		lobbyTypes:         make(map[int]string),
```

- [ ] **Step 4: Create `dota/hero_abilities.go` with the loader and lookup**

```go
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
	var raw map[string]heroAbilitiesRaw
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	out := make(map[string][3]string, len(raw))
	for heroName, v := range raw {
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
```

- [ ] **Step 5: Run test to verify it passes**

Run: `cd /home/osmarg/Code/discord-dota && go test ./dota/... -run 'TestParseHeroAbilitiesJSON|TestGetHeroQWE' -v`
Expected: PASS (both tests).

- [ ] **Step 6: Commit**

```bash
cd /home/osmarg/Code/discord-dota
git add dota/api.go dota/hero_abilities.go dota/hero_abilities_test.go
git commit -m "Add hero Q/W/E ability slot lookup backed by vendored OpenDota data"
```

---

### Task 3: Hero fuzzy-name matching (`dota/api.go`)

**Files:**
- Modify: `dota/api.go`
- Test: `dota/hero_search_test.go` (new)

- [ ] **Step 1: Write the failing test**

Create `dota/hero_search_test.go`:

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/osmarg/Code/discord-dota && go test ./dota/... -run TestFindHeroName -v`
Expected: FAIL — `FindHeroName` is undefined.

- [ ] **Step 3: Create `dota/hero_search.go`**

```go
package dota

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
)

func normalizeHeroQuery(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// FindHeroName does a fuzzy lookup of a hero by (partial) localized name.
// It returns the matched hero id/name on a single hit, or a list of
// candidate names (with a non-nil error) when zero or multiple heroes
// match.
func FindHeroName(query string, heroes map[int]string) (heroID int, name string, candidates []string, err error) {
	nq := normalizeHeroQuery(query)
	if nq == "" {
		return 0, "", nil, fmt.Errorf("nombre de héroe vacío")
	}

	for id, n := range heroes {
		if normalizeHeroQuery(n) == nq {
			return id, n, nil, nil
		}
	}

	var matchIDs []int
	var matchNames []string
	for id, n := range heroes {
		if strings.Contains(normalizeHeroQuery(n), nq) {
			matchIDs = append(matchIDs, id)
			matchNames = append(matchNames, n)
		}
	}

	switch len(matchIDs) {
	case 0:
		return 0, "", nil, fmt.Errorf("no encontré ningún héroe que coincida con %q", query)
	case 1:
		return matchIDs[0], matchNames[0], nil, nil
	default:
		sort.Strings(matchNames)
		return 0, "", matchNames, fmt.Errorf("varios héroes coinciden con %q", query)
	}
}

// FindHeroByName resolves a hero by fuzzy name match against the client's
// loaded hero cache.
func (c *Client) FindHeroByName(query string) (heroID int, name string, candidates []string, err error) {
	c.loadHeroesLocal()
	return FindHeroName(query, c.heroesCache)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /home/osmarg/Code/discord-dota && go test ./dota/... -run TestFindHeroName -v`
Expected: PASS (all 4 subtests).

- [ ] **Step 5: Commit**

```bash
cd /home/osmarg/Code/discord-dota
git add dota/hero_search.go dota/hero_search_test.go
git commit -m "Add fuzzy hero-name lookup for /dota mystats"
```

---

### Task 4: Ability-build tuple computation and grouping (`dota/ability_build.go`)

**Files:**
- Create: `dota/ability_build.go`
- Test: `dota/ability_build_test.go` (new)

This task implements the core logic from the spec's "Build computation" section, using the real Viper match `8883621088` pick log captured during research as the primary fixture.

- [ ] **Step 1: Write the failing tests**

Create `dota/ability_build_test.go`:

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/osmarg/Code/discord-dota && go test ./dota/... -run 'TestComputeBuildTuple|TestGroupBuildResults' -v`
Expected: FAIL — `AbilityPick`, `BuildTuple`, `ComputeBuildTuple`, `AbilityBuildMatch`, `GroupBuildResults` are undefined.

- [ ] **Step 3: Create `dota/ability_build.go`**

```go
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
	Picks   []AbilityPick
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
// hero level (valid for level 1-9 only — see spec). It returns ok=false
// when the match doesn't have enough non-talent picks to reach that level,
// or when a 5th distinct non-talent ability name appears (meaning the
// elimination rule used to detect R broke down for this hero — see
// docs/superpowers/specs/2026-07-06-mystats-ability-build-design.md).
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
	}

	result := make([]BuildGroup, 0, len(order))
	for _, t := range order {
		result = append(result, *groups[t])
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Total > result[j].Total })
	return result
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /home/osmarg/Code/discord-dota && go test ./dota/... -run 'TestComputeBuildTuple|TestGroupBuildResults' -v`
Expected: PASS (all 5 tests).

- [ ] **Step 5: Run the full `dota` package test suite**

Run: `cd /home/osmarg/Code/discord-dota && go test ./dota/... -v`
Expected: PASS (all tests from Tasks 2, 3, 4).

- [ ] **Step 6: Commit**

```bash
cd /home/osmarg/Code/discord-dota
git add dota/ability_build.go dota/ability_build_test.go
git commit -m "Add ability-build tuple computation and grouping for mystats"
```

---

### Task 5: Stratz query for a player's hero matches with ability data (`dota/stratz.go`)

**Files:**
- Modify: `dota/stratz.go`

No unit test in this task: every other Stratz query method in this file (`GetMatch`, `GetPlayerMatchesForBackfill`, etc.) is untested at the HTTP layer — there's no mocking scaffolding in this codebase for `StratzClient`. This method is verified instead by the manual Discord check in Task 8, plus the fixture in Task 4 already validates the parsing logic it depends on downstream.

- [ ] **Step 1: Add `GetPlayerHeroAbilityBuilds` to `dota/stratz.go`**

Add this method after `GetPlayerMatchesForBackfill` (after line 983, right before the `SearchPlayers` function):

```go
// GetPlayerHeroAbilityBuilds fetches every match a player has played on a
// given hero since afterUnix, including each match's ability pick log.
// Paginated the same way as GetPlayerMatchesForBackfill.
func (c *StratzClient) GetPlayerHeroAbilityBuilds(steamAccountID int64, heroID int, afterUnix int64) ([]AbilityBuildMatch, error) {
	const pageSize = 100
	var out []AbilityBuildMatch
	skip := 0

	query := `
		query GetPlayerHeroAbilityBuilds($steamAccountId: Long!, $heroId: Short!, $after: Long!, $take: Int!, $skip: Int!) {
			player(steamAccountId: $steamAccountId) {
				matches(request: { heroIds: [$heroId], startDateTime: $after, take: $take, skip: $skip }) {
					id
					didRadiantWin
					players(steamAccountId: $steamAccountId) {
						isRadiant
						abilities {
							time
							isTalent
							abilityType { name }
						}
					}
				}
			}
		}
	`

	for {
		var result struct {
			Player struct {
				Matches []struct {
					ID            int64 `json:"id"`
					DidRadiantWin bool  `json:"didRadiantWin"`
					Players       []struct {
						IsRadiant bool `json:"isRadiant"`
						Abilities []struct {
							Time        int  `json:"time"`
							IsTalent    bool `json:"isTalent"`
							AbilityType struct {
								Name string `json:"name"`
							} `json:"abilityType"`
						} `json:"abilities"`
					} `json:"players"`
				} `json:"matches"`
			} `json:"player"`
		}

		if err := c.makeRequest(query, map[string]interface{}{
			"steamAccountId": steamAccountID,
			"heroId":         heroID,
			"after":          afterUnix,
			"take":           pageSize,
			"skip":           skip,
		}, &result); err != nil {
			return nil, err
		}

		if len(result.Player.Matches) == 0 {
			break
		}

		for _, m := range result.Player.Matches {
			if len(m.Players) == 0 {
				continue
			}
			p := m.Players[0]
			picks := make([]AbilityPick, 0, len(p.Abilities))
			for _, a := range p.Abilities {
				picks = append(picks, AbilityPick{
					Name:     a.AbilityType.Name,
					Time:     a.Time,
					IsTalent: a.IsTalent,
				})
			}
			out = append(out, AbilityBuildMatch{
				MatchID: m.ID,
				Win:     m.DidRadiantWin == p.IsRadiant,
				Picks:   picks,
			})
		}

		if len(result.Player.Matches) < pageSize {
			break
		}
		skip += pageSize
	}

	return out, nil
}
```

- [ ] **Step 2: Verify the package builds**

Run: `cd /home/osmarg/Code/discord-dota && go build ./...`
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
cd /home/osmarg/Code/discord-dota
git add dota/stratz.go
git commit -m "Add GetPlayerHeroAbilityBuilds Stratz query for mystats"
```

---

### Task 6: PNG renderer (`internal/ranking/image_mystats.go`)

**Files:**
- Create: `internal/ranking/image_mystats.go`

- [ ] **Step 1: Create the renderer**

```go
package ranking

import (
	"bytes"
	"fmt"

	"github.com/fogleman/gg"
)

// BuildGroupRow is one ability-build group's record for a player+hero+level combo.
type BuildGroupRow struct {
	Label  string // e.g. "1-1-3-1"
	Wins   int
	Losses int
	Draws  int
	Total  int
}

// MyHeroStatsRenderData holds data for the /dota mystats PNG card.
type MyHeroStatsRenderData struct {
	PlayerName     string
	AvatarBytes    []byte
	HeroName       string
	HeroImageBytes []byte
	Level          int
	Groups         []BuildGroupRow // sorted by Total desc
	TotalGames     int
}

const (
	myStatsHeaderH = 76.0
	myStatsColHeadH = 28.0
	myStatsRowH     = 26.0
	myStatsFooterH  = 28.0
	myStatsPadV     = 10.0
)

// RenderMyHeroStats generates a PNG showing win/loss record grouped by
// ability-point allocation (Q-W-E-R) at a given hero level.
func (g *ImageGenerator) RenderMyHeroStats(d MyHeroStatsRenderData) ([]byte, error) {
	const w = canvasW
	totalH := int(myStatsHeaderH + myStatsPadV + myStatsColHeadH + float64(len(d.Groups))*myStatsRowH + myStatsPadV + myStatsFooterH)

	dc := gg.NewContext(w, totalH)
	dc.SetColor(colorBG)
	dc.Clear()

	g.drawMyStatsHeader(dc, d, myStatsHeaderH)
	g.drawBuildGroupTable(dc, d.Groups, myStatsHeaderH+myStatsPadV, myStatsColHeadH, myStatsRowH)

	fy := float64(totalH) - myStatsFooterH
	dc.SetColor(colorPanel)
	dc.DrawRectangle(0, fy, w, myStatsFooterH)
	dc.Fill()
	g.loadFont(dc, 10)
	dc.SetColor(colorGray)
	footer := fmt.Sprintf("%d partidas analizadas  •  %s  •  Nivel %d  •  Stratz", d.TotalGames, d.HeroName, d.Level)
	dc.DrawStringAnchored(footer, w/2, fy+myStatsFooterH/2, 0.5, 0.5)

	var buf bytes.Buffer
	if err := dc.EncodePNG(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (g *ImageGenerator) drawMyStatsHeader(dc *gg.Context, d MyHeroStatsRenderData, h float64) {
	dc.SetColor(colorPanel)
	dc.DrawRectangle(0, 0, canvasW, h)
	dc.Fill()

	const cr = 26.0
	cx, cy := 40.0, h/2

	if len(d.AvatarBytes) > 0 {
		if img, _, err := decodeImage(d.AvatarBytes); err == nil {
			dc.DrawCircle(cx, cy, cr)
			dc.Clip()
			scaled := scaleImage(img, int(cr*2), int(cr*2))
			dc.DrawImageAnchored(scaled, int(cx), int(cy), 0.5, 0.5)
			dc.ResetClip()
		}
	} else {
		dc.SetColor(colorGold)
		dc.DrawCircle(cx, cy, cr)
		dc.Fill()
	}
	dc.SetColor(colorGold)
	dc.SetLineWidth(1.5)
	dc.DrawCircle(cx, cy, cr)
	dc.Stroke()

	const hs = 52.0
	hx, hy := cx+cr+20, cy-hs/2
	if len(d.HeroImageBytes) > 0 {
		if img, _, err := decodeImage(d.HeroImageBytes); err == nil {
			scaled := scaleImage(img, int(hs), int(hs))
			dc.DrawImageAnchored(scaled, int(hx+hs/2), int(cy), 0.5, 0.5)
		}
	}
	dc.SetColor(colorGold)
	dc.SetLineWidth(1)
	dc.DrawRectangle(hx, hy, hs, hs)
	dc.Stroke()

	tx := hx + hs + 16
	g.loadFont(dc, 18)
	dc.SetColor(colorGold)
	dc.DrawStringAnchored(fmt.Sprintf("%s — %s", d.PlayerName, d.HeroName), tx, cy-8, 0, 0.5)

	g.loadFont(dc, 11)
	dc.SetColor(colorGray)
	dc.DrawStringAnchored(fmt.Sprintf("Build de habilidades en nivel %d", d.Level), tx, cy+10, 0, 0.5)

	dc.SetColor(colorGold)
	dc.SetLineWidth(1)
	dc.DrawLine(0, h-1, canvasW, h-1)
	dc.Stroke()
}

func (g *ImageGenerator) drawBuildGroupTable(dc *gg.Context, rows []BuildGroupRow, y, headH, rH float64) {
	const w = canvasW

	dc.SetColor(colorPanel)
	dc.DrawRectangle(0, y, w, headH)
	dc.Fill()
	g.loadFont(dc, 11)
	dc.SetColor(colorGray)
	dc.DrawStringAnchored("Build (Q-W-E-R)", w*0.06, y+headH/2, 0, 0.5)
	dc.DrawStringAnchored("G", w*0.45, y+headH/2, 0.5, 0.5)
	dc.DrawStringAnchored("W", w*0.58, y+headH/2, 0.5, 0.5)
	dc.DrawStringAnchored("L", w*0.68, y+headH/2, 0.5, 0.5)
	dc.DrawStringAnchored("E", w*0.78, y+headH/2, 0.5, 0.5)
	dc.DrawStringAnchored("%", w*0.90, y+headH/2, 0.5, 0.5)

	ry := y + headH
	for idx, row := range rows {
		if idx%2 == 1 {
			dc.SetColor(colorRowAlt)
			dc.DrawRectangle(0, ry, w, rH)
			dc.Fill()
		}
		pct := 0.0
		if row.Total > 0 {
			pct = float64(row.Wins) / float64(row.Total) * 100
		}
		g.loadFont(dc, 13)
		dc.SetColor(colorWhite)
		dc.DrawStringAnchored(row.Label, w*0.06, ry+rH/2, 0, 0.5)
		dc.DrawStringAnchored(fmt.Sprintf("%d", row.Total), w*0.45, ry+rH/2, 0.5, 0.5)
		dc.SetColor(colorGreen)
		dc.DrawStringAnchored(fmt.Sprintf("%d", row.Wins), w*0.58, ry+rH/2, 0.5, 0.5)
		dc.SetColor(colorRed)
		dc.DrawStringAnchored(fmt.Sprintf("%d", row.Losses), w*0.68, ry+rH/2, 0.5, 0.5)
		dc.SetColor(colorGray)
		dc.DrawStringAnchored(fmt.Sprintf("%d", row.Draws), w*0.78, ry+rH/2, 0.5, 0.5)
		dc.SetColor(colorGold)
		dc.DrawStringAnchored(fmt.Sprintf("%.0f%%", pct), w*0.90, ry+rH/2, 0.5, 0.5)
		ry += rH
	}
}
```

- [ ] **Step 2: Verify the package builds**

Run: `cd /home/osmarg/Code/discord-dota && go build ./...`
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
cd /home/osmarg/Code/discord-dota
git add internal/ranking/image_mystats.go
git commit -m "Add PNG renderer for /dota mystats ability-build card"
```

---

### Task 7: Wire up the `/dota mystats` command (`discord/bot.go`)

**Files:**
- Modify: `discord/bot.go`

- [ ] **Step 1: Register the subcommand**

In `registerCommands` (`discord/bot.go`), find the `"stats"` subcommand entry (around line 175-179):

```go
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "stats",
					Description: "Estadísticas por héroe en el parche actual (W/L, % victorias)",
				},
```

Add a new subcommand entry immediately after it:

```go
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "mystats",
					Description: "Record de victorias agrupado por build de habilidades (Q-W-E-R) en un héroe y nivel",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "hero",
							Description: "Nombre del héroe (ej. Viper, Juggernaut)",
							Required:    true,
						},
						{
							Type:        discordgo.ApplicationCommandOptionInteger,
							Name:        "level",
							Description: "Nivel del héroe a evaluar (1-9)",
							Required:    true,
							Choices: []*discordgo.ApplicationCommandOptionChoice{
								{Name: "1", Value: 1},
								{Name: "2", Value: 2},
								{Name: "3", Value: 3},
								{Name: "4", Value: 4},
								{Name: "5", Value: 5},
								{Name: "6", Value: 6},
								{Name: "7", Value: 7},
								{Name: "8", Value: 8},
								{Name: "9", Value: 9},
							},
						},
						{
							Type:        discordgo.ApplicationCommandOptionUser,
							Name:        "jugador",
							Description: "Jugador a consultar (opcional, por defecto tú)",
							Required:    false,
						},
					},
				},
```

- [ ] **Step 2: Wire the handler into the subcommand switch**

In `interactionCreate` (`discord/bot.go`, around line 332-333), find:

```go
	case "stats":
		b.handleStatsSlash(s, i)
```

Add right after it:

```go
	case "mystats":
		b.handleMyStatsSlash(s, i, subcommand)
```

- [ ] **Step 3: Write `handleMyStatsSlash`**

Add this new function right after `handleStatsSlash` (after line 893, before `handleHelpSlash`):

```go
func (b *Bot) handleMyStatsSlash(s *discordgo.Session, i *discordgo.InteractionCreate, subcommand *discordgo.ApplicationCommandInteractionDataOption) {
	if b.stratzClient == nil || !b.stratzClient.IsConfigured() {
		b.sendFollowup(s, i, "❌ Stratz no está configurado.")
		return
	}

	var heroQuery string
	var level int
	var jugadorID string
	for _, opt := range subcommand.Options {
		switch opt.Name {
		case "hero":
			heroQuery = opt.StringValue()
		case "level":
			level = int(opt.IntValue())
		case "jugador":
			jugadorID = opt.UserValue(s).ID
		}
	}

	heroID, heroName, candidates, err := b.dotaClient.FindHeroByName(heroQuery)
	if err != nil {
		if len(candidates) > 0 {
			b.sendFollowup(s, i, fmt.Sprintf("❌ Varios héroes coinciden con \"%s\": %s. Sé más específico.", heroQuery, strings.Join(candidates, ", ")))
		} else {
			b.sendFollowup(s, i, fmt.Sprintf("❌ No encontré el héroe \"%s\".", heroQuery))
		}
		return
	}

	targetDiscordID := jugadorID
	if targetDiscordID == "" {
		if i.Member != nil {
			targetDiscordID = i.Member.User.ID
		} else if i.User != nil {
			targetDiscordID = i.User.ID
		}
	}
	accountIDStr, found := b.userStore.Get(targetDiscordID)
	if !found {
		b.sendFollowup(s, i, "❌ Ese jugador no está registrado. Usa `/dota register account_id:<tu_steam_id>`.")
		return
	}
	accountID, parseErr := strconv.ParseInt(accountIDStr, 10, 64)
	if parseErr != nil {
		b.sendFollowup(s, i, "❌ account_id registrado inválido.")
		return
	}

	qName, wName, eName, ok := b.dotaClient.GetHeroQWE(heroID)
	if !ok {
		b.sendFollowup(s, i, fmt.Sprintf("❌ No tengo datos de habilidades para %s.", heroName))
		return
	}

	afterUnix := time.Date(b.config.BaseYear, 1, 1, 0, 0, 0, 0, time.UTC).Unix()
	matches, err := b.stratzClient.GetPlayerHeroAbilityBuilds(accountID, heroID, afterUnix)
	if err != nil {
		b.sendFollowup(s, i, fmt.Sprintf("❌ Error consultando Stratz: %v", err))
		return
	}

	groups := dota.GroupBuildResults(matches, level, qName, wName, eName)
	if len(groups) == 0 {
		b.sendFollowup(s, i, fmt.Sprintf("No encontré partidas de %s con datos de habilidades hasta nivel %d desde %d.", heroName, level, b.config.BaseYear))
		return
	}

	playerName, avatarURL := b.getPlayerNameAndAvatar(accountIDStr, accountID)

	var avatarBytes, heroImgBytes []byte
	ctx := context.Background()
	if b.minioClient != nil {
		avatarKey := fmt.Sprintf("assets/avatars/%s.jpg", accountIDStr)
		avatarBytes, _ = b.minioClient.GetCached(ctx, avatarKey)
		if len(avatarBytes) == 0 && avatarURL != "" {
			avatarBytes, _ = b.minioClient.GetOrFetchAsset(ctx, avatarKey, avatarURL)
		}
		if heroURL := b.dotaClient.GetHeroImageURL(heroID); heroURL != "" {
			heroImgBytes, _ = b.minioClient.GetOrFetchAsset(ctx, fmt.Sprintf("assets/heroes/%d.png", heroID), heroURL)
		}
	}

	rows := make([]ranking.BuildGroupRow, 0, len(groups))
	totalGames := 0
	for _, g := range groups {
		rows = append(rows, ranking.BuildGroupRow{
			Label:  g.Tuple.Label(),
			Wins:   g.Wins,
			Losses: g.Losses,
			Draws:  0,
			Total:  g.Total,
		})
		totalGames += g.Total
	}

	renderData := ranking.MyHeroStatsRenderData{
		PlayerName:     playerName,
		AvatarBytes:    avatarBytes,
		HeroName:       heroName,
		HeroImageBytes: heroImgBytes,
		Level:          level,
		Groups:         rows,
		TotalGames:     totalGames,
	}
	gen := ranking.NewImageGenerator()
	imgBytes, renderErr := gen.RenderMyHeroStats(renderData)
	if renderErr != nil {
		b.sendFollowup(s, i, fmt.Sprintf("❌ Error generando imagen: %v", renderErr))
		return
	}

	fname := fmt.Sprintf("mystats-%d-%d.png", heroID, level)
	_, ferr := s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Files: []*discordgo.File{{Name: fname, ContentType: "image/png", Reader: bytes.NewReader(imgBytes)}},
	})
	if ferr != nil {
		getLogger().Errorf("mystats: send PNG followup: %v", ferr)
	}
}
```

- [ ] **Step 4: Verify the package builds**

Run: `cd /home/osmarg/Code/discord-dota && go build ./...`
Expected: no errors. (All imports used — `dota`, `ranking`, `context`, `strconv`, `strings`, `time`, `bytes`, `fmt` — are already imported in `bot.go`.)

- [ ] **Step 5: Update the help text**

In `handleHelpSlash` (`discord/bot.go`), find the `/dota stats` field (around line 916-920):

```go
			{
				Name:   "/dota stats",
				Value:  "Un mensaje por cada usuario registrado: estadísticas por héroe (W/L, %) con ≥STATS_MIN_GAMES partidas en las últimas STATS_TAKE partidas. Colores: 🔴 ≤40%, 🟡 40-50%, 🟢 ≥50%.",
				Inline: false,
			},
```

Add a new field right after it (before the `/dota help` field):

```go
			{
				Name:   "/dota mystats hero:<nombre> level:<1-9> [jugador:@usuario]",
				Value:  "Record de victorias agrupado por build de habilidades (Q-W-E-R) en un héroe, hasta el nivel indicado. Sin `jugador`, usa tu propia cuenta registrada.\n**Ejemplo:** `/dota mystats hero:Viper level:6`",
				Inline: false,
			},
```

- [ ] **Step 6: Commit**

```bash
cd /home/osmarg/Code/discord-dota
git add discord/bot.go
git commit -m "Add /dota mystats command"
```

---

### Task 8: Manual verification against real data

**Files:** none (verification only)

- [ ] **Step 1: Run the full test suite**

Run: `cd /home/osmarg/Code/discord-dota && go test ./... -v`
Expected: PASS across all packages, no failures.

- [ ] **Step 2: Build the binary**

Run: `cd /home/osmarg/Code/discord-dota && go build -o /tmp/dota-discord-bot-mystats-check ./...`
Expected: builds successfully.

- [ ] **Step 3: Deploy to a test/dev bot instance and run the command in Discord**

Run `/dota mystats hero:Viper level:6` for Fifrex's account (steamAccountId `208925877`, the account used throughout the design research).

Expected: a PNG card is posted showing at least the `1-1-3-1` build group with 1+ games recorded (this exact build/level combo was manually confirmed against match `8883621088` during the design session — it must appear in the results).

- [ ] **Step 4: Spot-check the ambiguous/no-match error paths**

Run `/dota mystats hero:xyz123notahero level:6` — expect the "no encontré el héroe" followup message.

Run `/dota mystats hero:a level:6` (or another single-letter query likely to match 2+ heroes) — expect the "varios héroes coinciden" followup message listing candidates.

- [ ] **Step 5: Final commit (if any fixes were needed during verification)**

```bash
cd /home/osmarg/Code/discord-dota
git add -A
git commit -m "Fix issues found during mystats manual verification"
```

(Skip this step if no fixes were needed.)
