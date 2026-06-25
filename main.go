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
	debug := flag.Bool("debug", false, "Activar modo debug (logs en consola)")
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

	// Legacy JSON store (used for migration and fallback)
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

		// Migrate from JSON files (idempotent)
		jsonUsers := userStore.GetAll()
		jsonLastMatches := loadJSONLastMatches()
		jsonChannel, _ := userStore.GetChannel()
		if err := database.MigrateFromJSON(jsonUsers, jsonLastMatches, jsonChannel); err != nil {
			logrus.Warnf("JSON migration warning: %v", err)
		} else {
			logrus.Info("JSON migration complete (or already done)")
		}
	} else {
		logrus.Warn("POSTGRES_DSN no configurado — usando solo JSON storage")
	}

	// MinIO
	var minioClient *minioclient.Client
	if cfg.MinioEndpoint != "" && database != nil {
		minioClient, err = minioclient.New(
			cfg.MinioEndpoint,
			cfg.MinioAccessKey,
			cfg.MinioSecretKey,
			cfg.MinioBucket,
			cfg.MinioPublicURL,
		)
		if err != nil {
			logrus.Warnf("MinIO no disponible: %v", err)
		} else {
			logrus.Info("MinIO conectado")
		}
	}

	// Dota / Stratz clients
	dotaClient := dota.NewClient()
	if cfg.StratzToken == "" {
		logrus.Fatal("STRATZ_TOKEN es obligatorio. El bot usa solo la API de Stratz.")
	}
	stratzClient := dota.NewStratzClient(cfg.StratzToken)
	if cfg.Debug {
		stratzClient.SetDebug(true)
		logrus.Info("Debug Stratz activado")
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

	// Init ranking channel + updater
	if database != nil && minioClient != nil && cfg.RankingChannelID != "" {
		rankingUpdater := ranking.NewUpdater(
			database,
			minioClient,
			bot.Session(),
			cfg.RankingChannelID,
			cfg.BaseYear,
		)
		bot.SetRankingUpdater(rankingUpdater)
		go func() {
			time.Sleep(5 * time.Second)
			if err := rankingUpdater.InitChannel(); err != nil {
				logrus.Errorf("ranking init: %v", err)
			}
		}()
	}

	// Welcome + immediate match check
	go func() {
		time.Sleep(2 * time.Second)
		if err := bot.SendWelcomeMessage(); err != nil {
			logrus.Warnf("No se pudo enviar mensaje de bienvenida: %v", err)
		}
		logrus.Info("Ejecutando verificación inmediata de partidas...")
		if err := bot.CheckForNewMatches(); err != nil {
			logrus.Errorf("Error en verificación inicial: %v", err)
		}
	}()

	// Historical backfill (runs in background, rate-limited, idempotent)
	if database != nil {
		bfSvc := backfill.New(database, stratzClient, cfg.BaseYear, cfg.BackfillDelayMS)
		bot.SetBackfillService(bfSvc)
		go func() {
			time.Sleep(15 * time.Second) // let bot fully settle first
			bfSvc.Run()
		}()
	}

	// Polling ticker
	ticker := time.NewTicker(time.Duration(cfg.RefreshRateMinutes) * time.Minute)
	defer ticker.Stop()
	logrus.Infof("Verificación de partidas cada %d minuto(s)", cfg.RefreshRateMinutes)
	go func() {
		for range ticker.C {
			logrus.Debug("Ejecutando verificación periódica de partidas...")
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
