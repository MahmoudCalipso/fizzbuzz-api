// Command server starts the fizz-buzz REST API.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"fizzbuzz-api/internal/config"
	"fizzbuzz-api/internal/controller"
	"fizzbuzz-api/internal/repository"
	"fizzbuzz-api/internal/service"
)

// @title			FizzBuzz API
// @version		1.0
// @description	REST API generating customizable fizz-buzz lists and reporting the most frequent request.
// @BasePath		/api/v1
func main() {
	if err := run(); err != nil {
		slog.Error("server stopped with error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	if os.Getenv(gin.EnvGinMode) == "" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Composition root: the only place where concrete types are wired together.
	repo := repository.NewMemoryStats(cfg.MaxStatsEntries)
	svc := service.New(repo, cfg.MaxLimit)
	router := controller.NewRouter(svc, logger)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serverErr := make(chan error, 1)
	go func() {
		baseURL := "http://localhost:" + cfg.Port
		logger.Info("server listening",
			"addr", srv.Addr,
			"api_url", baseURL+"/api/v1",
			"swagger_url", baseURL+"/swagger/index.html",
			"max_limit", cfg.MaxLimit,
		)
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	select {
	case err := <-serverErr:
		return err
	case <-ctx.Done():
		logger.Info("shutdown signal received, draining connections")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	logger.Info("server stopped cleanly")
	return nil
}
