package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jdbnet/aptuary/internal/api/admin"
	"github.com/jdbnet/aptuary/internal/api/public"
	"github.com/jdbnet/aptuary/internal/audit"
	"github.com/jdbnet/aptuary/internal/auth"
	"github.com/jdbnet/aptuary/internal/config"
	"github.com/jdbnet/aptuary/internal/repo"
	"github.com/jdbnet/aptuary/internal/store"
)

var Version = "dev"

func main() {
	showVersion := flag.Bool("version", false, "print version")
	flag.Parse()
	if *showVersion {
		fmt.Println(Version)
		return
	}

	configPath := ""
	if args := flag.Args(); len(args) > 0 {
		configPath = args[0]
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}

	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		slog.Error("data_dir", "err", err)
		os.Exit(1)
	}
	if err := os.MkdirAll(cfg.RepoDir(), 0o755); err != nil {
		slog.Error("repo_dir", "err", err)
		os.Exit(1)
	}

	// Persist config if missing
	if _, err := os.Stat(cfg.ConfigPath()); os.IsNotExist(err) {
		if err := cfg.Save(); err != nil {
			slog.Error("save config", "err", err)
			os.Exit(1)
		}
	}

	db, err := store.Open(cfg.DataDir)
	if err != nil {
		slog.Error("store", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	authSvc := auth.New(db.SQL)
	username, created, err := authSvc.BootstrapAdmin(os.Getenv("APTUARY_ADMIN_USER"), os.Getenv("APTUARY_ADMIN_PASSWORD"))
	if err != nil {
		slog.Error("bootstrap admin", "err", err)
		os.Exit(1)
	}
	if created {
		slog.Warn("created admin account; change the password after first login", "username", username)
	}
	hasUsers, err := auth.HasUsers(db.SQL)
	if err != nil {
		slog.Error("users", "err", err)
		os.Exit(1)
	}

	al := audit.New(db.SQL)

	signer := &repo.Signer{KeyID: cfg.GPGKeyID, GPGHome: cfg.GPGHome}
	keyID, err := signer.EnsureKey()
	if err != nil {
		slog.Error("gpg", "err", err)
		os.Exit(1)
	}
	if cfg.GPGKeyID != keyID {
		cfg.GPGKeyID = keyID
		if err := cfg.Save(); err != nil {
			slog.Error("save gpg key id", "err", err)
			os.Exit(1)
		}
	}
	slog.Info("gpg key ready", "key_id", keyID)

	repoSvc := repo.NewService(db.SQL, cfg.RepoDir(), signer, func() []repo.RepoConfig {
		var out []repo.RepoConfig
		for _, r := range cfg.Repositories {
			out = append(out, repo.RepoConfig{
				Name:           r.Name,
				Components:     r.Components,
				Architectures:  r.Architectures,
			})
		}
		return out
	})

	if err := repoSvc.PublishIfStale(); err != nil {
		slog.Warn("initial publish", "err", err)
	}

	adminSrv := admin.New(cfg, authSvc, al, repoSvc, signer, hasUsers, Version)
	publicSrv := public.New(cfg, repoSvc, authSvc, al, signer)

	adminHTTP := &http.Server{
		Addr:              cfg.AdminListen,
		Handler:           adminSrv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	publicHTTP := &http.Server{
		Addr:              cfg.PublicListen,
		Handler:           publicSrv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		slog.Info("admin listening", "addr", cfg.AdminListen, "version", Version)
		if err := adminHTTP.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("admin server", "err", err)
			os.Exit(1)
		}
	}()

	go func() {
		slog.Info("public listening", "addr", cfg.PublicListen)
		if err := publicHTTP.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("public server", "err", err)
			os.Exit(1)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	slog.Info("shutting down")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = adminHTTP.Shutdown(ctx)
	_ = publicHTTP.Shutdown(ctx)
}
