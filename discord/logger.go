package discord

import (
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
)

var log *logrus.Logger

func InitLogger(debug bool) error {
	log = logrus.New()

	// Crear directorio logs/ si no existe
	if err := os.MkdirAll("logs", 0755); err != nil {
		return err
	}

	// Configurar archivo de log
	logFile, err := os.OpenFile(filepath.Join("logs", "bot.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return err
	}

	// Escribir a archivo y consola si debug está activado
	if debug {
		log.SetOutput(os.Stdout)
		log.SetLevel(logrus.DebugLevel)
	} else {
		log.SetOutput(logFile)
		log.SetLevel(logrus.InfoLevel)
	}

	// Formato con timestamps
	log.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
		TimestampFormat: "2006-01-02 15:04:05",
	})

	return nil
}

func getLogger() *logrus.Logger {
	if log == nil {
		// Fallback si no se inicializó
		log = logrus.New()
		log.SetOutput(os.Stdout)
	}
	return log
}

