package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/meirongdev/trends/internal/config"
	"github.com/meirongdev/trends/internal/github"
	"github.com/meirongdev/trends/internal/ingest"
	"github.com/meirongdev/trends/internal/scheduler"
	"github.com/meirongdev/trends/internal/store"
)

// MVP 阶段的发现查询:按 star 区间切片,后续可迁入配置。
var discoveryQueries = []string{
	"stars:50..100",
	"stars:100..250",
	"stars:250..1000",
	"stars:1000..5000",
	"stars:>5000",
}

func main() {
	cfg := config.Load()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	db, err := store.Open(cfg.DBPath)
	if err != nil {
		slog.Error("open db", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	gh := github.NewClient(cfg.GitHubAPIBaseURL, cfg.GitHubGraphQLURL, cfg.GitHubTokens)

	runDiscovery := func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		_, err := ingest.RunDiscovery(ctx, db, gh, discoveryQueries, 10)
		return err
	}
	runSnapshot := func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
		defer cancel()
		date := time.Now().UTC().Format("2006-01-02")
		return ingest.RunSnapshot(ctx, db, gh, date, 100)
	}

	// RUN_ONCE=discovery|snapshot 用于手动触发一次后退出;作业失败时以非零码退出,便于 CI/cron 感知。
	switch os.Getenv("RUN_ONCE") {
	case "discovery":
		if err := runDiscovery(); err != nil {
			slog.Error("discovery run-once failed", "err", err)
			os.Exit(1)
		}
		return
	case "snapshot":
		if err := runSnapshot(); err != nil {
			slog.Error("snapshot run-once failed", "err", err)
			os.Exit(1)
		}
		return
	}

	sch, err := scheduler.New(
		scheduler.Job{Spec: cfg.DiscoveryCron, Run: func() {
			if err := runDiscovery(); err != nil {
				slog.Error("discovery job", "err", err)
			}
		}},
		scheduler.Job{Spec: cfg.SnapshotCron, Run: func() {
			if err := runSnapshot(); err != nil {
				slog.Error("snapshot job", "err", err)
			}
		}},
	)
	if err != nil {
		slog.Error("scheduler", "err", err)
		os.Exit(1)
	}
	sch.Start()
	defer sch.Stop()
	slog.Info("trends started", "discovery_cron", cfg.DiscoveryCron, "snapshot_cron", cfg.SnapshotCron)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	slog.Info("shutting down")
}
