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
	ServerID              string
	StratzToken           string
	Debug                 bool
	RefreshRateMinutes    int    // intervalo en minutos para verificar nuevas partidas (>= 1, <= 60)
	RequireParsed         bool   // si true, solo notificar cuando la partida esté parseada (parsedDateTime > 0); si false, notificar cualquier partida nueva
	StatsMinGames         int    // mínimo de partidas por héroe para /dota stats (>= 2)
	StatsTime             string // hora militar (HH:MM) para envío diario de stats; vacío = desactivado
	StatsTake             int    // partidas analizadas para stats (0-100; 0 = 100)
}

func Load() (*Config, error) {
	// Cargar .env si existe
	if err := godotenv.Load(); err != nil {
		// No es crítico si no existe el archivo
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
		return nil, fmt.Errorf("STRATZ_TOKEN no está configurado en .env")
	}

	debug := os.Getenv("DEBUG") == "true"

	requireParsed := os.Getenv("PARSED") != "false" // true por defecto; solo "false" desactiva la verificación

	refreshRateMinutes := 1
	if s := os.Getenv("REFRESH_RATE"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n >= 1 {
			refreshRateMinutes = n
			if refreshRateMinutes > 60 {
				refreshRateMinutes = 60
			}
		}
	}

	statsMinGames := 2
	if s := os.Getenv("STATS_MIN_GAMES"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n >= 2 {
			statsMinGames = n
		}
	}

	statsTime := os.Getenv("STATS_TIME") // HH:MM, ej. "20:00"; vacío = no envío automático

	statsTake := 100
	if s := os.Getenv("STATS_TAKE"); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			if n <= 0 {
				statsTake = 100
			} else if n > 100 {
				statsTake = 100
			} else {
				statsTake = n
			}
		}
	}

	return &Config{
		DiscordToken:          discordToken,
		NotificationChannelID: notificationChannelID,
		ServerID:              serverID,
		StratzToken:           stratzToken,
		Debug:                 debug,
		RefreshRateMinutes:    refreshRateMinutes,
		RequireParsed:         requireParsed,
		StatsMinGames:         statsMinGames,
		StatsTime:             statsTime,
		StatsTake:             statsTake,
	}, nil
}
