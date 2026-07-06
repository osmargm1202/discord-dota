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
