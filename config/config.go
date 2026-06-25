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
