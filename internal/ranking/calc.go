package ranking

import (
	"dota-discord-bot/internal/db"
	"fmt"
	"time"
)

// WeekBounds returns Monday 00:00 UTC start and the following Monday for the week containing t.
func WeekBounds(t time.Time) (start, end time.Time) {
	t = t.UTC()
	weekday := int(t.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	start = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC).
		AddDate(0, 0, -(weekday - 1))
	end = start.AddDate(0, 0, 7)
	return
}

// MonthBounds returns first and first-of-next-month for year/month.
func MonthBounds(year, month int) (start, end time.Time) {
	start = time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	end = start.AddDate(0, 1, 0)
	return
}

// YearBounds returns full year range for the given base year.
func YearBounds(baseYear int) (start, end time.Time) {
	start = time.Date(baseYear, 1, 1, 0, 0, 0, 0, time.UTC)
	end = time.Date(baseYear+1, 1, 1, 0, 0, 0, 0, time.UTC)
	return
}

// PlayerRankRow holds individual ranking data for one player.
type PlayerRankRow struct {
	DotaID      int64
	DiscordID   *string
	DisplayName string
	Wins        int
	Losses      int
	Net         int
	WinPct      float64
	MMRTotal    int
}

// Team2Row holds ranking data for a 2-player combo.
type Team2Row struct {
	DotaID1     int64
	DotaID2     int64
	Name1       string
	Name2       string
	Wins        int
	Losses      int
	Net         int
	WinPct      float64
	MMRTogether int
}

// Team3Row holds ranking data for a 3-player combo.
type Team3Row struct {
	DotaID1     int64
	DotaID2     int64
	DotaID3     int64
	Name1       string
	Name2       string
	Name3       string
	Wins        int
	Losses      int
	Net         int
	WinPct      float64
	MMRTogether int
}

// Calculator runs ranking queries against the DB.
type Calculator struct {
	db *db.DB
}

func NewCalculator(database *db.DB) *Calculator {
	return &Calculator{db: database}
}

func (c *Calculator) IndividualRanking(start, end time.Time) ([]PlayerRankRow, error) {
	rows, err := c.db.Query(`
		SELECT
		  u.dota_id,
		  u.discord_id,
		  COALESCE(u.display_name, 'Jugador') AS display_name,
		  COUNT(*) FILTER (WHERE pm.won)       AS wins,
		  COUNT(*) FILTER (WHERE NOT pm.won)   AS losses,
		  COUNT(*) FILTER (WHERE pm.won) - COUNT(*) FILTER (WHERE NOT pm.won) AS net,
		  COALESCE(SUM(pm.mmr_delta), 0)       AS mmr_total
		FROM player_matches pm
		JOIN matches m ON m.match_id = pm.match_id
		JOIN users u   ON u.dota_id  = pm.dota_id
		WHERE m.start_time >= $1 AND m.start_time < $2
		  AND u.active = true
		GROUP BY u.dota_id, u.discord_id, u.display_name
		ORDER BY net DESC, mmr_total DESC
	`, start, end)
	if err != nil {
		return nil, fmt.Errorf("individual ranking query: %w", err)
	}
	defer rows.Close()

	var result []PlayerRankRow
	for rows.Next() {
		var r PlayerRankRow
		if err := rows.Scan(&r.DotaID, &r.DiscordID, &r.DisplayName,
			&r.Wins, &r.Losses, &r.Net, &r.MMRTotal); err != nil {
			return nil, err
		}
		if total := r.Wins + r.Losses; total > 0 {
			r.WinPct = float64(r.Wins) / float64(total) * 100
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

func (c *Calculator) Team2Ranking(start, end time.Time) ([]Team2Row, error) {
	rows, err := c.db.Query(`
		SELECT
		  a.dota_id,
		  b.dota_id,
		  COALESCE(ua.display_name, 'Jugador') AS name1,
		  COALESCE(ub.display_name, 'Jugador') AS name2,
		  COUNT(*) FILTER (WHERE a.won)       AS wins,
		  COUNT(*) FILTER (WHERE NOT a.won)   AS losses,
		  COUNT(*) FILTER (WHERE a.won) - COUNT(*) FILTER (WHERE NOT a.won) AS net,
		  COALESCE(SUM(a.mmr_delta + b.mmr_delta), 0) AS mmr_together
		FROM player_matches a
		JOIN player_matches b ON a.match_id = b.match_id
		  AND a.dota_id < b.dota_id
		  AND a.is_radiant = b.is_radiant
		JOIN matches m  ON m.match_id = a.match_id
		JOIN users ua   ON ua.dota_id = a.dota_id AND ua.active = true
		JOIN users ub   ON ub.dota_id = b.dota_id AND ub.active = true
		WHERE m.start_time >= $1 AND m.start_time < $2
		GROUP BY a.dota_id, b.dota_id, name1, name2
		HAVING COUNT(*) >= 1
		ORDER BY net DESC, mmr_together DESC
	`, start, end)
	if err != nil {
		return nil, fmt.Errorf("team2 ranking query: %w", err)
	}
	defer rows.Close()

	var result []Team2Row
	for rows.Next() {
		var r Team2Row
		if err := rows.Scan(&r.DotaID1, &r.DotaID2, &r.Name1, &r.Name2,
			&r.Wins, &r.Losses, &r.Net, &r.MMRTogether); err != nil {
			return nil, err
		}
		if total := r.Wins + r.Losses; total > 0 {
			r.WinPct = float64(r.Wins) / float64(total) * 100
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

func (c *Calculator) Team3Ranking(start, end time.Time) ([]Team3Row, error) {
	rows, err := c.db.Query(`
		SELECT
		  a.dota_id, b.dota_id, c2.dota_id,
		  COALESCE(ua.display_name, 'J1') AS name1,
		  COALESCE(ub.display_name, 'J2') AS name2,
		  COALESCE(uc.display_name, 'J3') AS name3,
		  COUNT(*) FILTER (WHERE a.won)     AS wins,
		  COUNT(*) FILTER (WHERE NOT a.won) AS losses,
		  COUNT(*) FILTER (WHERE a.won) - COUNT(*) FILTER (WHERE NOT a.won) AS net,
		  COALESCE(SUM(a.mmr_delta + b.mmr_delta + c2.mmr_delta), 0) AS mmr_together
		FROM player_matches a
		JOIN player_matches b  ON a.match_id = b.match_id
		  AND a.dota_id < b.dota_id AND a.is_radiant = b.is_radiant
		JOIN player_matches c2 ON a.match_id = c2.match_id
		  AND b.dota_id < c2.dota_id AND a.is_radiant = c2.is_radiant
		JOIN matches m  ON m.match_id = a.match_id
		JOIN users ua   ON ua.dota_id = a.dota_id  AND ua.active = true
		JOIN users ub   ON ub.dota_id = b.dota_id  AND ub.active = true
		JOIN users uc   ON uc.dota_id = c2.dota_id AND uc.active = true
		WHERE m.start_time >= $1 AND m.start_time < $2
		GROUP BY a.dota_id, b.dota_id, c2.dota_id, name1, name2, name3
		HAVING COUNT(*) >= 1
		ORDER BY net DESC, mmr_together DESC
	`, start, end)
	if err != nil {
		return nil, fmt.Errorf("team3 ranking query: %w", err)
	}
	defer rows.Close()

	var result []Team3Row
	for rows.Next() {
		var r Team3Row
		if err := rows.Scan(&r.DotaID1, &r.DotaID2, &r.DotaID3,
			&r.Name1, &r.Name2, &r.Name3,
			&r.Wins, &r.Losses, &r.Net, &r.MMRTogether); err != nil {
			return nil, err
		}
		if total := r.Wins + r.Losses; total > 0 {
			r.WinPct = float64(r.Wins) / float64(total) * 100
		}
		result = append(result, r)
	}
	return result, rows.Err()
}
