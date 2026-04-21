package main

import (
	"log/slog"
	"os"

	"github.com/LuizFernando991/gym-api/internal/config"
	"github.com/LuizFernando991/gym-api/internal/infra/http/router"
	"github.com/LuizFernando991/gym-api/internal/infra/http/server"
)

func main() {
	config := config.Load()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	slog.SetDefault(logger)

	handlers := router.Handlers{}

	httpRouter := router.NewRouter(config, handlers)

	httpServer := server.NewHttpServer(config, httpRouter)

	if err := httpServer.Start(); err != nil {
		slog.Error("application error", "error", err)
		os.Exit(1)
	}
}
