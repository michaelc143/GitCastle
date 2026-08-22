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

	"github.com/michaelc143/gitcastle/internal/auth"
	"github.com/michaelc143/gitcastle/internal/config"
	"github.com/michaelc143/gitcastle/internal/database"
	"github.com/michaelc143/gitcastle/internal/gitserve"
	"github.com/michaelc143/gitcastle/internal/httpapi"
	"github.com/michaelc143/gitcastle/internal/repos"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg := config.FromEnv()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("database unavailable", "error", err)
		os.Exit(1)
	}
	defer pool.Close()
	if err := database.Migrate(ctx, pool); err != nil {
		logger.Error("database migration failed", "error", err)
		os.Exit(1)
	}
	if err := os.MkdirAll(cfg.RepositoryRoot, 0o750); err != nil {
		logger.Error("repository storage unavailable", "error", err)
		os.Exit(1)
	}

	authStore := &auth.Store{Pool: pool}
	repositoryService := repos.Service{
		Store:          repos.PostgresStore{Pool: pool},
		RepositoryRoot: cfg.RepositoryRoot,
		Git:            repos.CommandGitInitializer{},
	}
	gitHandler := &gitserve.Handler{Root: cfg.RepositoryRoot, Prefix: "/git"}

	mux := http.NewServeMux()
	mux.Handle("/git/", gitHandler)
	mux.Handle("/", httpapi.NewHandler(repositoryService, authStore, logger))
	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("GitCastle listening", "addr", cfg.HTTPAddr, "repository_root", cfg.RepositoryRoot)
		serverErrors <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Error("HTTP server failed", "error", err)
			os.Exit(1)
		}
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Error("HTTP server shutdown failed", "error", err)
			os.Exit(1)
		}
	}
}
