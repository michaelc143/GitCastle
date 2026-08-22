package main

import (
	"context"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/michaelc143/gitcastle/internal/auth"
	"github.com/michaelc143/gitcastle/internal/automation"
	"github.com/michaelc143/gitcastle/internal/ci"
	"github.com/michaelc143/gitcastle/internal/collab"
	"github.com/michaelc143/gitcastle/internal/secrets"
	"github.com/michaelc143/gitcastle/internal/webhooks"
	"github.com/michaelc143/gitcastle/internal/config"
	"github.com/michaelc143/gitcastle/internal/database"
	"github.com/michaelc143/gitcastle/internal/gitserve"
	"github.com/michaelc143/gitcastle/internal/mergesvc"
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
	permissions := &auth.Permissions{Pool: pool}
	collabStore := &collab.Store{Pool: pool}

	webhookStore := &webhooks.Store{Pool: pool}
	webhookDispatcher := &webhooks.Dispatcher{Store: webhookStore, Logger: logger}
	ciStore := &ci.Store{Pool: pool}
	ciRunner := &ci.Runner{
		Image:    os.Getenv("CI_RUNNER_IMAGE"),
		WorkRoot: cfg.RepositoryRoot + "/.ci-work",
	}
	ciExecutor := &ci.Executor{
		Store:  ciStore,
		Runner: ciRunner,
		Logger: logger,
		RepoPath: func(ctx context.Context, repositoryID int64) (string, error) {
			var path string
			err := pool.QueryRow(ctx, `SELECT path FROM repositories WHERE id = $1`, repositoryID).Scan(&path)
			return path, err
		},
	}
	internalToken := os.Getenv("GITCASTLE_INTERNAL_TOKEN")
	if internalToken == "" {
		logger.Warn("push notifications disabled", "reason", "GITCASTLE_INTERNAL_TOKEN not set")
	}
	automationConfig := &automation.Config{
		WebhookStore: webhookStore,
		Dispatcher:   webhookDispatcher,
		CIStore:      ciStore,
		Executor:     ciExecutor,
		Logger:       logger,
		RepoPath:     ciExecutor.RepoPath,
	}
	var secretKey []byte
	if envKey := os.Getenv("SECRET_ENCRYPTION_KEY"); len(envKey) == 64 {
		decoded, err := hex.DecodeString(envKey)
		if err == nil && len(decoded) == 32 {
			secretKey = decoded
		}
	}
	secretStore, secretErr := secrets.NewStore(pool, secretKey)
	if secretErr != nil {
		logger.Warn("secrets disabled", "reason", secretErr.Error())
		secretStore = nil
	}
	repositoryService := repos.Service{
		Store:          repos.PostgresStore{Pool: pool},
		RepositoryRoot: cfg.RepositoryRoot,
		Git:            repos.CommandGitInitializer{},
	}
	mergeService := &mergesvc.Service{
		Root:   cfg.RepositoryRoot,
		Events: nil, // events flow through automationConfig on merge completion
	}
	gitBackend := &gitserve.Handler{Root: cfg.RepositoryRoot, Prefix: "/git"}
	gitHandler := gitserve.AuthHandler{
		Backend: gitBackend,
		Auth: gitserve.BridgeAuthorizer{
			AuthenticateFunc: func(ctx context.Context, username, password string) (int64, error) {
				user, err := authStore.Authenticate(ctx, username, password)
				if err != nil {
					return 0, err
				}
				return user.ID, nil
			},
			CheckAccessFunc: func(ctx context.Context, userID int64, owner, repo string, access gitserve.Access) error {
				repository, err := repositoryService.Get(ctx, owner, repo)
				if err != nil {
					return err
				}
				user, err := authStore.UserForID(ctx, userID)
				if err != nil {
					return err
				}
				role, err := permissions.RoleFor(ctx, repository.ID, user.Username)
				if err != nil {
					return err
				}
				required := auth.RoleRead
				if access == gitserve.AccessWrite {
					required = auth.RoleWrite
				}
				if !auth.HasAtLeast(role, required) {
					return gitserve.ErrAccessDenied
				}
				return nil
			},
		},
	}

	mux := http.NewServeMux()
	mux.Handle("/git/", gitHandler)
	httpOptions := httpapi.Options{
		Collab:        collabStore,
		Merger:        mergeService,
		Automation:    automationAdapter{config: automationConfig},
		Pushes:        automationAdapter{config: automationConfig},
		InternalToken: internalToken,
		Webhooks:      webhookStore,
		Jobs:          ciStore,
	}
	if secretStore != nil {
		httpOptions.Secrets = secretStore
	}
	mux.Handle("/", httpapi.NewHandler(repositoryService, authStore, permissions, logger, httpOptions))
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


// automationAdapter converts httpapi merge events into automation events.
type automationAdapter struct{ config *automation.Config }

func (a automationAdapter) PushReceived(ctx context.Context, event httpapi.PushEvent) {
	a.config.PushReceived(ctx, automation.PushEvent{
		RepositoryID: event.RepositoryID,
		Owner:        event.Owner,
		Name:         event.Name,
		Branch:       event.Branch,
		OldHash:      event.OldHash,
		NewHash:      event.NewHash,
	})
}

func (a automationAdapter) PullRequestMerged(event httpapi.MergeEvent) {
	a.config.PullRequestMerged(automation.MergeEvent{
		RepositoryID: event.RepositoryID,
		Owner:        event.Owner,
		Name:         event.Name,
		Number:       event.Number,
		Actor:        event.Actor,
		MergeCommit:  event.MergeCommit,
		SourceBranch: event.SourceBranch,
		TargetBranch: event.TargetBranch,
	})
}
