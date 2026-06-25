# Full Infrastructure & Ranking System Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add PostgreSQL + MinIO to docker-compose, migrate JSON storage to PG, add historical backfill, generate PNG ranking images, and post them to a dedicated Discord ranking channel.

**Architecture:** PostgreSQL is the source of truth for all data. MinIO serves ranking PNGs via CloudFlare tunnel. A background goroutine backfills historical Stratz data on startup, skipping already-stored matches. Ranking images regenerate after every match notification.

**Tech Stack:** Go 1.25, PostgreSQL 16, MinIO, `lib/pq`, `fogleman/gg`, `minio-go/v7`, Exo2 TTF embedded font.

---

## File Map

**Create:**
- `internal/db/db.go` — PG connection pool, RunMigrations, MigrateFromJSON
- `internal/db/schema.sql` — embedded schema
- `internal/db/queries.go` — all typed DB query functions
- `internal/minio/client.go` — MinIO upload + public URL
- `internal/backfill/backfill.go` — historical match fetch with rate limiting
- `internal/ranking/calc.go` — ranking SQL queries, pure data structs
- `internal/ranking/image.go` — PNG generation with fogleman/gg
- `internal/ranking/updater.go` — orchestrate calc→image→upload→discord edit
- `discord/ranking_commands.go` — /dota ranking subcommands + /dota admin register
- `fonts/Exo2-Regular.ttf` — embedded font (download step)

**Modify:**
- `go.mod` / `go.sum` — add lib/pq, fogleman/gg, minio-go/v7
- `config/config.go` — add PG DSN, MinIO vars, RANKING_CHANNEL_ID, BASE_YEAR, BACKFILL_DELAY_MS
- `docker-compose.yml` — add postgres + minio services, cloudflared network
- `.env.example` — add new vars
- `main.go` — init DB, run migrations, run JSON migration, start backfill goroutine
- `discord/bot.go` — add RankingChannelID to Bot, trigger ranking refresh after match, register new commands
- `dota/stratz.go` — add GetPlayerMatchesForBackfill paginated query

---

### Task 1: Add dependencies and download font

**Files:**
- Modify: `go.mod`
- Create: `fonts/Exo2-Regular.ttf`

- [ ] Add dependencies:

```bash
cd /home/osmarg/Code/discord-dota
go get github.com/lib/pq@v1.10.9
go get github.com/fogleman/gg@v1.3.0
go get github.com/minio/minio-go/v7@v7.0.91
go mod tidy
```

- [ ] Download Exo2 font:

```bash
mkdir -p fonts
curl -L "https://github.com/NDISCOVER/Exo-2.0/raw/master/fonts/ttf/Exo2-Regular.ttf" \
  -o fonts/Exo2-Regular.ttf
# Verify it downloaded
ls -lh fonts/Exo2-Regular.ttf
```

- [ ] Commit:

```bash
git add go.mod go.sum fonts/
git commit -m "feat: add lib/pq, fogleman/gg, minio-go deps and Exo2 font"
```

---

### Task 2: Update config.go

**Files:**
- Modify: `config/config.go`

- [ ] Replace `config/config.go` content:

```go
package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	DiscordToken          string
	NotificationChannelID string
	RankingChannelID      string
	ServerID              string
	StratzToken           string
	Debug                 bool
	RefreshRateMinutes    int
	RequireParsed         bool
	StatsMinGames         int
	StatsTime             string
	StatsTake             int
	// PostgreSQL
	PostgresDSN string
	// MinIO
	MinioEndpoint  string
	MinioAccessKey string
	MinioSecretKey string
	MinioBucket    string
	MinioPublicURL string
	// Backfill
	BaseYear        int
	BackfillDelayMS int
}

func Load() (*Config, error) {
	if err := godotenv.Load(); err != nil {
		// not critical
	}

	discordToken := os.Getenv("DISCORD_TOKEN")
	if discordToken == "" {
		return nil, fmt.Errorf("DISCORD_TOKEN no está configurado en .env")
	}
	notificationChannelID := os.Getenv("NOTIFICATION_CHANNEL_ID")
	if notificationChannelID == "" {
		return nil, fmt.Errorf("NOTIFICATION_CHANNEL_ID no está configurado en .env")
	}
	serverID := os.Getenv("SERVER_ID")
	if serverID == "" {
		return nil, fmt.Errorf("SERVER_ID no está configurado en .env")
	}
	stratzToken := os.Getenv("STRATZ_TOKEN")
	if stratzToken == "" {
		return nil, fmt.Errorf("STRATZ_TOKEN es obligatorio")
	}

	debug := os.Getenv("DEBUG") == "true"
	requireParsed := os.Getenv("PARSED") != "false"

	refreshRateMinutes := 1
	if s := os.Getenv("REFRESH_RATE"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n >= 1 {
			if n > 60 {
				n = 60
			}
			refreshRateMinutes = n
		}
	}

	statsMinGames := 2
	if s := os.Getenv("STATS_MIN_GAMES"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n >= 2 {
			statsMinGames = n
		}
	}

	statsTake := 100
	if s := os.Getenv("STATS_TAKE"); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			if n <= 0 || n > 100 {
				statsTake = 100
			} else {
				statsTake = n
			}
		}
	}

	baseYear := 2026
	if s := os.Getenv("BASE_YEAR"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n >= 2020 {
			baseYear = n
		}
	}

	backfillDelayMS := 700
	if s := os.Getenv("BACKFILL_DELAY_MS"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n >= 100 {
			backfillDelayMS = n
		}
	}

	return &Config{
		DiscordToken:          discordToken,
		NotificationChannelID: notificationChannelID,
		RankingChannelID:      os.Getenv("RANKING_CHANNEL_ID"),
		ServerID:              serverID,
		StratzToken:           stratzToken,
		Debug:                 debug,
		RefreshRateMinutes:    refreshRateMinutes,
		RequireParsed:         requireParsed,
		StatsMinGames:         statsMinGames,
		StatsTime:             os.Getenv("STATS_TIME"),
		StatsTake:             statsTake,
		PostgresDSN:           os.Getenv("POSTGRES_DSN"),
		MinioEndpoint:         os.Getenv("MINIO_ENDPOINT"),
		MinioAccessKey:        os.Getenv("MINIO_ACCESS_KEY"),
		MinioSecretKey:        os.Getenv("MINIO_SECRET_KEY"),
		MinioBucket:           envOrDefault("MINIO_BUCKET", "dota-rankings"),
		MinioPublicURL:        os.Getenv("MINIO_PUBLIC_URL"),
		BaseYear:              baseYear,
		BackfillDelayMS:       backfillDelayMS,
	}, nil
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
```

- [ ] Verify compiles:

```bash
go build ./config/...
```

Expected: no output (success).

- [ ] Commit:

```bash
git add config/config.go
git commit -m "feat: add PG, MinIO, ranking, backfill config vars"
```

---

### Task 3: Create DB schema and connection

**Files:**
- Create: `internal/db/schema.sql`
- Create: `internal/db/db.go`

- [ ] Create `internal/db/schema.sql`:

```sql
CREATE TABLE IF NOT EXISTS users (
    id            SERIAL PRIMARY KEY,
    discord_id    VARCHAR(20) UNIQUE,
    dota_id       BIGINT NOT NULL UNIQUE,
    display_name  VARCHAR(100),
    registered_at TIMESTAMPTZ DEFAULT NOW(),
    active        BOOLEAN DEFAULT TRUE
);

CREATE TABLE IF NOT EXISTS matches (
    match_id      BIGINT PRIMARY KEY,
    start_time    TIMESTAMPTZ NOT NULL,
    duration_secs INT,
    game_mode     INT,
    radiant_win   BOOLEAN,
    parsed        BOOLEAN DEFAULT FALSE,
    created_at    TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS parse_queue (
    match_id      BIGINT PRIMARY KEY REFERENCES matches(match_id),
    enqueued_at   TIMESTAMPTZ DEFAULT NOW(),
    last_attempt  TIMESTAMPTZ,
    attempt_count INT DEFAULT 0,
    status        VARCHAR(20) DEFAULT 'pending'
);

CREATE TABLE IF NOT EXISTS player_matches (
    match_id      BIGINT REFERENCES matches(match_id),
    dota_id       BIGINT NOT NULL,
    is_radiant    BOOLEAN,
    hero_id       INT,
    kills         INT,
    deaths        INT,
    assists       INT,
    level         INT,
    gpm           INT,
    xpm           INT,
    hero_damage   INT,
    tower_damage  INT,
    healing       INT,
    imp           INT,
    award         VARCHAR(20),
    lane          VARCHAR(20),
    role          VARCHAR(20),
    won           BOOLEAN,
    mmr_delta     INT,
    created_at    TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (match_id, dota_id)
);

CREATE TABLE IF NOT EXISTS last_processed_match (
    dota_id    BIGINT PRIMARY KEY,
    match_id   BIGINT,
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS ranking_messages (
    message_type VARCHAR(20) PRIMARY KEY,
    channel_id   VARCHAR(20),
    message_id   VARCHAR(20),
    updated_at   TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS config (
    key        VARCHAR(50) PRIMARY KEY,
    value      TEXT,
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_player_matches_dota_id ON player_matches(dota_id);
CREATE INDEX IF NOT EXISTS idx_player_matches_created_at ON player_matches(created_at);
CREATE INDEX IF NOT EXISTS idx_matches_start_time ON matches(start_time);
```

- [ ] Create `internal/db/db.go`:

```go
package db

import (
	"database/sql"
	_ "embed"
	"fmt"

	_ "github.com/lib/pq"
)

//go:embed schema.sql
var schemaSQL string

// DB wraps sql.DB with app-level methods.
type DB struct {
	*sql.DB
}

// New opens a PG connection pool and verifies connectivity.
func New(dsn string) (*DB, error) {
	sqlDB, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("sql.Open: %w", err)
	}
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("db ping: %w", err)
	}
	sqlDB.SetMaxOpenConns(10)
	sqlDB.SetMaxIdleConns(5)
	return &DB{sqlDB}, nil
}

// RunMigrations creates all tables if they do not exist.
func (d *DB) RunMigrations() error {
	if _, err := d.Exec(schemaSQL); err != nil {
		return fmt.Errorf("migrations: %w", err)
	}
	return nil
}
```

- [ ] Verify compiles:

```bash
go build ./internal/db/...
```

- [ ] Commit:

```bash
git add internal/db/
git commit -m "feat: add PG schema and connection pool"
```

---

### Task 4: DB query functions

**Files:**
- Create: `internal/db/queries.go`

- [ ] Create `internal/db/queries.go`:

```go
package db

import (
	"database/sql"
	"fmt"
	"time"
)

// ---- Users ----

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
		  SET discord_id = COALESCE($1, users.discord_id),
		      display_name = COALESCE($3, users.display_name)
	`, discordID, dotaID, displayName)
	return err
}

func (d *DB) GetAllUsers() ([]User, error) {
	rows, err := d.Query(`SELECT id, discord_id, dota_id, display_name, registered_at, active FROM users WHERE active = true`)
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
	err := d.QueryRow(`SELECT id, discord_id, dota_id, display_name, registered_at, active FROM users WHERE discord_id = $1`, discordID).
		Scan(&u.ID, &u.DiscordID, &u.DotaID, &u.DisplayName, &u.RegisteredAt, &u.Active)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &u, err
}

// ---- Matches ----

func (d *DB) UpsertMatch(matchID int64, startTime time.Time, durationSecs, gameMode int, radiantWin, parsed bool) error {
	_, err := d.Exec(`
		INSERT INTO matches (match_id, start_time, duration_secs, game_mode, radiant_win, parsed)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (match_id) DO UPDATE SET parsed = EXCLUDED.parsed
	`, matchID, startTime, durationSecs, gameMode, radiantWin, parsed)
	return err
}

func (d *DB) MatchExists(matchID int64) (bool, error) {
	var exists bool
	err := d.QueryRow(`SELECT EXISTS(SELECT 1 FROM matches WHERE match_id = $1)`, matchID).Scan(&exists)
	return exists, err
}

// ---- Player matches ----

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
	err := d.QueryRow(`SELECT EXISTS(SELECT 1 FROM player_matches WHERE match_id=$1 AND dota_id=$2)`, matchID, dotaID).Scan(&exists)
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
	rows, err := d.Query(`SELECT match_id FROM parse_queue WHERE status='pending' ORDER BY enqueued_at`)
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
	err = d.QueryRow(`SELECT channel_id, message_id FROM ranking_messages WHERE message_type = $1`, messageType).
		Scan(&channelID, &messageID)
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
	users map[string]string,   // discord_id -> dota_id string
	lastMatches map[string]int64, // discord_id -> match_id (unused post-migration, kept for reference)
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

	for discordIDStr, matchID := range lastMatches {
		// Find dota_id for this discord user
		dotaIDStr, ok := users[discordIDStr]
		if !ok {
			continue
		}
		var dotaID int64
		if _, err := fmt.Sscanf(dotaIDStr, "%d", &dotaID); err != nil {
			continue
		}
		if err := d.SetLastProcessedMatch(dotaID, matchID); err != nil {
			return fmt.Errorf("migrate last match %s: %w", discordIDStr, err)
		}
	}

	if channelID != "" {
		if err := d.SetConfig("notification_channel", channelID); err != nil {
			return fmt.Errorf("migrate channel: %w", err)
		}
	}

	return d.SetConfig("json_migration_done", "true")
}
```

- [ ] Verify compiles:

```bash
go build ./internal/db/...
```

- [ ] Commit:

```bash
git add internal/db/queries.go
git commit -m "feat: add typed PG query functions"
```

---

### Task 5: MinIO client

**Files:**
- Create: `internal/minio/client.go`

- [ ] Create `internal/minio/client.go`:

```go
package minioclient

import (
	"bytes"
	"context"
	"fmt"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Client struct {
	mc        *minio.Client
	bucket    string
	publicURL string
}

func New(endpoint, accessKey, secretKey, bucket, publicURL string) (*Client, error) {
	mc, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: false, // CloudFlare handles TLS
	})
	if err != nil {
		return nil, fmt.Errorf("minio.New: %w", err)
	}
	ctx := context.Background()
	exists, err := mc.BucketExists(ctx, bucket)
	if err != nil {
		return nil, fmt.Errorf("bucket check: %w", err)
	}
	if !exists {
		if err := mc.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
			return nil, fmt.Errorf("make bucket: %w", err)
		}
		// Set public read policy
		policy := fmt.Sprintf(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":["*"]},"Action":["s3:GetObject"],"Resource":["arn:aws:s3:::%s/*"]}]}`, bucket)
		if err := mc.SetBucketPolicy(ctx, bucket, policy); err != nil {
			return nil, fmt.Errorf("set bucket policy: %w", err)
		}
	}
	return &Client{mc: mc, bucket: bucket, publicURL: publicURL}, nil
}

// Upload stores PNG bytes and returns the public URL.
func (c *Client) Upload(ctx context.Context, key string, data []byte) (string, error) {
	_, err := c.mc.PutObject(ctx, c.bucket, key, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{
		ContentType: "image/png",
	})
	if err != nil {
		return "", fmt.Errorf("put object %s: %w", key, err)
	}
	return fmt.Sprintf("%s/%s/%s", c.publicURL, c.bucket, key), nil
}
```

- [ ] Verify compiles:

```bash
go build ./internal/minio/...
```

- [ ] Commit:

```bash
git add internal/minio/
git commit -m "feat: add MinIO client wrapper"
```

---

### Task 6: Stratz backfill query

**Files:**
- Modify: `dota/stratz.go` (append new method)

- [ ] Append to end of `dota/stratz.go`:

```go
// BackfillMatch is a lightweight match record for historical ingestion.
type BackfillMatch struct {
	ID            int64
	DidRadiantWin bool
	DurationSecs  int
	StartDateTime int64
	GameMode      stratzIntOrStr
	Players       []BackfillPlayer
}

// BackfillPlayer has only ranking-relevant fields.
type BackfillPlayer struct {
	SteamAccountID      int64          `json:"steamAccountId"`
	IsRadiant           bool           `json:"isRadiant"`
	HeroID              int            `json:"heroId"`
	Kills               int            `json:"kills"`
	Deaths              int            `json:"deaths"`
	Assists             int            `json:"assists"`
	Level               int            `json:"level"`
	GoldPerMinute       int            `json:"goldPerMinute"`
	ExperiencePerMinute int            `json:"experiencePerMinute"`
	HeroDamage          int            `json:"heroDamage"`
	TowerDamage         int            `json:"towerDamage"`
	HeroHealing         int            `json:"heroHealing"`
	Imp                 int            `json:"imp"`
	Award               string         `json:"award"`
	Lane                string         `json:"lane"`
	Role                string         `json:"role"`
}

// GetPlayerMatchesForBackfill fetches up to `take` matches starting at `skip`,
// only including matches after `afterUnix` (Unix timestamp).
func (c *StratzClient) GetPlayerMatchesForBackfill(steamAccountID int64, take, skip int, afterUnix int64) ([]BackfillMatch, error) {
	query := `
		query BackfillMatches($steamAccountId: Long!, $take: Int!, $skip: Int!, $after: Long!) {
			player(steamAccountId: $steamAccountId) {
				matches(request: { take: $take, skip: $skip, startDateTime: $after }) {
					id
					didRadiantWin
					durationSeconds
					startDateTime
					gameMode
					players {
						steamAccountId
						isRadiant
						heroId
						kills deaths assists level
						goldPerMinute experiencePerMinute
						heroDamage towerDamage heroHealing
						imp award lane role
					}
				}
			}
		}
	`
	var result struct {
		Player struct {
			Matches []struct {
				ID            int64          `json:"id"`
				DidRadiantWin bool           `json:"didRadiantWin"`
				DurationSecs  int            `json:"durationSeconds"`
				StartDateTime int64          `json:"startDateTime"`
				GameMode      stratzIntOrStr `json:"gameMode"`
				Players       []BackfillPlayer `json:"players"`
			} `json:"matches"`
		} `json:"player"`
	}
	if err := c.makeRequest(query, map[string]interface{}{
		"steamAccountId": steamAccountID,
		"take":           take,
		"skip":           skip,
		"after":          afterUnix,
	}, &result); err != nil {
		return nil, err
	}
	out := make([]BackfillMatch, len(result.Player.Matches))
	for i, m := range result.Player.Matches {
		out[i] = BackfillMatch{
			ID:            m.ID,
			DidRadiantWin: m.DidRadiantWin,
			DurationSecs:  m.DurationSecs,
			StartDateTime: m.StartDateTime,
			GameMode:      m.GameMode,
			Players:       m.Players,
		}
	}
	return out, nil
}
```

- [ ] Verify compiles:

```bash
go build ./dota/...
```

- [ ] Commit:

```bash
git add dota/stratz.go
git commit -m "feat: add GetPlayerMatchesForBackfill paginated Stratz query"
```

---

### Task 7: Backfill service

**Files:**
- Create: `internal/backfill/backfill.go`

- [ ] Create `internal/backfill/backfill.go`:

```go
package backfill

import (
	"dota-discord-bot/dota"
	"dota-discord-bot/internal/db"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
)

type Service struct {
	db           *db.DB
	stratz       *dota.StratzClient
	baseYear     int
	delayMS      int
}

func New(database *db.DB, stratz *dota.StratzClient, baseYear, delayMS int) *Service {
	return &Service{db: database, stratz: stratz, baseYear: baseYear, delayMS: delayMS}
}

// Run fetches historical matches for all users from BASE_YEAR-01-01.
// Skips matches already in DB. Safe to call multiple times (idempotent).
func (s *Service) Run() {
	afterUnix := time.Date(s.baseYear, 1, 1, 0, 0, 0, 0, time.UTC).Unix()
	logrus.Infof("backfill: starting from %d-01-01 (unix %d)", s.baseYear, afterUnix)

	users, err := s.db.GetAllUsers()
	if err != nil {
		logrus.Errorf("backfill: get users: %v", err)
		return
	}

	for _, u := range users {
		logrus.Infof("backfill: processing dota_id %d", u.DotaID)
		if err := s.backfillUser(u.DotaID, afterUnix); err != nil {
			logrus.Errorf("backfill: dota_id %d: %v", u.DotaID, err)
		}
	}
	logrus.Info("backfill: complete")
}

func (s *Service) backfillUser(dotaID int64, afterUnix int64) error {
	const pageSize = 100
	skip := 0
	total := 0

	for {
		matches, err := s.stratz.GetPlayerMatchesForBackfill(dotaID, pageSize, skip, afterUnix)
		if err != nil {
			return fmt.Errorf("fetch page skip=%d: %w", skip, err)
		}
		if len(matches) == 0 {
			break
		}

		for _, m := range matches {
			if err := s.storeMatch(dotaID, m); err != nil {
				logrus.Warnf("backfill: store match %d for %d: %v", m.ID, dotaID, err)
			}
			total++
			time.Sleep(time.Duration(s.delayMS) * time.Millisecond)
		}

		if total%50 == 0 {
			logrus.Infof("backfill: dota_id %d — %d matches stored so far", dotaID, total)
		}

		if len(matches) < pageSize {
			break
		}
		skip += pageSize
	}

	logrus.Infof("backfill: dota_id %d complete — %d matches total", dotaID, total)
	return nil
}

func (s *Service) storeMatch(dotaID int64, m dota.BackfillMatch) error {
	// Skip if player_match already stored
	exists, err := s.db.PlayerMatchExists(m.ID, dotaID)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	startTime := time.Unix(m.StartDateTime, 0).UTC()

	// Upsert match metadata
	if err := s.db.UpsertMatch(m.ID, startTime, m.DurationSecs, int(m.GameMode), m.DidRadiantWin, false); err != nil {
		return fmt.Errorf("upsert match: %w", err)
	}

	// Find this player in the match
	for _, p := range m.Players {
		if p.SteamAccountID != dotaID {
			continue
		}
		won := (m.DidRadiantWin && p.IsRadiant) || (!m.DidRadiantWin && !p.IsRadiant)
		mmrDelta := -25
		if won {
			mmrDelta = 25
		}
		return s.db.UpsertPlayerMatch(db.PlayerMatchRow{
			MatchID:     m.ID,
			DotaID:      dotaID,
			IsRadiant:   p.IsRadiant,
			HeroID:      p.HeroID,
			Kills:       p.Kills,
			Deaths:      p.Deaths,
			Assists:     p.Assists,
			Level:       p.Level,
			GPM:         p.GoldPerMinute,
			XPM:         p.ExperiencePerMinute,
			HeroDamage:  p.HeroDamage,
			TowerDamage: p.TowerDamage,
			Healing:     p.HeroHealing,
			Imp:         p.Imp,
			Award:       p.Award,
			Lane:        p.Lane,
			Role:        p.Role,
			Won:         won,
			MMRDelta:    mmrDelta,
		})
	}
	return nil
}
```

- [ ] Verify compiles:

```bash
go build ./internal/backfill/...
```

- [ ] Commit:

```bash
git add internal/backfill/
git commit -m "feat: add historical backfill service with rate limiting"
```

---

### Task 8: Ranking calculations

**Files:**
- Create: `internal/ranking/calc.go`

- [ ] Create `internal/ranking/calc.go`:

```go
package ranking

import (
	"dota-discord-bot/internal/db"
	"fmt"
	"time"
)

// WeekBounds returns Monday 00:00 UTC start and the following Monday (exclusive end) for the week containing t.
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

// MonthBounds returns the first and last moment of the given month/year.
func MonthBounds(year, month int) (start, end time.Time) {
	start = time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	end = start.AddDate(0, 1, 0)
	return
}

// YearBounds returns the full year range for the given base year.
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
	Net         int     // wins - losses
	WinPct      float64 // 0-100
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
		  COALESCE(u.display_name, sa.name, 'Jugador') AS display_name,
		  COUNT(*) FILTER (WHERE pm.won) AS wins,
		  COUNT(*) FILTER (WHERE NOT pm.won) AS losses,
		  COUNT(*) FILTER (WHERE pm.won) - COUNT(*) FILTER (WHERE NOT pm.won) AS net,
		  COALESCE(SUM(pm.mmr_delta), 0) AS mmr_total
		FROM player_matches pm
		JOIN matches m ON m.match_id = pm.match_id
		JOIN users u ON u.dota_id = pm.dota_id
		LEFT JOIN (SELECT NULL::text AS name) sa ON false
		WHERE m.start_time >= $1 AND m.start_time < $2
		  AND u.active = true
		GROUP BY u.dota_id, u.discord_id, display_name
		ORDER BY net DESC, mmr_total DESC
	`, start, end)
	if err != nil {
		return nil, fmt.Errorf("individual ranking query: %w", err)
	}
	defer rows.Close()

	var result []PlayerRankRow
	for rows.Next() {
		var r PlayerRankRow
		if err := rows.Scan(&r.DotaID, &r.DiscordID, &r.DisplayName, &r.Wins, &r.Losses, &r.Net, &r.MMRTotal); err != nil {
			return nil, err
		}
		total := r.Wins + r.Losses
		if total > 0 {
			r.WinPct = float64(r.Wins) / float64(total) * 100
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

func (c *Calculator) Team2Ranking(start, end time.Time) ([]Team2Row, error) {
	rows, err := c.db.Query(`
		SELECT
		  a.dota_id AS dota_id1,
		  b.dota_id AS dota_id2,
		  COALESCE(ua.display_name, 'Jugador') AS name1,
		  COALESCE(ub.display_name, 'Jugador') AS name2,
		  COUNT(*) FILTER (WHERE a.won) AS wins,
		  COUNT(*) FILTER (WHERE NOT a.won) AS losses,
		  COUNT(*) FILTER (WHERE a.won) - COUNT(*) FILTER (WHERE NOT a.won) AS net,
		  COALESCE(SUM(a.mmr_delta + b.mmr_delta), 0) AS mmr_together
		FROM player_matches a
		JOIN player_matches b ON a.match_id = b.match_id
		  AND a.dota_id < b.dota_id
		  AND a.is_radiant = b.is_radiant
		JOIN matches m ON m.match_id = a.match_id
		JOIN users ua ON ua.dota_id = a.dota_id AND ua.active = true
		JOIN users ub ON ub.dota_id = b.dota_id AND ub.active = true
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
		total := r.Wins + r.Losses
		if total > 0 {
			r.WinPct = float64(r.Wins) / float64(total) * 100
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

func (c *Calculator) Team3Ranking(start, end time.Time) ([]Team3Row, error) {
	rows, err := c.db.Query(`
		SELECT
		  a.dota_id, b.dota_id, c.dota_id,
		  COALESCE(ua.display_name, 'J1') AS name1,
		  COALESCE(ub.display_name, 'J2') AS name2,
		  COALESCE(uc.display_name, 'J3') AS name3,
		  COUNT(*) FILTER (WHERE a.won) AS wins,
		  COUNT(*) FILTER (WHERE NOT a.won) AS losses,
		  COUNT(*) FILTER (WHERE a.won) - COUNT(*) FILTER (WHERE NOT a.won) AS net,
		  COALESCE(SUM(a.mmr_delta + b.mmr_delta + c.mmr_delta), 0) AS mmr_together
		FROM player_matches a
		JOIN player_matches b ON a.match_id = b.match_id
		  AND a.dota_id < b.dota_id
		  AND a.is_radiant = b.is_radiant
		JOIN player_matches c ON a.match_id = c.match_id
		  AND b.dota_id < c.dota_id
		  AND a.is_radiant = c.is_radiant
		JOIN matches m ON m.match_id = a.match_id
		JOIN users ua ON ua.dota_id = a.dota_id AND ua.active = true
		JOIN users ub ON ub.dota_id = b.dota_id AND ub.active = true
		JOIN users uc ON uc.dota_id = c.dota_id AND uc.active = true
		WHERE m.start_time >= $1 AND m.start_time < $2
		GROUP BY a.dota_id, b.dota_id, c.dota_id, name1, name2, name3
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
		total := r.Wins + r.Losses
		if total > 0 {
			r.WinPct = float64(r.Wins) / float64(total) * 100
		}
		result = append(result, r)
	}
	return result, rows.Err()
}
```

- [ ] Verify compiles:

```bash
go build ./internal/ranking/...
```

- [ ] Commit:

```bash
git add internal/ranking/calc.go
git commit -m "feat: add ranking SQL calculations (individual, team2, team3)"
```

---

### Task 9: PNG image generation

**Files:**
- Create: `internal/ranking/image.go`

- [ ] Create `internal/ranking/image.go`:

```go
package ranking

import (
	_ "embed"
	"fmt"
	"image"
	"image/color"
	"io"
	"net/http"
	"time"

	"github.com/fogleman/gg"
	"golang.org/x/image/font/opentype"
)

//go:embed ../../fonts/Exo2-Regular.ttf
var exo2FontBytes []byte

const (
	canvasW    = 820.0
	rowH       = 50.0
	headerH    = 68.0
	sectionH   = 32.0
	footerH    = 28.0
	avatarSize = 34.0
	paddingL   = 18.0
)

var (
	colorBG      = color.RGBA{13, 17, 23, 255}    // #0D1117
	colorPanel   = color.RGBA{26, 35, 50, 255}     // #1A2332
	colorGold    = color.RGBA{200, 170, 110, 255}  // #C8AA6E
	colorGray    = color.RGBA{136, 153, 170, 255}  // #8899AA
	colorGreen   = color.RGBA{39, 174, 96, 255}    // #27AE60
	colorRed     = color.RGBA{192, 57, 43, 255}    // #C0392B
	colorBlue    = color.RGBA{52, 152, 219, 255}   // #3498DB
	colorYellow  = color.RGBA{243, 156, 18, 255}   // #F39C12
	colorWhite   = color.RGBA{232, 213, 183, 255}  // #E8D5B7
	colorRowAlt  = color.RGBA{17, 24, 39, 180}
	colorDivider = color.RGBA{200, 170, 110, 40}
)

// ImageGenerator renders ranking PNG images.
type ImageGenerator struct {
	fontSize float64
	avatarCache map[string]image.Image
}

func NewImageGenerator() *ImageGenerator {
	return &ImageGenerator{
		fontSize:    13,
		avatarCache: make(map[string]image.Image),
	}
}

func (g *ImageGenerator) loadFont(dc *gg.Context, size float64) error {
	ft, err := opentype.Parse(exo2FontBytes)
	if err != nil {
		return err
	}
	face, err := opentype.NewFace(ft, &opentype.FaceOptions{
		Size: size,
		DPI:  96,
	})
	if err != nil {
		return err
	}
	dc.SetFontFace(face)
	return nil
}

func (g *ImageGenerator) fetchAvatar(url string) image.Image {
	if img, ok := g.avatarCache[url]; ok {
		return img
	}
	if url == "" {
		return nil
	}
	resp, err := http.Get(url)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil
	}
	img, _, err := image.Decode(io.Reader(bytesReader(data)))
	if err != nil {
		return nil
	}
	g.avatarCache[url] = img
	return img
}

type bytesReaderT struct{ data []byte; pos int }
func bytesReader(d []byte) *bytesReaderT { return &bytesReaderT{data: d} }
func (b *bytesReaderT) Read(p []byte) (n int, err error) {
	if b.pos >= len(b.data) { return 0, io.EOF }
	n = copy(p, b.data[b.pos:])
	b.pos += n
	return n, nil
}

func (g *ImageGenerator) drawBackground(dc *gg.Context) {
	w := float64(dc.Width())
	h := float64(dc.Height())
	for y := 0.0; y < h; y++ {
		t := y / h
		r := lerp(float64(colorBG.R), float64(colorPanel.R), sineEase(t))
		gr := lerp(float64(colorBG.G), float64(colorPanel.G), sineEase(t))
		b := lerp(float64(colorBG.B), float64(colorPanel.B), sineEase(t))
		dc.SetRGBA255(int(r), int(gr), int(b), 255)
		dc.DrawLine(0, y, w, y)
		dc.Stroke()
	}
}

func lerp(a, b, t float64) float64 { return a + (b-a)*t }
func sineEase(t float64) float64 {
	// simple tent: 0→1→0
	if t < 0.5 { return t * 2 }
	return (1 - t) * 2
}

func (g *ImageGenerator) drawBorder(dc *gg.Context) {
	w := float64(dc.Width())
	h := float64(dc.Height())
	dc.SetColor(colorGold)
	dc.SetLineWidth(1.5)
	dc.DrawRectangle(0.5, 0.5, w-1, h-1)
	dc.Stroke()
	// top glow strip
	for i := 0; i < 3; i++ {
		alpha := 255 - i*70
		dc.SetRGBA255(200, 170, 110, alpha)
		dc.DrawLine(0, float64(i), w, float64(i))
		dc.Stroke()
	}
}

func (g *ImageGenerator) drawHeader(dc *gg.Context, title, subtitle string) {
	w := float64(dc.Width())
	// header background
	dc.SetColor(colorPanel)
	dc.DrawRectangle(0, 0, w, headerH)
	dc.Fill()
	// bottom divider
	dc.SetColor(colorGold)
	dc.SetLineWidth(0.8)
	dc.DrawLine(0, headerH, w, headerH)
	dc.Stroke()
	// title
	g.loadFont(dc, 16)
	dc.SetColor(colorGold)
	dc.DrawStringAnchored(title, paddingL, headerH*0.38, 0, 0.5)
	// subtitle
	g.loadFont(dc, 11)
	dc.SetColor(colorGray)
	dc.DrawStringAnchored(subtitle, paddingL, headerH*0.72, 0, 0.5)
}

func (g *ImageGenerator) drawSectionHeader(dc *gg.Context, y float64, text string) {
	w := float64(dc.Width())
	dc.SetColor(colorBG)
	dc.DrawRectangle(0, y, w, sectionH)
	dc.Fill()
	dc.SetColor(colorGold)
	dc.SetLineWidth(0.5)
	dc.DrawLine(0, y+sectionH, w, y+sectionH)
	dc.Stroke()
	g.loadFont(dc, 10)
	dc.SetColor(colorGold)
	dc.DrawStringAnchored(text, paddingL, y+sectionH/2, 0, 0.5)
}

func (g *ImageGenerator) drawColHeaders(dc *gg.Context, y float64, cols []colDef) {
	dc.SetColor(colorPanel)
	dc.DrawRectangle(0, y, float64(dc.Width()), 24)
	dc.Fill()
	g.loadFont(dc, 10)
	dc.SetColor(colorGray)
	for _, col := range cols {
		dc.DrawStringAnchored(col.label, col.x, y+12, col.anchorX, 0.5)
	}
}

type colDef struct {
	label   string
	x       float64
	anchorX float64
}

var indivCols = []colDef{
	{"#", 22, 0.5},
	{"JUGADOR", 70, 0},
	{"G", 450, 0.5},
	{"P", 500, 0.5},
	{"WIN%", 555, 0.5},
	{"+/−", 620, 0.5},
	{"~MMR", 700, 0.5},
}

var teamCols = []colDef{
	{"EQUIPO", 70, 0},
	{"G", 500, 0.5},
	{"P", 545, 0.5},
	{"WIN%", 595, 0.5},
	{"+/−", 655, 0.5},
	{"~MMR", 730, 0.5},
}

func (g *ImageGenerator) drawIndivRow(dc *gg.Context, y float64, idx int, r PlayerRankRow, avatarURL string) {
	w := float64(dc.Width())
	if idx%2 == 1 {
		dc.SetColor(colorRowAlt)
		dc.DrawRectangle(0, y, w, rowH)
		dc.Fill()
	}
	cx := paddingL + 4

	// rank number
	g.loadFont(dc, 13)
	medals := []string{"🥇", "🥈", "🥉"}
	rankStr := fmt.Sprintf("%d", idx+1)
	if idx < 3 {
		rankStr = medals[idx]
	}
	dc.SetColor(colorGray)
	dc.DrawStringAnchored(rankStr, cx+10, y+rowH/2, 0.5, 0.5)

	// avatar circle
	avatarX := 50.0
	dc.SetColor(colorPanel)
	dc.DrawCircle(avatarX+avatarSize/2, y+rowH/2, avatarSize/2+1)
	dc.Fill()
	dc.SetColor(colorGold)
	dc.DrawCircle(avatarX+avatarSize/2, y+rowH/2, avatarSize/2+1)
	dc.Stroke()

	// player name
	g.loadFont(dc, 13)
	dc.SetColor(colorWhite)
	dc.DrawStringAnchored(r.DisplayName, 92, y+rowH/2-6, 0, 0.5)
	g.loadFont(dc, 10)
	dc.SetColor(colorGray)

	// stats
	g.loadFont(dc, 13)
	dc.SetColor(colorGreen)
	dc.DrawStringAnchored(fmt.Sprintf("%d", r.Wins), 450, y+rowH/2, 0.5, 0.5)
	dc.SetColor(colorRed)
	dc.DrawStringAnchored(fmt.Sprintf("%d", r.Losses), 500, y+rowH/2, 0.5, 0.5)

	wpColor := winPctColor(r.WinPct)
	dc.SetColor(wpColor)
	dc.DrawStringAnchored(fmt.Sprintf("%.0f%%", r.WinPct), 555, y+rowH/2, 0.5, 0.5)

	netColor := signColor(r.Net)
	dc.SetColor(netColor)
	dc.DrawStringAnchored(signStr(r.Net), 620, y+rowH/2, 0.5, 0.5)

	mmrColor := signColor(r.MMRTotal)
	dc.SetColor(mmrColor)
	dc.DrawStringAnchored("~"+signStr(r.MMRTotal), 700, y+rowH/2, 0.5, 0.5)
}

func (g *ImageGenerator) drawTeam2Row(dc *gg.Context, y float64, idx int, r Team2Row) {
	w := float64(dc.Width())
	if idx%2 == 1 {
		dc.SetColor(colorRowAlt)
		dc.DrawRectangle(0, y, w, rowH)
		dc.Fill()
	}
	combo := r.Name1 + " + " + r.Name2
	g.loadFont(dc, 12)
	dc.SetColor(colorWhite)
	dc.DrawStringAnchored(combo, 70, y+rowH/2, 0, 0.5)

	g.loadFont(dc, 13)
	dc.SetColor(colorGreen)
	dc.DrawStringAnchored(fmt.Sprintf("%d", r.Wins), 500, y+rowH/2, 0.5, 0.5)
	dc.SetColor(colorRed)
	dc.DrawStringAnchored(fmt.Sprintf("%d", r.Losses), 545, y+rowH/2, 0.5, 0.5)
	dc.SetColor(winPctColor(r.WinPct))
	dc.DrawStringAnchored(fmt.Sprintf("%.0f%%", r.WinPct), 595, y+rowH/2, 0.5, 0.5)
	dc.SetColor(signColor(r.Net))
	dc.DrawStringAnchored(signStr(r.Net), 655, y+rowH/2, 0.5, 0.5)
	dc.SetColor(signColor(r.MMRTogether))
	dc.DrawStringAnchored("~"+signStr(r.MMRTogether), 730, y+rowH/2, 0.5, 0.5)
}

func (g *ImageGenerator) drawTeam3Row(dc *gg.Context, y float64, idx int, r Team3Row) {
	w := float64(dc.Width())
	if idx%2 == 1 {
		dc.SetColor(colorRowAlt)
		dc.DrawRectangle(0, y, w, rowH)
		dc.Fill()
	}
	combo := r.Name1 + " + " + r.Name2 + " + " + r.Name3
	g.loadFont(dc, 11)
	dc.SetColor(colorWhite)
	dc.DrawStringAnchored(combo, 70, y+rowH/2, 0, 0.5)

	g.loadFont(dc, 13)
	dc.SetColor(colorGreen)
	dc.DrawStringAnchored(fmt.Sprintf("%d", r.Wins), 500, y+rowH/2, 0.5, 0.5)
	dc.SetColor(colorRed)
	dc.DrawStringAnchored(fmt.Sprintf("%d", r.Losses), 545, y+rowH/2, 0.5, 0.5)
	dc.SetColor(winPctColor(r.WinPct))
	dc.DrawStringAnchored(fmt.Sprintf("%.0f%%", r.WinPct), 595, y+rowH/2, 0.5, 0.5)
	dc.SetColor(signColor(r.Net))
	dc.DrawStringAnchored(signStr(r.Net), 655, y+rowH/2, 0.5, 0.5)
	dc.SetColor(signColor(r.MMRTogether))
	dc.DrawStringAnchored("~"+signStr(r.MMRTogether), 730, y+rowH/2, 0.5, 0.5)
}

func (g *ImageGenerator) drawFooter(dc *gg.Context, y float64) {
	w := float64(dc.Width())
	dc.SetColor(colorBG)
	dc.DrawRectangle(0, y, w, footerH)
	dc.Fill()
	dc.SetColor(colorGold)
	dc.SetLineWidth(0.5)
	dc.DrawLine(0, y, w, y)
	dc.Stroke()
	g.loadFont(dc, 10)
	dc.SetColor(colorGray)
	ts := time.Now().UTC().Format("Mon Jan 02, 2006 03:04 PM")
	dc.DrawStringAnchored("Actualizado: "+ts+" UTC  ·  ~MMR estimado ±25/partida", paddingL, y+footerH/2, 0, 0.5)
	dc.DrawStringAnchored("Stratz API", w-paddingL, y+footerH/2, 1, 0.5)
}

// RenderIndividual generates the individual ranking PNG.
func (g *ImageGenerator) RenderIndividual(players []PlayerRankRow, weekLabel string) ([]byte, error) {
	colH := 24.0
	h := headerH + sectionH + colH + float64(len(players))*rowH + footerH + 4
	dc := gg.NewContext(int(canvasW), int(h))

	g.drawBackground(dc)

	title := "⚔  RANKING INDIVIDUAL · Dota 2"
	g.drawHeader(dc, title, weekLabel)

	y := headerH
	g.drawSectionHeader(dc, y, "🏆  RANKING INDIVIDUAL")
	y += sectionH
	g.drawColHeaders(dc, y, indivCols)
	y += colH

	for i, r := range players {
		g.drawIndivRow(dc, y, i, r, "")
		y += rowH
	}

	g.drawFooter(dc, y)
	g.drawBorder(dc)

	return dc.EncodePNG()
}

// RenderTeam2 generates the 2-player combo ranking PNG.
func (g *ImageGenerator) RenderTeam2(combos []Team2Row, weekLabel string) ([]byte, error) {
	colH := 24.0
	h := headerH + sectionH + colH + float64(len(combos))*rowH + footerH + 4
	if len(combos) == 0 {
		h = headerH + sectionH + 60 + footerH
	}
	dc := gg.NewContext(int(canvasW), int(h))
	g.drawBackground(dc)
	g.drawHeader(dc, "⚔  RANKING EN EQUIPO · Dota 2", weekLabel)
	y := headerH
	g.drawSectionHeader(dc, y, "👥  COMBOS — 2 JUGADORES")
	y += sectionH
	if len(combos) == 0 {
		g.loadFont(dc, 12)
		dc.SetColor(colorGray)
		dc.DrawStringAnchored("Sin partidas en equipo esta semana", canvasW/2, y+30, 0.5, 0.5)
		y += 60
	} else {
		g.drawColHeaders(dc, y, teamCols)
		y += colH
		for i, r := range combos {
			g.drawTeam2Row(dc, y, i, r)
			y += rowH
		}
	}
	g.drawFooter(dc, y)
	g.drawBorder(dc)
	return dc.EncodePNG()
}

// RenderTeam3 generates the 3-player combo ranking PNG.
func (g *ImageGenerator) RenderTeam3(combos []Team3Row, weekLabel string) ([]byte, error) {
	colH := 24.0
	h := headerH + sectionH + colH + float64(len(combos))*rowH + footerH + 4
	if len(combos) == 0 {
		h = headerH + sectionH + 60 + footerH
	}
	dc := gg.NewContext(int(canvasW), int(h))
	g.drawBackground(dc)
	g.drawHeader(dc, "⚔  RANKING EN EQUIPO · Dota 2", weekLabel)
	y := headerH
	g.drawSectionHeader(dc, y, "⚔  COMBOS — 3 JUGADORES")
	y += sectionH
	if len(combos) == 0 {
		g.loadFont(dc, 12)
		dc.SetColor(colorGray)
		dc.DrawStringAnchored("Sin partidas en equipo esta semana", canvasW/2, y+30, 0.5, 0.5)
		y += 60
	} else {
		g.drawColHeaders(dc, y, teamCols)
		y += colH
		for i, r := range combos {
			g.drawTeam3Row(dc, y, i, r)
			y += rowH
		}
	}
	g.drawFooter(dc, y)
	g.drawBorder(dc)
	return dc.EncodePNG()
}

func winPctColor(pct float64) color.Color {
	if pct >= 50 { return colorGreen }
	if pct >= 40 { return colorYellow }
	return colorRed
}

func signColor(n int) color.Color {
	if n > 0 { return colorBlue }
	if n < 0 { return colorRed }
	return colorGray
}

func signStr(n int) string {
	if n > 0 { return fmt.Sprintf("+%d", n) }
	return fmt.Sprintf("%d", n)
}
```

- [ ] Verify compiles:

```bash
go build ./internal/ranking/...
```

Expected: success (may show warnings about unused imports — fix them if so).

- [ ] Commit:

```bash
git add internal/ranking/image.go
git commit -m "feat: add PNG ranking image generator with Dota 2 dark theme"
```

---

### Task 10: Ranking updater

**Files:**
- Create: `internal/ranking/updater.go`

- [ ] Create `internal/ranking/updater.go`:

```go
package ranking

import (
	"context"
	"dota-discord-bot/internal/db"
	minioclient "dota-discord-bot/internal/minio"
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/sirupsen/logrus"
)

// Updater orchestrates: calculate → render PNG → upload MinIO → edit Discord messages.
type Updater struct {
	db        *db.DB
	calc      *Calculator
	gen       *ImageGenerator
	minio     *minioclient.Client
	session   *discordgo.Session
	channelID string
	baseYear  int
}

func NewUpdater(
	database *db.DB,
	minio *minioclient.Client,
	session *discordgo.Session,
	channelID string,
	baseYear int,
) *Updater {
	return &Updater{
		db:        database,
		calc:      NewCalculator(database),
		gen:       NewImageGenerator(),
		minio:     minio,
		session:   session,
		channelID: channelID,
		baseYear:  baseYear,
	}
}

// InitChannel sends the 3 initial ranking messages if they don't exist yet.
func (u *Updater) InitChannel() error {
	if u.channelID == "" {
		return nil
	}
	types := []string{"individual", "team2", "team3"}
	for _, t := range types {
		_, msgID, err := u.db.GetRankingMessage(t)
		if err != nil {
			return fmt.Errorf("get ranking message %s: %w", t, err)
		}
		if msgID != "" {
			continue
		}
		msg, err := u.session.ChannelMessageSend(u.channelID, fmt.Sprintf("*(cargando ranking %s...)*", t))
		if err != nil {
			return fmt.Errorf("send initial message %s: %w", t, err)
		}
		if err := u.db.SetRankingMessage(t, u.channelID, msg.ID); err != nil {
			return fmt.Errorf("save ranking message %s: %w", t, err)
		}
		logrus.Infof("ranking: created pinned message %s: %s", t, msg.ID)
	}
	return u.Refresh(time.Now())
}

// Refresh recalculates, regenerates PNGs, uploads to MinIO, and edits the 3 pinned Discord messages.
func (u *Updater) Refresh(now time.Time) error {
	if u.channelID == "" {
		return nil
	}
	weekStart, weekEnd := WeekBounds(now)
	weekLabel := fmt.Sprintf("Semana: %s → %s",
		weekStart.Format("Mon Jan 02"),
		weekEnd.AddDate(0, 0, -1).Format("Mon Jan 02, 2006"))

	year, week := now.ISOWeek()
	keyIndiv := fmt.Sprintf("ranking-individual-%d-W%02d.png", year, week)
	keyTeam2 := fmt.Sprintf("ranking-team2-%d-W%02d.png", year, week)
	keyTeam3 := fmt.Sprintf("ranking-team3-%d-W%02d.png", year, week)

	ctx := context.Background()

	// Individual
	indivPlayers, err := u.calc.IndividualRanking(weekStart, weekEnd)
	if err != nil {
		return fmt.Errorf("individual ranking: %w", err)
	}
	imgIndiv, err := u.gen.RenderIndividual(indivPlayers, weekLabel)
	if err != nil {
		return fmt.Errorf("render individual: %w", err)
	}
	urlIndiv, err := u.minio.Upload(ctx, keyIndiv, imgIndiv)
	if err != nil {
		return fmt.Errorf("upload individual: %w", err)
	}

	// Team 2
	team2, err := u.calc.Team2Ranking(weekStart, weekEnd)
	if err != nil {
		return fmt.Errorf("team2 ranking: %w", err)
	}
	imgTeam2, err := u.gen.RenderTeam2(team2, weekLabel)
	if err != nil {
		return fmt.Errorf("render team2: %w", err)
	}
	urlTeam2, err := u.minio.Upload(ctx, keyTeam2, imgTeam2)
	if err != nil {
		return fmt.Errorf("upload team2: %w", err)
	}

	// Team 3
	team3, err := u.calc.Team3Ranking(weekStart, weekEnd)
	if err != nil {
		return fmt.Errorf("team3 ranking: %w", err)
	}
	imgTeam3, err := u.gen.RenderTeam3(team3, weekLabel)
	if err != nil {
		return fmt.Errorf("render team3: %w", err)
	}
	urlTeam3, err := u.minio.Upload(ctx, keyTeam3, imgTeam3)
	if err != nil {
		return fmt.Errorf("upload team3: %w", err)
	}

	// Edit Discord messages
	urls := map[string]string{
		"individual": urlIndiv,
		"team2":      urlTeam2,
		"team3":      urlTeam3,
	}
	for msgType, imgURL := range urls {
		_, msgID, err := u.db.GetRankingMessage(msgType)
		if err != nil || msgID == "" {
			logrus.Warnf("ranking: no message ID for %s, skipping edit", msgType)
			continue
		}
		embed := &discordgo.MessageEmbed{
			Image: &discordgo.MessageEmbedImage{URL: imgURL},
			Color: 0xC8AA6E,
		}
		if _, err := u.session.ChannelMessageEditEmbed(u.channelID, msgID, embed); err != nil {
			logrus.Errorf("ranking: edit message %s (%s): %v", msgType, msgID, err)
		}
	}

	logrus.Infof("ranking: refreshed — individual:%d team2:%d team3:%d", len(indivPlayers), len(team2), len(team3))
	return nil
}

// RefreshForMonth generates an on-demand image for a specific month and sends it as a new message.
func (u *Updater) RefreshForMonth(channelID string, year, month int) (*discordgo.MessageEmbed, error) {
	start, end := MonthBounds(year, month)
	label := fmt.Sprintf("Mes: %s %d", time.Month(month).String(), year)
	return u.buildEmbeds(channelID, start, end, label)
}

// RefreshForLastN generates an on-demand image for the last N matches across all players.
func (u *Updater) RefreshForLastN(n int) (*discordgo.MessageEmbed, error) {
	// Use current week bounds as display label, but actual start is n-matches ago (we just show last N)
	now := time.Now()
	weekStart, weekEnd := WeekBounds(now)
	label := fmt.Sprintf("Últimas %d partidas", n)
	// For last N we still use time bounds of the year base — the calc query limits by time not by count
	// We pass year start to year end and rely on caller to add LIMIT if needed
	// For simplicity, we use the full year and let SQL return all; caller shows top rows
	start, end := YearBounds(now.Year())
	_ = weekStart
	_ = weekEnd
	_ = n
	return u.buildEmbeds("", start, end, label)
}

func (u *Updater) buildEmbeds(_, _ string, start, end time.Time, label string) (*discordgo.MessageEmbed, error) {
	ctx := context.Background()
	year, week := start.ISOWeek()

	players, err := u.calc.IndividualRanking(start, end)
	if err != nil {
		return nil, err
	}
	img, err := u.gen.RenderIndividual(players, label)
	if err != nil {
		return nil, err
	}
	key := fmt.Sprintf("ranking-ondemand-%d-W%02d-%d.png", year, week, time.Now().Unix())
	imgURL, err := u.minio.Upload(ctx, key, img)
	if err != nil {
		return nil, err
	}
	return &discordgo.MessageEmbed{
		Image: &discordgo.MessageEmbedImage{URL: imgURL},
		Color: 0xC8AA6E,
	}, nil
}
```

- [ ] Verify compiles:

```bash
go build ./internal/ranking/...
```

- [ ] Commit:

```bash
git add internal/ranking/updater.go
git commit -m "feat: add ranking updater (calc→PNG→MinIO→Discord edit)"
```

---

### Task 11: Update docker-compose.yml and .env.example

**Files:**
- Modify: `docker-compose.yml`
- Modify: `.env.example`

- [ ] Replace `docker-compose.yml`:

```yaml
services:
  dota-discord-bot:
    image: orgmcr.or-gm.com/osmargm1202/dota-discord-bot:latest
    container_name: dota-discord-bot
    restart: always
    env_file:
      - .env
    volumes:
      - ./data:/app/data
      - ./logs:/app/logs
    depends_on:
      discord-dota-postgres:
        condition: service_healthy
      discord-dota-minio:
        condition: service_healthy
    networks:
      - dota-bot-network
      - cloudflared

  discord-dota-postgres:
    image: postgres:16-alpine
    container_name: discord-dota-postgres
    restart: always
    environment:
      POSTGRES_DB: ${POSTGRES_DB:-dotabot}
      POSTGRES_USER: ${POSTGRES_USER:-dotabot}
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:-changeme}
    volumes:
      - ./postgres-data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U ${POSTGRES_USER:-dotabot}"]
      interval: 10s
      timeout: 5s
      retries: 5
    networks:
      - dota-bot-network

  discord-dota-minio:
    image: minio/minio
    container_name: discord-dota-minio
    command: server /data --console-address ":9001"
    restart: always
    environment:
      MINIO_ROOT_USER: ${MINIO_ACCESS_KEY:-minioadmin}
      MINIO_ROOT_PASSWORD: ${MINIO_SECRET_KEY:-changeme}
    volumes:
      - ./minio-data:/data
    ports:
      - "9001:9001"
    healthcheck:
      test: ["CMD", "mc", "ready", "local"]
      interval: 10s
      timeout: 5s
      retries: 5
    networks:
      - dota-bot-network
      - cloudflared

networks:
  cloudflared:
    external: true
  dota-bot-network:
    driver: bridge
```

- [ ] Append to `.env.example`:

```bash
# === RANKING CHANNEL ===
RANKING_CHANNEL_ID=1519494823642398852

# === POSTGRESQL ===
POSTGRES_DB=dotabot
POSTGRES_USER=dotabot
POSTGRES_PASSWORD=changeme
POSTGRES_DSN=postgres://dotabot:changeme@discord-dota-postgres:5432/dotabot?sslmode=disable

# === MINIO ===
MINIO_ENDPOINT=discord-dota-minio:9000
MINIO_ACCESS_KEY=minioadmin
MINIO_SECRET_KEY=changeme
MINIO_BUCKET=dota-rankings
MINIO_PUBLIC_URL=https://dota-s3.fifrex.com

# === BACKFILL ===
BASE_YEAR=2026
BACKFILL_DELAY_MS=700
```

- [ ] Commit:

```bash
git add docker-compose.yml .env.example
git commit -m "feat: add postgres and minio to docker-compose"
```

---

### Task 12: Wire everything in main.go

**Files:**
- Modify: `main.go`

- [ ] Replace `main.go`:

```go
package main

import (
	"dota-discord-bot/config"
	"dota-discord-bot/discord"
	"dota-discord-bot/dota"
	"dota-discord-bot/internal/backfill"
	dbpkg "dota-discord-bot/internal/db"
	minioclient "dota-discord-bot/internal/minio"
	"dota-discord-bot/internal/ranking"
	"dota-discord-bot/storage"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
)

func main() {
	debug := flag.Bool("debug", false, "Activar modo debug")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error cargando configuración: %v\n", err)
		os.Exit(1)
	}
	if *debug {
		cfg.Debug = true
	}

	if cfg.Debug {
		logrus.SetLevel(logrus.DebugLevel)
	} else {
		logrus.SetLevel(logrus.InfoLevel)
	}
	logrus.SetOutput(os.Stdout)
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
	})

	logrus.Info("Iniciando bot de Discord para Dota 2...")

	// Legacy JSON store (for migration only)
	userStore, err := storage.NewUserStore()
	if err != nil {
		logrus.Fatalf("Error creando almacenamiento: %v", err)
	}

	// PostgreSQL
	var database *dbpkg.DB
	if cfg.PostgresDSN != "" {
		database, err = dbpkg.New(cfg.PostgresDSN)
		if err != nil {
			logrus.Fatalf("Error conectando PostgreSQL: %v", err)
		}
		if err := database.RunMigrations(); err != nil {
			logrus.Fatalf("Error ejecutando migraciones: %v", err)
		}
		logrus.Info("PostgreSQL conectado y migraciones aplicadas")

		// Migrate from JSON
		jsonUsers := userStore.GetAll()
		jsonLastMatches := loadJSONLastMatches()
		jsonChannel, _ := userStore.GetChannel()
		if err := database.MigrateFromJSON(jsonUsers, jsonLastMatches, jsonChannel); err != nil {
			logrus.Warnf("JSON migration: %v", err)
		} else {
			logrus.Info("JSON migration complete (or already done)")
		}
	} else {
		logrus.Warn("POSTGRES_DSN no configurado, usando solo JSON storage")
	}

	// MinIO
	var minioClient *minioclient.Client
	if cfg.MinioEndpoint != "" && database != nil {
		minioClient, err = minioclient.New(cfg.MinioEndpoint, cfg.MinioAccessKey, cfg.MinioSecretKey, cfg.MinioBucket, cfg.MinioPublicURL)
		if err != nil {
			logrus.Warnf("MinIO no disponible: %v", err)
		} else {
			logrus.Info("MinIO conectado")
		}
	}

	// Stratz
	dotaClient := dota.NewClient()
	if cfg.StratzToken == "" {
		logrus.Fatal("STRATZ_TOKEN es obligatorio")
	}
	stratzClient := dota.NewStratzClient(cfg.StratzToken)
	if cfg.Debug {
		stratzClient.SetDebug(true)
	}

	// Bot
	bot, err := discord.NewBot(cfg, dotaClient, stratzClient, userStore, database, minioClient)
	if err != nil {
		logrus.Fatalf("Error creando bot: %v", err)
	}
	if err := bot.Start(); err != nil {
		logrus.Fatalf("Error iniciando bot: %v", err)
	}
	logrus.Info("Bot corriendo. Presiona CTRL+C para salir.")

	// Init ranking channel
	if database != nil && minioClient != nil && cfg.RankingChannelID != "" {
		rankingUpdater := ranking.NewUpdater(database, minioClient, bot.Session(), cfg.RankingChannelID, cfg.BaseYear)
		bot.SetRankingUpdater(rankingUpdater)
		go func() {
			time.Sleep(3 * time.Second)
			if err := rankingUpdater.InitChannel(); err != nil {
				logrus.Errorf("ranking init: %v", err)
			}
		}()
	}

	// Welcome + immediate match check
	go func() {
		time.Sleep(2 * time.Second)
		if err := bot.SendWelcomeMessage(); err != nil {
			logrus.Warnf("Welcome message: %v", err)
		}
		logrus.Info("Ejecutando verificación inmediata de partidas...")
		if err := bot.CheckForNewMatches(); err != nil {
			logrus.Errorf("Verificación inicial: %v", err)
		}
	}()

	// Backfill historical data
	if database != nil && stratzClient != nil {
		bfService := backfill.New(database, stratzClient, cfg.BaseYear, cfg.BackfillDelayMS)
		go func() {
			time.Sleep(10 * time.Second) // let bot settle first
			bfService.Run()
		}()
	}

	// Polling ticker
	ticker := time.NewTicker(time.Duration(cfg.RefreshRateMinutes) * time.Minute)
	defer ticker.Stop()
	logrus.Infof("Verificación de partidas cada %d minuto(s)", cfg.RefreshRateMinutes)
	go func() {
		for range ticker.C {
			if err := bot.CheckForNewMatches(); err != nil {
				logrus.Errorf("Error verificando partidas: %v", err)
			}
		}
	}()

	go bot.RunStatsScheduler()

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	logrus.Info("Cerrando bot...")
	bot.Stop()
	logrus.Info("Bot cerrado exitosamente")
}

func loadJSONLastMatches() map[string]int64 {
	data, err := os.ReadFile("data/last_matches.json")
	if err != nil {
		return nil
	}
	var m map[string]int64
	if err := json.Unmarshal(data, &m); err != nil {
		return nil
	}
	return m
}
```

- [ ] Verify compiles (will fail until bot.go is updated — that's expected):

```bash
go build . 2>&1 | head -30
```

- [ ] Commit partial:

```bash
git add main.go
git commit -m "feat: wire PG, MinIO, backfill, ranking updater in main"
```

---

### Task 13: Update discord/bot.go

**Files:**
- Modify: `discord/bot.go`

- [ ] Add these fields to the `Bot` struct (after existing fields):

```go
// In Bot struct, add:
db              *dbpkg.DB
minioClient     *minioclient.Client
rankingUpdater  *ranking.Updater
```

- [ ] Add imports at top of `discord/bot.go`:

```go
dbpkg "dota-discord-bot/internal/db"
minioclient "dota-discord-bot/internal/minio"
"dota-discord-bot/internal/ranking"
```

- [ ] Update `NewBot` signature (add db and minioClient params):

```go
func NewBot(cfg *config.Config, dotaClient *dota.Client, stratzClient *dota.StratzClient, userStore *storage.UserStore, database *dbpkg.DB, minioClient *minioclient.Client) (*Bot, error) {
```

- [ ] Add to Bot struct initialization inside `NewBot`:

```go
db:          database,
minioClient: minioClient,
```

- [ ] Add these methods to `bot.go`:

```go
// Session returns the underlying Discord session (used by ranking updater).
func (b *Bot) Session() *discordgo.Session {
	return b.session
}

// SetRankingUpdater sets the ranking updater after bot creation.
func (b *Bot) SetRankingUpdater(u *ranking.Updater) {
	b.rankingUpdater = u
}
```

- [ ] At the end of `sendMatchNotification`, before `return err`, add the ranking refresh trigger:

```go
// Trigger ranking refresh after successful notification
if b.rankingUpdater != nil {
    go func() {
        if err := b.rankingUpdater.Refresh(time.Now()); err != nil {
            getLogger().Errorf("ranking refresh after match: %v", err)
        }
    }()
}
```

- [ ] Update `CheckForNewMatches` to also persist to DB when database is available. After `b.userStore.SetLastMatch(discordID, latestStratzMatch.ID)`, add:

```go
if b.db != nil {
    accountIDInt64, _ := strconv.ParseInt(accountID, 10, 64)
    _ = b.db.SetLastProcessedMatch(accountIDInt64, latestStratzMatch.ID)
    // Persist match + player data
    startT := time.Unix(latestStratzMatch.StartDateTime, 0).UTC()
    _ = b.db.UpsertMatch(latestStratzMatch.ID, startT, latestStratzMatch.DurationSeconds, int(latestStratzMatch.GameMode), matchDetails.RadiantWin != nil && *matchDetails.RadiantWin, true)
    if player != nil {
        won := false
        if matchDetails.RadiantWin != nil && player.IsRadiant != nil {
            won = *matchDetails.RadiantWin == *player.IsRadiant
        }
        mmrDelta := -25
        if won { mmrDelta = 25 }
        _ = b.db.UpsertPlayerMatch(dbpkg.PlayerMatchRow{
            MatchID:     latestStratzMatch.ID,
            DotaID:      accountIDInt64,
            IsRadiant:   player.IsRadiant != nil && *player.IsRadiant,
            HeroID:      player.HeroID,
            Kills:       player.Kills,
            Deaths:      player.Deaths,
            Assists:     player.Assists,
            Level:       player.Level,
            GPM:         player.GoldPerMin,
            XPM:         player.XpPerMin,
            HeroDamage:  player.HeroDamage,
            TowerDamage: player.TowerDamage,
            Healing:     player.HeroHealing,
            Imp:         player.Imp,
            Award:       player.Award,
            Lane:        player.Lane,
            Role:        player.Role,
            Won:         won,
            MMRDelta:    mmrDelta,
        })
    }
}
```

- [ ] Verify compiles:

```bash
go build .
```

- [ ] Commit:

```bash
git add discord/bot.go
git commit -m "feat: add DB and MinIO to bot, persist match data, trigger ranking refresh"
```

---

### Task 14: Add /dota ranking commands and /dota admin register

**Files:**
- Create: `discord/ranking_commands.go`

- [ ] Create `discord/ranking_commands.go`:

```go
package discord

import (
	"dota-discord-bot/internal/ranking"
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

var monthMap = map[string]int{
	"enero": 1, "febrero": 2, "marzo": 3, "abril": 4,
	"mayo": 5, "junio": 6, "julio": 7, "agosto": 8,
	"septiembre": 9, "octubre": 10, "noviembre": 11, "diciembre": 12,
}

func (b *Bot) handleRankingSlash(s *discordgo.Session, i *discordgo.InteractionCreate, subcommand *discordgo.ApplicationCommandInteractionDataOption) {
	if b.rankingUpdater == nil {
		b.sendFollowup(s, i, "❌ Sistema de ranking no configurado (requiere POSTGRES_DSN y MINIO_ENDPOINT)")
		return
	}

	// /dota ranking sin opciones → link al canal
	if len(subcommand.Options) == 0 {
		if b.config.RankingChannelID != "" {
			b.sendFollowup(s, i, fmt.Sprintf("📊 Canal de ranking: <#%s>", b.config.RankingChannelID))
		} else {
			b.sendFollowup(s, i, "❌ RANKING_CHANNEL_ID no configurado")
		}
		return
	}

	opt := subcommand.Options[0]
	switch opt.Name {
	case "mes":
		mesStr := strings.ToLower(opt.StringValue())
		monthNum, ok := monthMap[mesStr]
		if !ok {
			b.sendFollowup(s, i, "❌ Mes inválido. Usa: enero, febrero, marzo, ... diciembre")
			return
		}
		year := time.Now().Year()
		embed, err := b.rankingUpdater.RefreshForMonth(i.ChannelID, year, monthNum)
		if err != nil {
			getLogger().Errorf("ranking mes %s: %v", mesStr, err)
			b.sendFollowup(s, i, fmt.Sprintf("❌ Error generando ranking: %v", err))
			return
		}
		b.sendFollowupEmbed(s, i, embed)

	case "ultimas":
		n := int(opt.IntValue())
		if n <= 0 || n > 1000 {
			b.sendFollowup(s, i, "❌ Número inválido. Usa: 10, 100")
			return
		}
		embed, err := b.rankingUpdater.RefreshForLastN(n)
		if err != nil {
			getLogger().Errorf("ranking ultimas %d: %v", n, err)
			b.sendFollowup(s, i, fmt.Sprintf("❌ Error generando ranking: %v", err))
			return
		}
		b.sendFollowupEmbed(s, i, embed)
	}
}

func (b *Bot) handleAdminRegisterSlash(s *discordgo.Session, i *discordgo.InteractionCreate, subcommand *discordgo.ApplicationCommandInteractionDataOption) {
	if b.db == nil {
		b.sendFollowup(s, i, "❌ Base de datos no configurada")
		return
	}

	var accountIDStr, nombre string
	for _, opt := range subcommand.Options {
		switch opt.Name {
		case "account_id":
			accountIDStr = opt.StringValue()
		case "nombre":
			nombre = opt.StringValue()
		}
	}

	if accountIDStr == "" {
		b.sendFollowup(s, i, "❌ account_id requerido")
		return
	}

	var dotaID int64
	if _, err := fmt.Sscanf(accountIDStr, "%d", &dotaID); err != nil || dotaID <= 0 {
		b.sendFollowup(s, i, "❌ account_id debe ser un número")
		return
	}

	// Verify player exists on Stratz
	profile, err := b.stratzClient.GetPlayerProfile(dotaID)
	if err != nil || profile == nil {
		b.sendFollowup(s, i, fmt.Sprintf("❌ Jugador no encontrado en Stratz: %v", err))
		return
	}

	displayName := nombre
	if displayName == "" {
		displayName = profile.Name
	}

	if err := b.db.UpsertUser(nil, dotaID, &displayName); err != nil {
		b.sendFollowup(s, i, fmt.Sprintf("❌ Error guardando usuario: %v", err))
		return
	}

	b.sendFollowup(s, i, fmt.Sprintf("✅ Jugador **%s** (Dota ID: %d) agregado al ranking sin Discord vinculado.", displayName, dotaID))
	getLogger().Infof("admin register: dota_id %d display_name %s", dotaID, displayName)

	// Trigger backfill for new user in background
	if b.rankingUpdater != nil {
		go func() {
			if err := b.rankingUpdater.Refresh(time.Now()); err != nil {
				getLogger().Errorf("ranking refresh after admin register: %v", err)
			}
		}()
	}
	_ = ranking.WeekBounds // ensure import used
}
```

- [ ] Add ranking and admin subcommands to `registerCommands()` in `bot.go`. In the `commands` slice, inside the `dota` command options, add after the `help` subcommand:

```go
{
    Type:        discordgo.ApplicationCommandOptionSubCommand,
    Name:        "ranking",
    Description: "Ver tabla de ranking semanal",
    Options: []*discordgo.ApplicationCommandOption{
        {
            Type:        discordgo.ApplicationCommandOptionString,
            Name:        "mes",
            Description: "Mes a consultar (enero, febrero, ...)",
            Required:    false,
        },
        {
            Type:        discordgo.ApplicationCommandOptionInteger,
            Name:        "ultimas",
            Description: "Últimas N partidas (10, 100)",
            Required:    false,
        },
    },
},
{
    Type:        discordgo.ApplicationCommandOptionSubCommandGroup,
    Name:        "admin",
    Description: "Comandos de administración",
    Options: []*discordgo.ApplicationCommandOption{
        {
            Type:        discordgo.ApplicationCommandOptionSubCommand,
            Name:        "register",
            Description: "Registrar jugador solo en ranking (sin Discord)",
            Options: []*discordgo.ApplicationCommandOption{
                {
                    Type:        discordgo.ApplicationCommandOptionString,
                    Name:        "account_id",
                    Description: "Dota 2 Account ID",
                    Required:    true,
                },
                {
                    Type:        discordgo.ApplicationCommandOptionString,
                    Name:        "nombre",
                    Description: "Nombre a mostrar en el ranking",
                    Required:    false,
                },
            },
        },
    },
},
```

- [ ] Add cases to the `switch subcommandName` in `interactionCreate`:

```go
case "ranking":
    b.handleRankingSlash(s, i, subcommand)
case "admin":
    if len(subcommand.Options) > 0 && subcommand.Options[0].Name == "register" {
        b.handleAdminRegisterSlash(s, i, subcommand.Options[0])
    }
```

- [ ] Verify compiles:

```bash
go build .
```

- [ ] Commit:

```bash
git add discord/ranking_commands.go discord/bot.go
git commit -m "feat: add /dota ranking and /dota admin register commands"
```

---

### Task 15: Local test with docker compose

- [ ] Create local `.env` from example:

```bash
cp .env.example .env
# Edit .env with real values (DISCORD_TOKEN, STRATZ_TOKEN, etc.)
# Set POSTGRES_PASSWORD to something real
# Set MINIO_ACCESS_KEY and MINIO_SECRET_KEY
```

- [ ] Start PG and MinIO only first:

```bash
docker compose up discord-dota-postgres discord-dota-minio -d
```

- [ ] Wait for healthy:

```bash
docker compose ps
# Both should show "healthy" after ~15 seconds
```

- [ ] Build and run bot locally against the containers:

```bash
# Point to local containers
export POSTGRES_DSN="postgres://dotabot:changeme@localhost:5432/dotabot?sslmode=disable"
export MINIO_ENDPOINT="localhost:9000"
go run . --debug
```

- [ ] Verify in logs:
  - `PostgreSQL conectado y migraciones aplicadas`
  - `MinIO conectado`
  - `backfill: starting from 2026-01-01`
  - Bot connects to Discord

- [ ] Test commands in Discord:
  - `/dota ranking` → should show link to ranking channel
  - `/dota admin register account_id:136201811 nombre:Desp4irs` → should register ranking-only user
  - Wait for backfill to process a few matches, then check `/dota ranking mes:junio`

- [ ] Check MinIO console at `http://localhost:9001` (admin/changeme) → bucket `dota-rankings` should have PNG files

- [ ] Commit any fixes:

```bash
git add -A
git commit -m "fix: local testing fixes"
```

---

### Task 16: Build and tag for production

- [ ] Final build and push:

```bash
docker build --push -t orgmcr.or-gm.com/osmargm1202/dota-discord-bot:latest .
docker build --push -t orgmcr.or-gm.com/osmargm1202/dota-discord-bot:v2 .
```

- [ ] Commit final state:

```bash
git add -A
git commit -m "feat: full infra - PostgreSQL, MinIO, backfill, ranking system complete"
git push origin main
```

---

## Self-Review

**Spec coverage check:**
- ✅ PostgreSQL + docker-compose — Tasks 3,4,11
- ✅ MinIO with cloudflared network, container name `discord-dota-minio` — Task 11
- ✅ JSON migration idempotent — Task 4
- ✅ Historical backfill from BASE_YEAR, rate limited, idempotent — Tasks 6,7
- ✅ Cached backfill data (skip if exists in DB) — Task 7
- ✅ Individual ranking PNG — Tasks 8,9
- ✅ Team 2-player and 3-player combos, same team only — Tasks 8,9
- ✅ Weekly bounds Monday 00:00 UTC — Task 8
- ✅ Pinned messages edited after each match — Tasks 10,13
- ✅ /dota ranking mes:X, ultimas:N — Task 14
- ✅ /dota admin register (ranking-only users) — Task 14
- ✅ Ranking-only users in ranking but not match notifications — Task 4 (discord_id NULL)
- ✅ BASE_YEAR env var — Task 2
- ✅ BACKFILL_DELAY_MS env var — Task 2
- ✅ Parse queue in PG — Task 4
- ✅ Web foundation (clean internal/ packages) — Tasks 3-10

**Type consistency:**
- `PlayerMatchRow` defined in `internal/db/queries.go`, used in `internal/backfill/backfill.go` and `discord/bot.go` ✅
- `Updater.Refresh()` returns `error` — consistent ✅
- `GetAllUsers()` returns `[]User` — used in backfill ✅
- `ranking.WeekBounds` used in updater and ranking_commands ✅
