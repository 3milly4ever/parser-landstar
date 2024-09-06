package config

import (
	"os"

	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
)

type Config struct {
	ServerIP           string
	ServerPort         string
	LogFile            string
	AWSRegion          string
	AWSAccessKeyID     string
	AWSSecretAccessKey string
	SQSQueueURL        string
	MySQLDSN           string
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
		ServerIP:           getEnv("SERVER_IP", "127.0.0.1"),
		ServerPort:         getEnv("SERVER_PORT", "54321"),
		LogFile:            getEnv("LOG_FILE", "logs/app.log"),
		AWSRegion:          getEnv("AWS_REGION", "us-east-1"),
		AWSAccessKeyID:     getEnv("AWS_ACCESS_KEY_ID", ""),
		AWSSecretAccessKey: getEnv("AWS_SECRET_ACCESS_KEY", ""),
		SQSQueueURL:        getEnv("SQS_QUEUE_URL", ""),
		MySQLDSN:           getEnv("MYSQL_DSN", ""),
	}
}

// Helper function to read an environment variable or return a default value
func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}
