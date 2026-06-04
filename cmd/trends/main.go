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

	"github.com/meirongdev/trends/internal/api"
	"github.com/meirongdev/trends/internal/auth"
	"github.com/meirongdev/trends/internal/config"
	"github.com/meirongdev/trends/internal/github"
	"github.com/meirongdev/trends/internal/ingest"
	"github.com/meirongdev/trends/internal/scheduler"
	"github.com/meirongdev/trends/internal/scoring"
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

func todayUTC() string { return time.Now().UTC().Format("2006-01-02") }

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
	scoreCfg := scoring.DefaultConfig()

	runDiscovery := func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		if _, err := ingest.RunDiscovery(ctx, db, gh, discoveryQueries, 10); err != nil {
			return err
		}
		// 处理用户提交的收录请求,把存在的仓库纳入宇宙。
		return ingest.RunSubmissions(ctx, db, gh, 200)
	}
	runSnapshot := func(date string) error {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
		defer cancel()
		return ingest.RunSnapshot(ctx, db, gh, date, 100)
	}
	runScoring := func(date string) error {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		return ingest.RunScoring(ctx, db, date, scoreCfg)
	}

	// RUN_ONCE=discovery|snapshot|score 用于手动触发一次后退出;失败以非零码退出。不起 HTTP 服务。
	switch os.Getenv("RUN_ONCE") {
	case "discovery":
		if err := runDiscovery(); err != nil {
			slog.Error("discovery run-once failed", "err", err)
			os.Exit(1)
		}
		return
	case "snapshot":
		if err := runSnapshot(todayUTC()); err != nil {
			slog.Error("snapshot run-once failed", "err", err)
			os.Exit(1)
		}
		return
	case "score":
		if err := runScoring(todayUTC()); err != nil {
			slog.Error("score run-once failed", "err", err)
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
		// 快照成功后链式评分;两步共用同一 as-of 日期,确保榜单与刚写入的快照对齐
		// (避免快照跨过 UTC 午夜时,评分用到与快照不同的日期)。
		scheduler.Job{Spec: cfg.SnapshotCron, Run: func() {
			date := todayUTC()
			if err := runSnapshot(date); err != nil {
				slog.Error("snapshot job", "err", err)
				return
			}
			if err := runScoring(date); err != nil {
				slog.Error("scoring job", "err", err)
			}
		}},
	)
	if err != nil {
		slog.Error("scheduler", "err", err)
		os.Exit(1)
	}
	sch.Start()
	defer sch.Stop()

	providers := auth.NewProviders(auth.Config{
		BaseURL:            cfg.OAuthBaseURL,
		GitHubClientID:     cfg.GitHubOAuthClientID,
		GitHubClientSecret: cfg.GitHubOAuthClientSecret,
		GoogleClientID:     cfg.GoogleOAuthClientID,
		GoogleClientSecret: cfg.GoogleOAuthClientSecret,
	})
	httpServer := &http.Server{
		Addr:              cfg.APIListenAddr,
		Handler:           api.NewServer(db, providers, cfg.OAuthBaseURL).Routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		slog.Info("api listening", "addr", cfg.APIListenAddr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("http server", "err", err)
		}
	}()

	slog.Info("trends started", "discovery_cron", cfg.DiscoveryCron, "snapshot_cron", cfg.SnapshotCron)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	slog.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		slog.Error("http shutdown", "err", err)
	}
}
