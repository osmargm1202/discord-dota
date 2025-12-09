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

	// Crear cliente de Dota API
	dotaClient := dota.NewClient()

	// Cargar constants al inicio (se cargarán automáticamente cuando se necesiten)
	logrus.Info("Bot listo. Los constants se cargarán automáticamente cuando se necesiten.")

	// Crear bot
	bot, err := discord.NewBot(cfg, dotaClient, userStore)
	if err != nil {
		logrus.Fatalf("Error creando bot: %v", err)
	}

	// Iniciar bot
	if err := bot.Start(); err != nil {
		logrus.Fatalf("Error iniciando bot: %v", err)
	}

	logrus.Info("Bot corriendo. Presiona CTRL+C para salir.")

	// Enviar mensaje de bienvenida al canal configurado
	go func() {
		time.Sleep(2 * time.Second) // Esperar un poco para asegurar que el bot esté completamente conectado
		if err := bot.SendWelcomeMessage(); err != nil {
			logrus.Warnf("No se pudo enviar mensaje de bienvenida: %v", err)
		}
	}()

	// Configurar polling cada 10 minutos
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	// Ejecutar verificación inicial después de 30 segundos
	go func() {
		time.Sleep(30 * time.Second)
		logrus.Info("Ejecutando verificación inicial de partidas...")
		if err := bot.CheckForNewMatches(); err != nil {
			logrus.Errorf("Error en verificación inicial: %v", err)
		}
	}()

	// Loop de polling
	go func() {
		for range ticker.C {
			logrus.Debug("Ejecutando verificación periódica de partidas...")
			if err := bot.CheckForNewMatches(); err != nil {
				logrus.Errorf("Error verificando partidas: %v", err)
			}
		}
	}()

	// Esperar señal de interrupción
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	logrus.Info("Cerrando bot...")
	bot.Stop()
	logrus.Info("Bot cerrado exitosamente")
}

