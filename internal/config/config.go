package config

import (
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type AppConfig struct {
	Env            string
	Name           string
	HttpPort       string
	MetricsEnabled bool
}

type DBConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Name     string
	SSLMode  string
}

type AuthConfig struct {
	JWTSecret string
	TokenTTL  time.Duration
}

type Config struct {
	App  AppConfig
	DB   DBConfig
	Auth AuthConfig
}

func Load() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("no .env file found, reading from environment")
	}

	return &Config{
		App: AppConfig{
			Env:            mustGetEnv("APP_ENV"),
			Name:           mustGetEnv("APP_NAME"),
			HttpPort:       mustGetEnv("APP_PORT"),
			MetricsEnabled: os.Getenv("METRICS_ENABLED") == "true",
		},
		DB: DBConfig{
			Host:     mustGetEnv("DB_HOST"),
			Port:     mustGetEnvAsInt("DB_PORT"),
			User:     mustGetEnv("DB_USER"),
			Password: mustGetEnv("DB_PASSWORD"),
			Name:     mustGetEnv("DB_NAME"),
			SSLMode:  mustGetEnv("DB_SSLMODE"),
		},
		Auth: AuthConfig{
			JWTSecret: mustGetEnv("AUTH_JWT_SECRET"),
			TokenTTL:  mustGetEnvAsDuration("AUTH_TOKEN_TTL"),
		},
	}
}

func mustGetEnv(key string) string {
	val, ok := os.LookupEnv(key)
	if !ok {
		log.Fatalf("missing required env: %s", key)
	}
	return val
}

func mustGetEnvAsInt(key string) int {
	valStr := mustGetEnv(key)

	val, err := strconv.Atoi(valStr)
	if err != nil {
		log.Fatalf("invalid int for env %s: %v", key, err)
	}

	return val
}

func mustGetEnvAsDuration(key string) time.Duration {
	valStr := mustGetEnv(key)

	val, err := time.ParseDuration(valStr)
	if err != nil {
		log.Fatalf("invalid duration for env %s: %v", key, err)
	}

	return val
}
