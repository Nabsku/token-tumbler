package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/nabsku/token-tumbler/internal/config"
	"github.com/nabsku/token-tumbler/internal/group"
	"github.com/nabsku/token-tumbler/internal/helper"
	"github.com/nabsku/token-tumbler/internal/logger"
	"github.com/nabsku/token-tumbler/internal/metrics"
	"github.com/nabsku/token-tumbler/internal/project"
	"github.com/nabsku/token-tumbler/internal/secrets"
	"github.com/nabsku/token-tumbler/internal/types/repository"
	"net/http"

	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gitlab.com/gitlab-org/api/client-go"

	"go.uber.org/zap"
)

var (
	ErrGroupAndRepoDefined = errors.New("you cannot define both a Repository and Group name. Choose one or the other")
)

const (
	defaultPollInterval        = 5 * time.Minute
	operationTimeout           = 30 * time.Second
	errorString         string = "while processing %v at index %v, the following error occurred: %w"
	pollIntervalEnvVar         = "TOKEN_TUMBLER_INTERVAL"
	metricsAddrEnvVar          = "TOKEN_TUMBLER_METRICS_ADDR"
)

func pollIntervalFromEnv() (time.Duration, error) {
	value := strings.TrimSpace(os.Getenv(pollIntervalEnvVar))
	if value == "" {
		return defaultPollInterval, nil
	}
	interval, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", pollIntervalEnvVar, err)
	}
	if interval <= 0 {
		return 0, fmt.Errorf("invalid %s: must be greater than zero", pollIntervalEnvVar)
	}
	return interval, nil
}

func readConfig() (*repository.Config, error) {
	buff, err := os.ReadFile("config.yaml")
	if err != nil {
		return nil, err
	}

	repoConfig := repository.Config{}
	err = yaml.Unmarshal(buff, &repoConfig)
	if err != nil {
		return nil, err
	}
	if err := repoConfig.Validate(); err != nil {
		return nil, err
	}

	return &repoConfig, nil
}

func NewClient() (*gitlab.Client, error) {
	newConfig := config.NewConfig()
	gitlabClient, err := gitlab.NewClient(
		newConfig.GitlabToken,
		gitlab.WithBaseURL(newConfig.GitlabUrl),
		gitlab.WithHTTPClient(&http.Client{Timeout: operationTimeout}),
	)
	if err != nil {
		return nil, err
	}

	return gitlabClient, nil
}

func checkEnvVars(vars ...string) error {
	var missingVars []string
	for _, v := range vars {
		if !helper.CheckAndGetEnvVar(v) {
			missingVars = append(missingVars, v)
		}
	}
	if len(missingVars) > 0 {
		return fmt.Errorf("missing the following environment variables: %v", strings.Join(missingVars, ", "))
	}
	return nil
}

func writeSecret(ctx context.Context, entry *repository.Repository, secret secrets.SecretStore, token string) error {
	if secret == nil {
		return nil
	}

	store := strings.ToLower(strings.TrimSpace(entry.SecretStore))
	l := logger.GetLogger()
	l.Info("Writing secret to selected secret store",
		zap.String("operation", "secret_write"),
		zap.String("secret_store", entry.SecretStore),
		zap.String("token_name", entry.Name),
	)
	if err := secret.Write(ctx, token); err != nil {
		metrics.SecretStoreOperations.WithLabelValues(store, "write", "error").Inc()
		return fmt.Errorf("writing secret to %s: %w", entry.SecretStore, err)
	}
	metrics.SecretStoreOperations.WithLabelValues(store, "write", "success").Inc()
	return nil
}

func matchingGroupTokens(tokens []*gitlab.GroupAccessToken, entry *repository.Repository, prefix string, index int) []*gitlab.GroupAccessToken {
	return matchingTokens(tokens, entry, prefix, *entry.GroupName, index, false, func(token *gitlab.GroupAccessToken) tokenState {
		return tokenState{name: token.Name, active: token.Active, revoked: token.Revoked}
	})
}

func matchingProjectTokens(tokens []*gitlab.ProjectAccessToken, entry *repository.Repository, prefix string, index int) []*gitlab.ProjectAccessToken {
	return matchingTokens(tokens, entry, prefix, *entry.RepoName, index, true, func(token *gitlab.ProjectAccessToken) tokenState {
		return tokenState{name: token.Name, active: token.Active, revoked: token.Revoked}
	})
}

type tokenState struct {
	name    string
	active  bool
	revoked bool
}

func matchingTokens[T any](tokens []T, entry *repository.Repository, prefix string, target string, index int, logMatches bool, state func(T) tokenState) []T {
	l := logger.GetLogger()
	var matches []T
	for _, token := range tokens {
		current := state(token)
		if current.revoked || !current.active {
			continue
		}
		if ok, err := entry.ParseTokenName(prefix, current.name); ok {
			if logMatches {
				l.Info("token matches prefix, appending to check queue", zap.String("token_name", current.name))
			}
			matches = append(matches, token)
		} else if err != nil {
			l.Debug(fmt.Errorf(errorString, target, index, err).Error())
			continue
		}
	}
	return matches
}

func processGroupTokens(ctx context.Context, gitlabClient *gitlab.Client, entry *repository.Repository, index int, yamlConfig *repository.Config) error {
	start := time.Now()
	l := logger.GetLogger()
	store := strings.ToLower(strings.TrimSpace(entry.SecretStore))

	var groupToken *gitlab.GroupAccessToken

	info, err := group.GatherGroup(gitlabClient, entry)
	if err != nil {
		metrics.TokenRotations.WithLabelValues("group", entry.Name, store, "error").Inc()
		return fmt.Errorf(errorString, *entry.GroupName, index, err)
	}

	if info == nil {
		metrics.TokenRotations.WithLabelValues("group", entry.Name, store, "error").Inc()
		return fmt.Errorf("no group returned for %v, skipping", *entry.GroupName)
	}

	tokenInfo, err := group.GatherGroupTokenInfo(gitlabClient, info.ID)
	if err != nil {
		metrics.TokenRotations.WithLabelValues("group", entry.Name, store, "error").Inc()
		return fmt.Errorf(errorString, *entry.GroupName, index, err)
	}

	tokenQueue := matchingGroupTokens(tokenInfo, entry, yamlConfig.Prefix, index)
	metrics.ActiveTokens.WithLabelValues("group", entry.Name).Set(float64(len(tokenQueue)))

	secret, err := secrets.ForRepository(entry)
	if err != nil {
		return err
	}
	if secret != nil {
		if err := secret.InitClient(ctx); err != nil {
			return fmt.Errorf("initializing secret store for %s: %w", entry.Name, err)
		}
	}

	if len(tokenQueue) < 1 {
		l.Info("no tokens found in group, creating new token", zap.String("group", *entry.GroupName))
		token, errTokenCreation := group.CreateNewGroupToken(gitlabClient, info.ID, entry, yamlConfig.Prefix)
		if errTokenCreation != nil {
			metrics.TokenRotations.WithLabelValues("group", entry.Name, store, "error").Inc()
			return fmt.Errorf(errorString, *entry.GroupName, index, errTokenCreation)
		}
		groupToken = token
	}

	needsRenewal, err := group.CheckGroupTokensForRenewal(tokenQueue, entry)
	if err != nil {
		metrics.TokenRotations.WithLabelValues("group", entry.Name, store, "error").Inc()
		return fmt.Errorf(errorString, *entry.GroupName, index, err)
	}

	if needsRenewal {
		l.Info("token ready for renewal", zap.String("token_name", entry.Name), zap.String("group", *entry.GroupName))
		token, errRenewal := group.RenewGroupAccessToken(gitlabClient, info.ID, entry, yamlConfig.Prefix)
		if errRenewal != nil {
			metrics.TokenRotations.WithLabelValues("group", entry.Name, store, "error").Inc()
			return fmt.Errorf(errorString, entry.Name, index, errRenewal)
		}
		groupToken = token
	} else {
		l.Info("no tokens need renewal", zap.String("token_name", entry.Name), zap.String("group", *entry.GroupName))
	}

	if groupToken == nil {
		return nil
	}

	metrics.TokenRotations.WithLabelValues("group", entry.Name, store, "success").Inc()
	metrics.TokenRotationDuration.WithLabelValues("group", entry.Name).Observe(time.Since(start).Seconds())

	return writeSecret(ctx, entry, secret, groupToken.Token)
}

func processProjectTokens(ctx context.Context, gitlabClient *gitlab.Client, entry *repository.Repository, index int, yamlConfig *repository.Config) error {
	start := time.Now()
	var projectToken *gitlab.ProjectAccessToken
	store := strings.ToLower(strings.TrimSpace(entry.SecretStore))

	l := logger.GetLogger()

	info, err := project.GatherProject(gitlabClient, entry)
	if err != nil {
		metrics.TokenRotations.WithLabelValues("project", entry.Name, store, "error").Inc()
		return fmt.Errorf(errorString, *entry.RepoName, index, err)
	}

	if info == nil {
		metrics.TokenRotations.WithLabelValues("project", entry.Name, store, "error").Inc()
		return fmt.Errorf("no project returned for %v, skipping", *entry.RepoName)
	}

	tokenInfo, err := project.GatherProjectTokenInfo(gitlabClient, info.ID)
	if err != nil {
		metrics.TokenRotations.WithLabelValues("project", entry.Name, store, "error").Inc()
		return fmt.Errorf(errorString, *entry.RepoName, index, err)
	}

	tokenQueue := matchingProjectTokens(tokenInfo, entry, yamlConfig.Prefix, index)
	metrics.ActiveTokens.WithLabelValues("project", entry.Name).Set(float64(len(tokenQueue)))

	secret, err := secrets.ForRepository(entry)
	if err != nil {
		return err
	}
	if secret != nil {
		if err := secret.InitClient(ctx); err != nil {
			return fmt.Errorf("initializing secret store for %s: %w", entry.Name, err)
		}
	}

	if len(tokenQueue) < 1 {
		l.Info("no tokens found in repo, creating new token", zap.String("repo", *entry.RepoName))

		token, errTokenCreation := project.CreateNewProjectToken(gitlabClient, info.ID, entry, yamlConfig.Prefix)
		if errTokenCreation != nil {
			metrics.TokenRotations.WithLabelValues("project", entry.Name, store, "error").Inc()
			return fmt.Errorf(errorString, *entry.RepoName, index, errTokenCreation)
		}
		projectToken = token
	}

	needsRenewal, err := project.CheckProjectTokensForRenewal(tokenQueue, entry)
	if err != nil {
		metrics.TokenRotations.WithLabelValues("project", entry.Name, store, "error").Inc()
		return fmt.Errorf(errorString, *entry.RepoName, index, err)
	}

	if needsRenewal {
		l.Info("token ready for renewal", zap.String("token_name", entry.Name), zap.String("repo", *entry.RepoName))
		token, errRenewal := project.RenewProjectAccessToken(gitlabClient, info.ID, entry, yamlConfig.Prefix)
		if errRenewal != nil {
			metrics.TokenRotations.WithLabelValues("project", entry.Name, store, "error").Inc()
			return fmt.Errorf(errorString, entry.Name, index, errRenewal)
		}
		projectToken = token
	} else {
		l.Info("no tokens need renewal", zap.String("token_name", entry.Name), zap.String("repo", *entry.RepoName))
	}

	if projectToken == nil {
		return nil
	}

	metrics.TokenRotations.WithLabelValues("project", entry.Name, store, "success").Inc()
	metrics.TokenRotationDuration.WithLabelValues("project", entry.Name).Observe(time.Since(start).Seconds())

	return writeSecret(ctx, entry, secret, projectToken.Token)
}

type Runner struct {
	GitLab           *gitlab.Client
	Config           *repository.Config
	Logger           *zap.Logger
	OperationTimeout time.Duration
}

func NewRunner(gitlabClient *gitlab.Client, yamlConfig *repository.Config, l *zap.Logger) *Runner {
	return &Runner{
		GitLab:           gitlabClient,
		Config:           yamlConfig,
		Logger:           l,
		OperationTimeout: operationTimeout,
	}
}

func (r *Runner) RunOnce(ctx context.Context) {
	if r == nil || r.Config == nil {
		return
	}
	for index, repo := range r.Config.Repos {
		if err := ctx.Err(); err != nil {
			r.logger().Info("Shutdown signal received, stopping token chaser")
			return
		}
		r.ProcessRepository(ctx, repo, index)
	}
}

func (r *Runner) logger() *zap.Logger {
	if r != nil && r.Logger != nil {
		return r.Logger
	}
	return logger.GetLogger()
}

func (r *Runner) timeout() time.Duration {
	if r != nil && r.OperationTimeout > 0 {
		return r.OperationTimeout
	}
	return operationTimeout
}

func (r *Runner) ProcessRepository(ctx context.Context, repo repository.Repository, index int) {
	started := time.Now()
	l := r.logger().With(repositoryLogFields(repo, index)...)
	if err := r.validateDependencies(); err != nil {
		l.Error("Repository processing failed before start",
			zap.String("operation", "runner_validate"),
			zap.String("outcome", "failed"),
			zap.Error(err),
		)
		return
	}
	entryCtx, cancel := context.WithTimeout(ctx, r.timeout())
	defer cancel()
	if err := entryCtx.Err(); err != nil {
		l.Info("Skipping repository processing because context is done",
			zap.String("operation", "repository_process"),
			zap.String("outcome", "canceled"),
			zap.Error(err),
		)
		return
	}
	l.Info("Started repository processing",
		zap.String("operation", "repository_process"),
		zap.String("outcome", "started"),
	)
	defer func() {
		l.Info("Finished repository processing",
			zap.String("operation", "repository_process"),
			zap.String("outcome", "finished"),
			zap.Duration("duration", time.Since(started)),
		)
	}()

	if err := repo.CheckKeyRotationAndTokenAge(); err != nil {
		l.Warn("Repository configuration failed validation",
			zap.String("operation", "config_validate"),
			zap.String("outcome", "failed"),
			zap.Error(err),
		)
		return
	}
	if repo.GroupName != nil && repo.RepoName != nil {
		l.Warn("Repository target failed validation",
			zap.String("operation", "target_validate"),
			zap.String("outcome", "failed"),
			zap.Error(ErrGroupAndRepoDefined),
		)
		return
	}
	if repo.GroupName != nil {
		r.processGroupRepository(entryCtx, &repo, index, l)
		return
	}
	if repo.RepoName != nil {
		r.processProjectRepository(entryCtx, &repo, index, l)
		return
	}
}

func (r *Runner) validateDependencies() error {
	if r == nil {
		return errors.New("runner is nil")
	}
	if r.GitLab == nil {
		return errors.New("gitlab client is nil")
	}
	if r.Config == nil {
		return errors.New("repository config is nil")
	}
	return nil
}

func (r *Runner) processGroupRepository(ctx context.Context, repo *repository.Repository, index int, l *zap.Logger) {
	if err := processGroupTokens(ctx, r.GitLab, repo, index, r.Config); err != nil {
		l.Error("Group token processing failed",
			zap.String("operation", "token_process"),
			zap.String("outcome", "failed"),
			zap.Error(err),
		)
		return
	}
	if err := ctx.Err(); err != nil {
		l.Error("Group token processing context ended",
			zap.String("operation", "token_process"),
			zap.String("outcome", "canceled"),
			zap.Error(err),
		)
		return
	}
	if err := group.DeleteGroupTokens(r.GitLab, repo, r.Config.Prefix); err != nil {
		l.Error("Group token deletion failed",
			zap.String("operation", "token_delete"),
			zap.String("outcome", "failed"),
			zap.Error(err),
		)
		return
	}
}

func (r *Runner) processProjectRepository(ctx context.Context, repo *repository.Repository, index int, l *zap.Logger) {
	if err := processProjectTokens(ctx, r.GitLab, repo, index, r.Config); err != nil {
		l.Error("Project token processing failed",
			zap.String("operation", "token_process"),
			zap.String("outcome", "failed"),
			zap.Error(err),
		)
		return
	}
	if err := ctx.Err(); err != nil {
		l.Error("Project token processing context ended",
			zap.String("operation", "token_process"),
			zap.String("outcome", "canceled"),
			zap.Error(err),
		)
		return
	}
	if err := project.DeleteProjectTokens(r.GitLab, repo, r.Config.Prefix); err != nil {
		l.Error("Project token deletion failed",
			zap.String("operation", "token_delete"),
			zap.String("outcome", "failed"),
			zap.Error(err),
		)
		return
	}
}

func startHTTPServer(ctx context.Context, l *zap.Logger, addr string) {
	if addr == "" {
		addr = ":9090"
	}
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	server := &http.Server{Addr: addr, Handler: mux}
	go func() {
		l.Info("Starting HTTP server", zap.String("addr", addr))
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			l.Error("HTTP server error", zap.Error(err))
		}
	}()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			l.Error("HTTP server shutdown error", zap.Error(err))
		}
	}()
}

func repositoryLogFields(repo repository.Repository, index int) []zap.Field {
	fields := []zap.Field{
		zap.Int("repository_index", index),
		zap.String("token_name", repo.Name),
	}
	if repo.GroupName != nil {
		fields = append(fields,
			zap.String("target_type", "group"),
			zap.String("target", *repo.GroupName),
		)
	}
	if repo.RepoName != nil {
		fields = append(fields,
			zap.String("target_type", "project"),
			zap.String("target", *repo.RepoName),
		)
	}
	return fields
}

func main() {
	l := logger.GetLogger()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := checkEnvVars("GITLAB_TOKEN", "GITLAB_URL"); err != nil {
		l.Fatal("the following error occurred:", zap.Error(err))
	}

	yamlConfig, err := readConfig()
	if err != nil {
		l.Fatal("reading the yamlConfig failed", zap.Error(err))
	}
	if yamlConfig.UsesVaultAppRole() {
		if err := checkEnvVars("APPROLE_ID", "APPROLE_SECRET"); err != nil {
			l.Fatal("the following error occurred:", zap.Error(err))
		}
	}
	if yamlConfig.UsesVaultToken() {
		if err := checkEnvVars("VAULT_TOKEN"); err != nil {
			l.Fatal("the following error occurred:", zap.Error(err))
		}
	}

	gitlabClient, err := NewClient()
	if err != nil {
		l.Fatal("initialising the gitlab client failed", zap.Error(err))
	}

	startHTTPServer(ctx, l, os.Getenv(metricsAddrEnvVar))

	pollInterval, err := pollIntervalFromEnv()
	if err != nil {
		l.Fatal("reading poll interval failed", zap.Error(err))
	}
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	runner := NewRunner(gitlabClient, yamlConfig, l)

	for {
		select {
		case <-ctx.Done():
			l.Info("Shutdown signal received, stopping token chaser")
			return
		case <-ticker.C:
			runner.RunOnce(ctx)
		}
	}
}
