// Command server はLESSのREST API serverを起動する。
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/app"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/infrastructure/config"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/platform/logging"
)

// graceful shutdownの待ち時間。
const shutdownTimeout = 15 * time.Second

// HTTP serverのtimeout設定。slowloris攻撃への基本的な対策となる。
const (
	readHeaderTimeout = 10 * time.Second
	readTimeout       = 30 * time.Second
	writeTimeout      = 60 * time.Second
	idleTimeout       = 120 * time.Second
)

func main() {
	if err := run(); err != nil {
		slog.Error("server terminated", "error", err.Error())
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	logger := logging.New(os.Stdout, cfg.LogLevel)
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	application, err := app.New(ctx, cfg, logger)
	if err != nil {
		return err
	}
	defer application.Close()

	server := &http.Server{
		Addr:              cfg.ListenAddress(),
		Handler:           application.Handler,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}

	serverErrors := make(chan error, 1)
	go func() {
		logger.Info(
			"server started",
			"address", cfg.ListenAddress(),
			"appEnv", cfg.AppEnv,
		)
		if listenErr := server.ListenAndServe(); listenErr != nil &&
			!errors.Is(listenErr, http.ErrServerClosed) {
			serverErrors <- listenErr
		}
		close(serverErrors)
	}()

	select {
	case listenErr := <-serverErrors:
		if listenErr != nil {
			return listenErr
		}
		return nil
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	}

	shutdownContext, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := server.Shutdown(shutdownContext); err != nil {
		return err
	}

	logger.Info("server stopped")
	return nil
}
