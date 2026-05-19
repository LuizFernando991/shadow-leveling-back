package main

import (
	"log/slog"
	"os"

	"github.com/LuizFernando991/gym-api/internal/config"
	"github.com/LuizFernando991/gym-api/internal/database"
	"github.com/LuizFernando991/gym-api/internal/features/auth"
	"github.com/LuizFernando991/gym-api/internal/features/leveling"
	"github.com/LuizFernando991/gym-api/internal/features/task"
	"github.com/LuizFernando991/gym-api/internal/features/usermetrics"
	"github.com/LuizFernando991/gym-api/internal/features/workout"
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

	levelingModule := leveling.NewModule(db)

	modules := router.Modules{
		Auth:        auth.NewModule(db, cfg.Auth),
		Task:        task.NewModule(db),
		UserMetrics: usermetrics.NewModule(db),
		Workout:     workout.NewModule(db, levelingModule.Awarder()),
		Leveling:    levelingModule,
	}

	httpRouter := router.NewRouter(cfg, modules)

	httpServer := server.NewHttpServer(cfg, httpRouter)

	if err := httpServer.Start(); err != nil {
		slog.Error("application error", "error", err)
		os.Exit(1)
	}
}
