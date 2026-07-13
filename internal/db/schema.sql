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
    match_id      BIGINT NOT NULL,
    dota_id       BIGINT NOT NULL,
    discord_id    VARCHAR(20) NOT NULL,
    enqueued_at   TIMESTAMPTZ DEFAULT NOW(),
    last_attempt  TIMESTAMPTZ,
    attempt_count INT DEFAULT 0,
    status        VARCHAR(20) DEFAULT 'pending',
    PRIMARY KEY (match_id, dota_id)
);

-- Migrate the legacy match-only queue. Those rows lack recipient identity and
-- cannot be processed safely, so recent-match discovery will repopulate them.
ALTER TABLE parse_queue ADD COLUMN IF NOT EXISTS dota_id BIGINT;
ALTER TABLE parse_queue ADD COLUMN IF NOT EXISTS discord_id VARCHAR(20);
DELETE FROM parse_queue WHERE dota_id IS NULL OR discord_id IS NULL;
ALTER TABLE parse_queue ALTER COLUMN dota_id SET NOT NULL;
ALTER TABLE parse_queue ALTER COLUMN discord_id SET NOT NULL;
ALTER TABLE parse_queue DROP CONSTRAINT IF EXISTS parse_queue_match_id_fkey;
DO $$
DECLARE
    current_pk TEXT;
BEGIN
    SELECT pg_get_constraintdef(oid)
      INTO current_pk
      FROM pg_constraint
     WHERE conrelid = 'parse_queue'::regclass AND contype = 'p';
    IF current_pk IS DISTINCT FROM 'PRIMARY KEY (match_id, dota_id)' THEN
        ALTER TABLE parse_queue DROP CONSTRAINT IF EXISTS parse_queue_pkey;
        ALTER TABLE parse_queue ADD CONSTRAINT parse_queue_pkey PRIMARY KEY (match_id, dota_id);
    END IF;
END $$;
CREATE INDEX IF NOT EXISTS idx_parse_queue_pending_order
    ON parse_queue (match_id DESC) WHERE status = 'pending';

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

-- Lane outcome columns (added after initial schema; ADD COLUMN IF NOT EXISTS is idempotent)
ALTER TABLE matches ADD COLUMN IF NOT EXISTS top_lane_outcome    VARCHAR(20);
ALTER TABLE matches ADD COLUMN IF NOT EXISTS mid_lane_outcome    VARCHAR(20);
ALTER TABLE matches ADD COLUMN IF NOT EXISTS bottom_lane_outcome VARCHAR(20);

-- All-time lane phase record per player (recomputed on demand)
CREATE TABLE IF NOT EXISTS lane_records (
    dota_id    BIGINT PRIMARY KEY,
    wins       INT NOT NULL DEFAULT 0,
    draws      INT NOT NULL DEFAULT 0,
    losses     INT NOT NULL DEFAULT 0,
    unknown    INT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
