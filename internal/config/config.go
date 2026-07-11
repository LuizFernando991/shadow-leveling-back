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

type EmailConfig struct {
	ResendAPIKey string
	FromAddress  string
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

type PushConfig struct {
	ExpoAccessToken string
}

// StorageConfig configures image uploads (Firebase Storage / GCS bucket).
// When Bucket is empty the app falls back to a noop uploader (local dev/tests).
// Credentials come from the individual service-account fields (preferred for
// prod, passed as secrets) or, as a dev convenience, SAJSONPath (a file on disk).
type StorageConfig struct {
	Bucket      string
	ProjectID   string
	ClientEmail string
	PrivateKey  string
	SAJSONPath  string
}

type Config struct {
	App     AppConfig
	DB      DBConfig
	Auth    AuthConfig
	Email   EmailConfig
	Storage StorageConfig
	Redis   RedisConfig
	Push    PushConfig
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
		Email: EmailConfig{
			ResendAPIKey: os.Getenv("RESEND_API_KEY"),
			FromAddress:  os.Getenv("EMAIL_FROM"),
		},
		Storage: StorageConfig{
			Bucket:      os.Getenv("STORAGE_BUCKET"),
			ProjectID:   os.Getenv("STORAGE_PROJECT_ID"),
			ClientEmail: os.Getenv("STORAGE_CLIENT_EMAIL"),
			PrivateKey:  os.Getenv("STORAGE_PRIVATE_KEY"),
			SAJSONPath:  os.Getenv("STORAGE_SA_JSON_PATH"),
		},
		Redis: RedisConfig{
			Addr:     os.Getenv("REDIS_ADDR"),
			Password: os.Getenv("REDIS_PASSWORD"),
			DB:       0,
		},
		Push: PushConfig{
			ExpoAccessToken: os.Getenv("EXPO_ACCESS_TOKEN"),
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
