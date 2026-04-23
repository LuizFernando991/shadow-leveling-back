package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/LuizFernando991/gym-api/internal/config"
	"github.com/LuizFernando991/gym-api/internal/database"
	"github.com/LuizFernando991/gym-api/internal/features/auth"
	"github.com/LuizFernando991/gym-api/internal/features/task"
	"github.com/LuizFernando991/gym-api/internal/features/workout"
	"github.com/LuizFernando991/gym-api/internal/infra/cache"
	"github.com/LuizFernando991/gym-api/internal/infra/email"
	"github.com/LuizFernando991/gym-api/internal/infra/http/router"
	"github.com/LuizFernando991/gym-api/internal/infra/http/server"
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

	redisClient := cache.NewRedisClient(cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB)
	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		slog.Error("redis connection failed", "error", err)
		os.Exit(1)
	}
	defer redisClient.Close()
	rateLimiter := cache.NewRedisRateLimiter(redisClient)

	var emailSender email.Sender
	if cfg.App.Env == "production" {
		emailSender = email.NewResendSender(cfg.Email.ResendAPIKey, cfg.Email.FromAddress)
	} else {
		emailSender = email.NewDevSender()
	}

	modules := router.Modules{
		Auth:    auth.NewModule(db, cfg.Auth, emailSender, rateLimiter),
		Task:    task.NewModule(db),
		Workout: workout.NewModule(db),
	}

	httpRouter := router.NewRouter(cfg, modules)

	httpServer := server.NewHttpServer(cfg, httpRouter)

	if err := httpServer.Start(); err != nil {
		slog.Error("application error", "error", err)
		os.Exit(1)
	}
}
