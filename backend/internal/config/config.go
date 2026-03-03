package config

import (
	"os"
)

type Config struct {
	Port         string
	DBUser       string
	DBPass       string
	DBHost       string
	DBName       string
	KafkaBrokers string
}

func LoadConfig() *Config {
	return &Config{
		Port:         getEnv("PORT", "8080"),
		DBUser:       getEnv("DB_USER", "postgres"),
		DBPass:       getEnv("DB_PASS", "postgres"),
		DBHost:       getEnv("DB_HOST", "postgres"),
		DBName:       getEnv("DB_NAME", "minion_bank"),
		KafkaBrokers: getEnv("KAFKA_BROKERS", "kafka:9092"),
	}
}

func getEnv(key, defaultVal string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultVal
}
