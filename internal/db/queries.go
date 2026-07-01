package db

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/lib/pq"
)

// ---- Users ----

// User represents a registered player.
type User struct {
	ID           int
	DiscordID    *string
	DotaID       int64
	DisplayName  *string
	RegisteredAt time.Time
	Active       bool
}

func (d *DB) UpsertUser(discordID *string, dotaID int64, displayName *string) error {
	_, err := d.Exec(`
		INSERT INTO users (discord_id, dota_id, display_name)
		VALUES ($1, $2, $3)
		ON CONFLICT (dota_id) DO UPDATE
		  SET discord_id   = COALESCE($1, users.discord_id),
		      display_name = COALESCE($3, users.display_name)
	`, discordID, dotaID, displayName)
	return err
}

func (d *DB) GetAllUsers() ([]User, error) {
	rows, err := d.Query(`
		SELECT id, discord_id, dota_id, display_name, registered_at, active
		FROM users WHERE active = true
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.DiscordID, &u.DotaID, &u.DisplayName, &u.RegisteredAt, &u.Active); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (d *DB) GetUserByDiscordID(discordID string) (*User, error) {
	var u User
	err := d.QueryRow(`
		SELECT id, discord_id, dota_id, display_name, registered_at, active
		FROM users WHERE discord_id = $1
	`, discordID).Scan(&u.ID, &u.DiscordID, &u.DotaID, &u.DisplayName, &u.RegisteredAt, &u.Active)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &u, err
}

// ---- Matches ----

func (d *DB) UpsertMatch(matchID int64, startTime time.Time, durationSecs, gameMode int, radiantWin, parsed bool, topLane, midLane, botLane string) error {
	_, err := d.Exec(`
		INSERT INTO matches (match_id, start_time, duration_secs, game_mode, radiant_win, parsed,
		                     top_lane_outcome, mid_lane_outcome, bottom_lane_outcome)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (match_id) DO UPDATE SET
		    parsed              = EXCLUDED.parsed,
		    top_lane_outcome    = COALESCE(NULLIF(EXCLUDED.top_lane_outcome,''),    matches.top_lane_outcome),
		    mid_lane_outcome    = COALESCE(NULLIF(EXCLUDED.mid_lane_outcome,''),    matches.mid_lane_outcome),
		    bottom_lane_outcome = COALESCE(NULLIF(EXCLUDED.bottom_lane_outcome,''), matches.bottom_lane_outcome)
	`, matchID, startTime, durationSecs, gameMode, radiantWin, parsed, topLane, midLane, botLane)
	return err
}

func (d *DB) MatchExists(matchID int64) (bool, error) {
	var exists bool
	err := d.QueryRow(`SELECT EXISTS(SELECT 1 FROM matches WHERE match_id = $1)`, matchID).Scan(&exists)
	return exists, err
}

// ---- Player matches ----

// PlayerMatchRow holds all per-player data for one match.
type PlayerMatchRow struct {
	MatchID     int64
	DotaID      int64
	IsRadiant   bool
	HeroID      int
	Kills       int
	Deaths      int
	Assists     int
	Level       int
	GPM         int
	XPM         int
	HeroDamage  int
	TowerDamage int
	Healing     int
	Imp         int
	Award       string
	Lane        string
	Role        string
	Won         bool
	MMRDelta    int
}

func (d *DB) UpsertPlayerMatch(r PlayerMatchRow) error {
	_, err := d.Exec(`
		INSERT INTO player_matches
		  (match_id, dota_id, is_radiant, hero_id, kills, deaths, assists,
		   level, gpm, xpm, hero_damage, tower_damage, healing, imp, award,
		   lane, role, won, mmr_delta)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19)
		ON CONFLICT (match_id, dota_id) DO NOTHING
	`, r.MatchID, r.DotaID, r.IsRadiant, r.HeroID, r.Kills, r.Deaths, r.Assists,
		r.Level, r.GPM, r.XPM, r.HeroDamage, r.TowerDamage, r.Healing, r.Imp, r.Award,
		r.Lane, r.Role, r.Won, r.MMRDelta)
	return err
}

func (d *DB) PlayerMatchExists(matchID, dotaID int64) (bool, error) {
	var exists bool
	err := d.QueryRow(`
		SELECT EXISTS(SELECT 1 FROM player_matches WHERE match_id=$1 AND dota_id=$2)
	`, matchID, dotaID).Scan(&exists)
	return exists, err
}

// ---- Last processed match ----

func (d *DB) GetLastProcessedMatch(dotaID int64) (int64, bool, error) {
	var matchID int64
	err := d.QueryRow(`SELECT match_id FROM last_processed_match WHERE dota_id = $1`, dotaID).Scan(&matchID)
	if err == sql.ErrNoRows {
		return 0, false, nil
	}
	return matchID, err == nil, err
}

func (d *DB) SetLastProcessedMatch(dotaID, matchID int64) error {
	_, err := d.Exec(`
		INSERT INTO last_processed_match (dota_id, match_id, updated_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (dota_id) DO UPDATE SET match_id = $2, updated_at = NOW()
	`, dotaID, matchID)
	return err
}

// ---- Parse queue ----

func (d *DB) EnqueueParse(matchID int64) error {
	_, err := d.Exec(`
		INSERT INTO parse_queue (match_id) VALUES ($1)
		ON CONFLICT (match_id) DO NOTHING
	`, matchID)
	return err
}

func (d *DB) MarkParseDone(matchID int64) error {
	_, err := d.Exec(`UPDATE parse_queue SET status='done' WHERE match_id=$1`, matchID)
	return err
}

func (d *DB) GetPendingParseQueue() ([]int64, error) {
	rows, err := d.Query(`
		SELECT match_id FROM parse_queue WHERE status='pending' ORDER BY enqueued_at
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (d *DB) IncrementParseAttempt(matchID int64) error {
	_, err := d.Exec(`
		UPDATE parse_queue
		SET attempt_count = attempt_count + 1, last_attempt = NOW()
		WHERE match_id = $1
	`, matchID)
	return err
}

// ---- Config ----

func (d *DB) GetConfig(key string) (string, error) {
	var val string
	err := d.QueryRow(`SELECT value FROM config WHERE key = $1`, key).Scan(&val)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return val, err
}

func (d *DB) SetConfig(key, value string) error {
	_, err := d.Exec(`
		INSERT INTO config (key, value, updated_at) VALUES ($1, $2, NOW())
		ON CONFLICT (key) DO UPDATE SET value = $2, updated_at = NOW()
	`, key, value)
	return err
}

// ---- Ranking messages ----

func (d *DB) GetRankingMessage(messageType string) (channelID, messageID string, err error) {
	err = d.QueryRow(`
		SELECT channel_id, message_id FROM ranking_messages WHERE message_type = $1
	`, messageType).Scan(&channelID, &messageID)
	if err == sql.ErrNoRows {
		return "", "", nil
	}
	return channelID, messageID, err
}

func (d *DB) SetRankingMessage(messageType, channelID, messageID string) error {
	_, err := d.Exec(`
		INSERT INTO ranking_messages (message_type, channel_id, message_id, updated_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (message_type) DO UPDATE SET channel_id=$2, message_id=$3, updated_at=NOW()
	`, messageType, channelID, messageID)
	return err
}

// ---- JSON migration ----

// MigrateFromJSON reads legacy JSON stores and inserts into PG.
// Idempotent: skips if config key 'json_migration_done' = 'true'.
func (d *DB) MigrateFromJSON(
	users map[string]string,
	lastMatches map[string]int64,
	channelID string,
) error {
	done, err := d.GetConfig("json_migration_done")
	if err != nil {
		return fmt.Errorf("check migration flag: %w", err)
	}
	if done == "true" {
		return nil
	}

	for discordID, dotaIDStr := range users {
		var dotaID int64
		if _, err := fmt.Sscanf(dotaIDStr, "%d", &dotaID); err != nil {
			continue
		}
		did := discordID
		if err := d.UpsertUser(&did, dotaID, nil); err != nil {
			return fmt.Errorf("migrate user %s: %w", discordID, err)
		}
	}

	for discordID, matchID := range lastMatches {
		dotaIDStr, ok := users[discordID]
		if !ok {
			continue
		}
		var dotaID int64
		if _, err := fmt.Sscanf(dotaIDStr, "%d", &dotaID); err != nil {
			continue
		}
		if err := d.SetLastProcessedMatch(dotaID, matchID); err != nil {
			return fmt.Errorf("migrate last match %s: %w", discordID, err)
		}
	}

	if channelID != "" {
		if err := d.SetConfig("notification_channel", channelID); err != nil {
			return fmt.Errorf("migrate channel: %w", err)
		}
	}

	return d.SetConfig("json_migration_done", "true")
}

// ---- Lane records ----

// LaneRecord holds all-time lane phase stats for one player.
type LaneRecord struct {
	DotaID  int64
	Wins    int
	Draws   int
	Losses  int
	Unknown int
}

// RefreshLaneRecords recomputes lane phase win/draw/loss/unknown for the given
// dota IDs from the player_matches + matches join. Safe to call repeatedly.
func (d *DB) RefreshLaneRecords(dotaIDs []int64) error {
	if len(dotaIDs) == 0 {
		return nil
	}
	_, err := d.Exec(`
		WITH lane_data AS (
		    SELECT
		        pm.dota_id,
		        pm.is_radiant,
		        CASE
		            WHEN pm.lane = 'SAFE_LANE' AND     pm.is_radiant THEN m.bottom_lane_outcome
		            WHEN pm.lane = 'SAFE_LANE' AND NOT pm.is_radiant THEN m.top_lane_outcome
		            WHEN pm.lane = 'MID_LANE'                        THEN m.mid_lane_outcome
		            WHEN pm.lane = 'OFF_LANE' AND     pm.is_radiant  THEN m.top_lane_outcome
		            WHEN pm.lane = 'OFF_LANE' AND NOT pm.is_radiant  THEN m.bottom_lane_outcome
		            ELSE NULL
		        END AS lane_outcome
		    FROM player_matches pm
		    JOIN matches m ON m.match_id = pm.match_id
		    WHERE pm.dota_id = ANY($1)
		)
		INSERT INTO lane_records (dota_id, wins, draws, losses, unknown, updated_at)
		SELECT
		    dota_id,
		    COUNT(*) FILTER (WHERE
		        (lane_outcome IN ('RADIANT_VICTORY','RADIANT_STOMP') AND     is_radiant) OR
		        (lane_outcome IN ('DIRE_VICTORY',   'DIRE_STOMP')    AND NOT is_radiant)
		    ) AS wins,
		    COUNT(*) FILTER (WHERE lane_outcome = 'TIE') AS draws,
		    COUNT(*) FILTER (WHERE
		        (lane_outcome IN ('DIRE_VICTORY',   'DIRE_STOMP')    AND     is_radiant) OR
		        (lane_outcome IN ('RADIANT_VICTORY','RADIANT_STOMP') AND NOT is_radiant)
		    ) AS losses,
		    COUNT(*) FILTER (WHERE lane_outcome IS NULL OR lane_outcome = '') AS unknown,
		    NOW()
		FROM lane_data
		GROUP BY dota_id
		ON CONFLICT (dota_id) DO UPDATE SET
		    wins       = EXCLUDED.wins,
		    draws      = EXCLUDED.draws,
		    losses     = EXCLUDED.losses,
		    unknown    = EXCLUDED.unknown,
		    updated_at = NOW()
	`, pq.Array(dotaIDs))
	return err
}

// GetAllLaneRecords returns lane phase records for all players.
func (d *DB) GetAllLaneRecords() (map[int64]LaneRecord, error) {
	rows, err := d.Query(`SELECT dota_id, wins, draws, losses, unknown FROM lane_records`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[int64]LaneRecord)
	for rows.Next() {
		var r LaneRecord
		if err := rows.Scan(&r.DotaID, &r.Wins, &r.Draws, &r.Losses, &r.Unknown); err != nil {
			return nil, err
		}
		result[r.DotaID] = r
	}
	return result, rows.Err()
}
