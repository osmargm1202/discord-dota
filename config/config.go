package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DiscordToken         string
	NotificationChannelID string
	ServerID             string
	Debug                bool
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
	serverID := os.Getenv("SERVER_ID")

	debug := os.Getenv("DEBUG") == "true"

	return &Config{
		DiscordToken:          discordToken,
		NotificationChannelID: notificationChannelID,
		ServerID:              serverID,
		Debug:                 debug,
	}, nil
}

