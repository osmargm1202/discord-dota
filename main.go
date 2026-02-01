package main

import (
	"dota-discord-bot/config"
	"dota-discord-bot/discord"
	"dota-discord-bot/dota"
	"dota-discord-bot/storage"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
)

func main() {
	// Parsear flag --debug
	debug := flag.Bool("debug", false, "Activar modo debug (logs en consola)")
	flag.Parse()

	// Cargar configuración
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error cargando configuración: %v\n", err)
		os.Exit(1)
	}

	// Si se pasó --debug, sobrescribir configuración
	if *debug {
		cfg.Debug = true
	}

	// Configurar logger básico para main
	if cfg.Debug {
		logrus.SetLevel(logrus.DebugLevel)
		logrus.SetOutput(os.Stdout)
	} else {
		logrus.SetLevel(logrus.InfoLevel)
		logrus.SetOutput(os.Stdout)
	}
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
	})

	logrus.Info("Iniciando bot de Discord para Dota 2...")

	// Crear almacenamiento
	userStore, err := storage.NewUserStore()
	if err != nil {
		logrus.Fatalf("Error creando almacenamiento: %v", err)
	}

	// Cliente OpenDota: código conservado pero no se usa (solo Stratz)
	dotaClient := dota.NewClient()

	// Stratz es obligatorio: el bot usa solo la API de Stratz
	if cfg.StratzToken == "" {
		logrus.Fatal("STRATZ_TOKEN es obligatorio. El bot usa solo la API de Stratz. Configura STRATZ_TOKEN en .env")
	}
	stratzClient := dota.NewStratzClient(cfg.StratzToken)
	if cfg.Debug {
		stratzClient.SetDebug(true)
		logrus.Info("Debug Stratz activado: request/response en logs/stratz_debug.log")
	}
	logrus.Info("Cliente de Stratz configurado (solo Stratz)")

	// Crear bot
	bot, err := discord.NewBot(cfg, dotaClient, stratzClient, userStore)
	if err != nil {
		logrus.Fatalf("Error creando bot: %v", err)
	}

	// Iniciar bot
	if err := bot.Start(); err != nil {
		logrus.Fatalf("Error iniciando bot: %v", err)
	}

	logrus.Info("Bot corriendo. Presiona CTRL+C para salir.")

	// Enviar mensaje de bienvenida y verificar partidas inmediatamente
	go func() {
		time.Sleep(2 * time.Second) // Esperar un poco para asegurar que el bot esté completamente conectado
		if err := bot.SendWelcomeMessage(); err != nil {
			logrus.Warnf("No se pudo enviar mensaje de bienvenida: %v", err)
		}
		// Verificación INMEDIATA de partidas al iniciar
		logrus.Info("Ejecutando verificación inmediata de partidas...")
		if err := bot.CheckForNewMatches(); err != nil {
			logrus.Errorf("Error en verificación inicial: %v", err)
		}
	}()

	// Configurar polling cada REFRESH_RATE minutos (por defecto 1)
	ticker := time.NewTicker(time.Duration(cfg.RefreshRateMinutes) * time.Minute)
	defer ticker.Stop()
	logrus.Infof("Verificación de partidas cada %d minuto(s)", cfg.RefreshRateMinutes)

	// Loop de polling
	go func() {
		for range ticker.C {
			logrus.Debug("Ejecutando verificación periódica de partidas...")
			if err := bot.CheckForNewMatches(); err != nil {
				logrus.Errorf("Error verificando partidas: %v", err)
			}
		}
	}()

	// Scheduler diario de stats (STATS_TIME en .env, ej. 20:00)
	go bot.RunStatsScheduler()

	// Esperar señal de interrupción
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	logrus.Info("Cerrando bot...")
	bot.Stop()
	logrus.Info("Bot cerrado exitosamente")
}
