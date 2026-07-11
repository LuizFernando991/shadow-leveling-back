package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/LuizFernando991/gym-api/internal/config"
	"github.com/LuizFernando991/gym-api/internal/database"
	"github.com/LuizFernando991/gym-api/internal/features/auth"
	"github.com/LuizFernando991/gym-api/internal/features/group"
	"github.com/LuizFernando991/gym-api/internal/features/leveling"
	"github.com/LuizFernando991/gym-api/internal/features/task"
	"github.com/LuizFernando991/gym-api/internal/features/usermetrics"
	"github.com/LuizFernando991/gym-api/internal/features/workout"
	"github.com/LuizFernando991/gym-api/internal/infra/cache"
	"github.com/LuizFernando991/gym-api/internal/infra/http/router"
	"github.com/LuizFernando991/gym-api/internal/infra/http/server"
	"github.com/LuizFernando991/gym-api/internal/infra/storage"
	"github.com/LuizFernando991/gym-api/internal/shared/httputil"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	cfg := config.Load()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	slog.SetDefault(logger)

	db, err := database.Connect(cfg.DB)
	if err != nil {
		slog.Error("database connection failed", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	uploader := buildUploader(cfg.Storage)
	rateLimiter := buildRateLimiter(cfg.Redis)

	levelingModule := leveling.NewModule(db)

	modules := router.Modules{
		Auth:        auth.NewModule(db, cfg.Auth),
		Task:        task.NewModule(db),
		UserMetrics: usermetrics.NewModule(db),
		Workout:     workout.NewModule(db, levelingModule.Awarder(), uploader, rateLimiter),
		Leveling:    levelingModule,
		Group:       group.NewModule(db, uploader, rateLimiter),
	}

	httpRouter := router.NewRouter(cfg, modules)

	httpServer := server.NewHttpServer(cfg, httpRouter)

	if err := httpServer.Start(); err != nil {
		slog.Error("application error", "error", err)
		os.Exit(1)
	}
}

// buildUploader returns the Firebase/GCS uploader when a bucket is configured,
// otherwise a noop uploader for local development.
func buildUploader(cfg config.StorageConfig) storage.Uploader {
	if cfg.Bucket == "" {
		slog.Warn("storage: no STORAGE_BUCKET configured, image uploads are noop")
		return storage.NewNoopUploader()
	}

	// Prefer individual fields (prod secrets); fall back to a JSON file (dev).
	var (
		up  storage.Uploader
		err error
	)
	switch {
	case cfg.PrivateKey != "" && cfg.ClientEmail != "":
		up, err = storage.NewGCSUploaderFromFields(
			context.Background(), cfg.Bucket, cfg.ProjectID, cfg.ClientEmail, cfg.PrivateKey,
		)
	case cfg.SAJSONPath != "":
		var b []byte
		b, err = os.ReadFile(cfg.SAJSONPath)
		if err == nil {
			up, err = storage.NewGCSUploader(context.Background(), cfg.Bucket, b)
		}
	default:
		slog.Error("storage: STORAGE_BUCKET set but no credentials (set STORAGE_PRIVATE_KEY + STORAGE_CLIENT_EMAIL, or STORAGE_SA_JSON_PATH)")
		os.Exit(1)
	}
	if err != nil {
		slog.Error("storage: init gcs uploader failed", "error", err)
		os.Exit(1)
	}
	return up
}

// buildRateLimiter returns a Redis-backed limiter when REDIS_ADDR is set and
// reachable, otherwise a noop limiter (rate limiting disabled in local dev).
func buildRateLimiter(cfg config.RedisConfig) httputil.RateAllower {
	if cfg.Addr == "" {
		slog.Warn("rate limit: no REDIS_ADDR configured, uploads are not rate-limited")
		return cache.NoopRateLimiter{}
	}
	client := cache.NewRedisClient(cfg.Addr, cfg.Password, cfg.DB)
	if err := client.Ping(context.Background()).Err(); err != nil {
		slog.Error("rate limit: redis unreachable, falling back to noop", "error", err)
		return cache.NoopRateLimiter{}
	}
	return cache.NewRedisRateLimiter(client)
}
