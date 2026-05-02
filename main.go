package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nabsku/token-tumbler/internal/config"
	"github.com/nabsku/token-tumbler/internal/gitlabutil"
	"github.com/nabsku/token-tumbler/internal/helper"
	leader "github.com/nabsku/token-tumbler/internal/leaderelection"
	"github.com/nabsku/token-tumbler/internal/logger"
	"github.com/nabsku/token-tumbler/internal/runner"
	"github.com/nabsku/token-tumbler/internal/server"
	"go.uber.org/zap"
)

const metricsAddrEnvVar = "TOKEN_TUMBLER_METRICS_ADDR"

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "version") {
		fmt.Printf("token-tumbler %s (%s, %s)\n", version, commit, date)
		return
	}

	l := logger.GetLogger()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := helper.CheckEnvVars("GITLAB_TOKEN"); err != nil {
		l.Fatal("the following error occurred:", zap.Error(err))
	}

	yamlConfig, err := config.ReadRepositoryConfig("config.yaml")
	if err != nil {
		l.Fatal("reading the yamlConfig failed", zap.Error(err))
	}
	if err := helper.CheckEnvVars("GITLAB_URL"); err != nil {
		l.Fatal("the following error occurred:", zap.Error(err))
	}
	if yamlConfig.UsesVaultAppRole() {
		if err := helper.CheckEnvVars("APPROLE_ID", "APPROLE_SECRET"); err != nil {
			l.Fatal("the following error occurred:", zap.Error(err))
		}
	}
	if yamlConfig.UsesVaultToken() {
		if err := helper.CheckEnvVars("VAULT_TOKEN"); err != nil {
			l.Fatal("the following error occurred:", zap.Error(err))
		}
	}

	gitlabClient, err := gitlabutil.NewClient()
	if err != nil {
		l.Fatal("initialising the gitlab client failed", zap.Error(err))
	}

	server.StartHTTPServer(ctx, l, os.Getenv(metricsAddrEnvVar))

	pollInterval, err := config.PollIntervalFromEnv()
	if err != nil {
		l.Fatal("reading poll interval failed", zap.Error(err))
	}
	r := runner.NewRunner(gitlabClient, yamlConfig, l)
	leaderConfig, err := leader.ConfigFromEnv()
	if err != nil {
		l.Fatal("reading leader election config failed", zap.Error(err))
	}
	if leaderConfig.Enabled {
		elector := leader.NewRunner(leaderConfig, l)
		if err := elector.Run(ctx, func(leaderCtx context.Context) { runLoop(leaderCtx, l, r, pollInterval) }); err != nil {
			l.Fatal("leader election failed", zap.Error(err))
		}
		return
	}

	runLoop(ctx, l, r, pollInterval)
}

func runLoop(ctx context.Context, l *zap.Logger, r *runner.Runner, pollInterval time.Duration) {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			l.Info("Shutdown signal received, stopping token chaser")
			return
		case <-ticker.C:
			r.RunOnce(ctx)
		}
	}
}
