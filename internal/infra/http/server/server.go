package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/LuizFernando991/gym-api/internal/config"
)

const (
	readTimeout     = 5 * time.Second
	writeTimeout    = 10 * time.Second
	idleTimeout     = 120 * time.Second
	shutdownTimeout = 30 * time.Second
)

type HttpServer struct {
	http *http.Server
	cfg  *config.Config
}

func NewHttpServer(cfg *config.Config, handler http.Handler) *HttpServer {
	return &HttpServer{
		cfg: cfg,
		http: &http.Server{
			Addr:         fmt.Sprintf(":%s", cfg.App.HttpPort),
			Handler:      handler,
			ReadTimeout:  readTimeout,
			WriteTimeout: writeTimeout,
			IdleTimeout:  idleTimeout,
		},
	}
}

func (s *HttpServer) Start() error {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	errCh := make(chan error, 1)

	go func() {
		slog.Info("server started",
			"app", s.cfg.App.Name,
			"env", s.cfg.App.Env,
			"addr", s.http.Addr,
		)
		if err := s.http.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("server error: %w", err)
	case sig := <-quit:
		slog.Info("shutdown signal received", "signal", sig)
	}

	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	slog.Info("shutting down server gracefully...")
	if err := s.http.Shutdown(ctx); err != nil {
		return fmt.Errorf("server forced shutdown: %w", err)
	}

	slog.Info("server exited")
	return nil
}
