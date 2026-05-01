package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nabsku/token-tumbler/internal/config"
	"github.com/nabsku/token-tumbler/internal/gitlabutil"
	"github.com/nabsku/token-tumbler/internal/helper"
	"github.com/nabsku/token-tumbler/internal/logger"
	"github.com/nabsku/token-tumbler/internal/runner"
	"github.com/nabsku/token-tumbler/internal/server"
	"go.uber.org/zap"
)

const metricsAddrEnvVar = "TOKEN_TUMBLER_METRICS_ADDR"

func main() {
	l := logger.GetLogger()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := helper.CheckEnvVars("GITLAB_TOKEN", "GITLAB_URL"); err != nil {
		l.Fatal("the following error occurred:", zap.Error(err))
	}

	yamlConfig, err := config.ReadRepositoryConfig("config.yaml")
	if err != nil {
		l.Fatal("reading the yamlConfig failed", zap.Error(err))
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
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	r := runner.NewRunner(gitlabClient, yamlConfig, l)

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
