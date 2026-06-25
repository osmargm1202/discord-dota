# Discord Dota Bot — Full Infrastructure & Ranking System

**Date:** 2026-06-25
**Status:** Approved

---

## Overview

Add PostgreSQL, MinIO, and a ranking system to the Discord Dota bot. Migrate from JSON file storage to PostgreSQL. Generate PNG ranking images via `fogleman/gg` served from MinIO. Add a dedicated ranking channel that shows individual and team combo leaderboards, updated after every match.

---

## Infrastructure

### Docker Compose Services

- `dota-discord-bot` — Go bot (existing)
- `discord-dota-postgres` — PostgreSQL 16 Alpine
- `discord-dota-minio` — MinIO, container name required by CloudFlare tunnel

### Networks

- `dota-bot-network` — bridge, internal
- `cloudflared` — external, existing; bot and minio join it

### MinIO public URL

`https://dota-s3.fifrex.com` via CloudFlare tunnel pointing to `discord-dota-minio:9000`

---

## New Environment Variables

```env
# Ranking channel
RANKING_CHANNEL_ID=1519494823642398852

# PostgreSQL
POSTGRES_DSN=postgres://dotabot:changeme@discord-dota-postgres:5432/dotabot

# MinIO
MINIO_ENDPOINT=discord-dota-minio:9000
MINIO_ACCESS_KEY=minioadmin
MINIO_SECRET_KEY=changeme
MINIO_BUCKET=dota-rankings
MINIO_PUBLIC_URL=https://dota-s3.fifrex.com

# Historical backfill
BASE_YEAR=2026                  # year from which to start counting stats (resets Jan 1 next year)
BACKFILL_DELAY_MS=700           # ms between API calls during backfill (default 700, ~85 req/min)
```

---

## Database Schema

```sql
-- Users: discord_id NULL = ranking-only (no match notifications)
CREATE TABLE users (
    id            SERIAL PRIMARY KEY,
    discord_id    VARCHAR(20) UNIQUE,
    dota_id       BIGINT NOT NULL UNIQUE,
    display_name  VARCHAR(100),
    registered_at TIMESTAMPTZ DEFAULT NOW(),
    active        BOOLEAN DEFAULT TRUE
);

-- Match metadata (dedup key)
CREATE TABLE matches (
    match_id      BIGINT PRIMARY KEY,
    start_time    TIMESTAMPTZ NOT NULL,
    duration_secs INT,
    game_mode     INT,
    radiant_win   BOOLEAN,
    parsed        BOOLEAN DEFAULT FALSE,
    created_at    TIMESTAMPTZ DEFAULT NOW()
);

-- Parse queue — persists across restarts (replaces in-memory logic)
CREATE TABLE parse_queue (
    match_id      BIGINT PRIMARY KEY REFERENCES matches(match_id),
    enqueued_at   TIMESTAMPTZ DEFAULT NOW(),
    last_attempt  TIMESTAMPTZ,
    attempt_count INT DEFAULT 0,
    status        VARCHAR(20) DEFAULT 'pending'  -- pending / done / failed
);

-- Per-player match data (primary stats store)
CREATE TABLE player_matches (
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
    mmr_delta     INT,  -- estimated: +25 win, -25 loss
    created_at    TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (match_id, dota_id)
);

-- Replaces last_matches.json
CREATE TABLE last_processed_match (
    dota_id    BIGINT PRIMARY KEY,
    match_id   BIGINT,
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- IDs of pinned ranking messages (to edit them)
CREATE TABLE ranking_messages (
    message_type VARCHAR(20) PRIMARY KEY,  -- individual / team2 / team3
    channel_id   VARCHAR(20),
    message_id   VARCHAR(20),
    updated_at   TIMESTAMPTZ DEFAULT NOW()
);

-- Replaces notification_channel.json and any future config keys
CREATE TABLE config (
    key        VARCHAR(50) PRIMARY KEY,
    value      TEXT,
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
```

---

## New Package Structure

```
/internal/
  db/
    db.go          -- pg connection pool, RunMigrations()
    migrations.go  -- embedded SQL migrations (run on startup)
    queries.go     -- typed query functions
  ranking/
    calc.go        -- SQL-based ranking calculations
    image.go       -- PNG generation with fogleman/gg
    updater.go     -- orchestrate calc → image → minio → discord edit
  minio/
    client.go      -- upload, public URL
  backfill/
    backfill.go    -- historical match fetch with rate limiting
```

---

## Historical Backfill

### Trigger
Runs once on startup in a background goroutine, after DB migrations and before the polling loop.

### Logic
```
For each user in users table:
  fetch all matches from Stratz where startDateTime >= BASE_YEAR-01-01
  for each match:
    if (match_id, dota_id) already in player_matches → skip (idempotent)
    if match not in matches table → insert
    insert player_matches row
    sleep BACKFILL_DELAY_MS
  log progress every 50 matches
```

### Rate limiting
- `BACKFILL_DELAY_MS=700` default → ~85 req/min (Stratz free tier limit ~100/min)
- Backfill uses paginated Stratz query: `matches(request: { take: 100, skip: N })`
- Fetches in chunks of 100 until no more results or `startDateTime < BASE_YEAR-01-01`
- Skips already-stored matches without API call

### Idempotency
Data already in DB is never re-fetched. Re-running backfill (e.g., after adding a new user) only fetches missing matches.

---

## Ranking Calculation

### Time boundaries
- **Week:** Monday 00:00 → Sunday 23:59 UTC
- **Month:** first day 00:00 → last day 23:59 UTC
- **Base year:** `BASE_YEAR-01-01` to `(BASE_YEAR+1)-01-01` (exclusive)
- Stats reset when calendar year changes (new rows accumulate for new year)

### Individual ranking query
Filter `player_matches` by time range, group by `dota_id`, sort by `net DESC, win% DESC`.

Columns: `#, player, wins, losses, win%, net (+/-), mmr_est`.

### Team combos (same team only)
- **2-player:** self-join `player_matches` on `match_id` where `a.dota_id < b.dota_id` and `a.is_radiant = b.is_radiant`; both must be in `users` table
- **3-player:** same but triple self-join
- Minimum 1 game together to appear
- Columns: `combo, wins, losses, win%, net, mmr_together`

### MMR estimation
`mmr_delta = +25` on win, `-25` on loss. Displayed with `~` prefix to signal approximation.

---

## Image Generation

**Library:** `fogleman/gg` + `golang.org/x/image` for font loading
**Font:** Exo2-Regular.ttf embedded via `//go:embed fonts/Exo2-Regular.ttf`
**Canvas size:** 820px wide, height dynamic based on row count

**Design (from approved HTML preview):**
- Background: gradient `#0D1117` → `#1A2332` → `#0D1117`
- Border: `#C8AA6E` (Dota gold), 1px + top glow strip
- Header: gold text, week label
- Rows: alternating subtle shade, player avatar circles 36px loaded from Steam CDN
- Colors: wins `#27AE60`, losses `#C0392B`, win% green/yellow/red by threshold, MMR+ `#3498DB`, MMR- `#E74C3C`
- Footer: timestamp + "Stratz API"

**Upload:** PNG bytes → MinIO bucket `dota-rankings` → public key `ranking-{type}-{YYYY}-W{WW}.png`
**Discord:** bot edits the 3 pinned messages in `RANKING_CHANNEL_ID` with `embed.Image.URL = minioPublicURL`

---

## Commands Added

```
/dota ranking                          → link to #dota-rankings channel
/dota ranking mes:<enero|febrero|...>  → on-demand image for that month (reply in current channel)
/dota ranking ultimas:<10|100>         → last N matches image (reply)
/dota admin register account_id:<id> nombre:<name>
                                       → register ranking-only user (no Discord ID)
```

---

## Ranking Channel Behavior

- On first startup with `RANKING_CHANNEL_ID` set: bot sends 3 messages (individual, team2, team3) and saves their IDs in `ranking_messages` table.
- After every `sendMatchNotification`: `ranking.Updater.Refresh()` recalculates, regenerates PNGs, uploads to MinIO, edits the 3 Discord messages.
- Commands `/dota ranking mes:X` and `/dota ranking ultimas:N` generate a temporary image and send as a new reply (do not edit pinned messages).

---

## Migration from JSON

On startup `db.RunMigrations()` creates tables. Separately `db.MigrateFromJSON()` runs once:
- Reads `data/users.json` → inserts into `users`
- Reads `data/last_matches.json` → inserts into `last_processed_match`
- Reads `data/notification_channel.json` → inserts key `notification_channel` into `config`
- Marks migration done in `config` table (key `json_migration_done = true`) to skip on next startup

---

## Web Foundation (architecture note)

- `internal/db/queries.go` exposes typed functions, no SQL in `discord/` or `ranking/`
- `internal/ranking/calc.go` is pure data — no Discord dependency
- Future: add `api/` package with HTTP handlers that call the same `internal/` packages

---

## Rollback

Previous version tagged: `orgmcr.or-gm.com/osmargm1202/dota-discord-bot:v1`
