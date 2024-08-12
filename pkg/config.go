package config

import (
	"os"

	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
)

type Config struct {
	ServerIP   string
	ServerPort string
	LogFile    string
}

var AppConfig Config

func LoadConfig() {
	// Load .env file if present
	err := godotenv.Load("/home/emil/parser-landstar/.env")
	if err != nil {
		logrus.Warn("No .env file found")
	} else {
		logrus.Info(".env file loaded successfully")
	}

	AppConfig = Config{
		ServerIP:   getEnv("SERVER_IP", "127.0.0.1"),
		ServerPort: getEnv("SERVER_PORT", "54321"),
		LogFile:    getEnv("LOG_FILE", "logs/app.log"),
	}
}

// Helper function to read an environment variable or return a default value
func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}
