package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/nabsku/token-tumbler/internal/config"
	"github.com/nabsku/token-tumbler/internal/group"
	"github.com/nabsku/token-tumbler/internal/helper"
	"github.com/nabsku/token-tumbler/internal/logger"
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

func writeSecret(ctx context.Context, entry *repository.Repository, secret secrets.SecretStore) error {
	if secret == nil {
		return nil
	}

	l := logger.GetLogger()
	l.Info("Writing secret to selected secret store",
		zap.String("operation", "secret_write"),
		zap.String("secret_store", entry.SecretStore),
		zap.String("token_name", entry.Name),
	)
	if err := secret.Write(ctx); err != nil {
		return fmt.Errorf("writing secret to %s: %w", entry.SecretStore, err)
	}
	return nil
}

func matchingGroupTokens(tokens []*gitlab.GroupAccessToken, entry *repository.Repository, prefix string, index int) []*gitlab.GroupAccessToken {
	l := logger.GetLogger()
	var matches []*gitlab.GroupAccessToken
	for _, token := range tokens {
		if token.Revoked || !token.Active {
			continue
		}
		if ok, err := entry.ParseTokenName(prefix, token.Name); ok {
			matches = append(matches, token)
		} else if err != nil {
			l.Debug(fmt.Errorf(errorString, *entry.GroupName, index, err).Error())
			continue
		}
	}
	return matches
}

func matchingProjectTokens(tokens []*gitlab.ProjectAccessToken, entry *repository.Repository, prefix string, index int) []*gitlab.ProjectAccessToken {
	l := logger.GetLogger()
	var matches []*gitlab.ProjectAccessToken
	for _, token := range tokens {
		if token.Revoked || !token.Active {
			continue
		}
		if ok, err := entry.ParseTokenName(prefix, token.Name); ok {
			l.Info(fmt.Sprintf("Token %v is valid, appending to queue of tokens to check further", token.Name))
			matches = append(matches, token)
		} else if err != nil {
			l.Debug(fmt.Errorf(errorString, *entry.RepoName, index, err).Error())
			continue
		}
	}
	return matches
}

func processGroupTokens(ctx context.Context, gitlabClient *gitlab.Client, entry *repository.Repository, index int, yamlConfig *repository.Config) error {
	l := logger.GetLogger()

	var groupToken *gitlab.GroupAccessToken

	info, err := group.GatherGroup(gitlabClient, entry)
	if err != nil {
		return fmt.Errorf(errorString, *entry.GroupName, index, err)
	}

	if info == nil {
		return fmt.Errorf("no group returned for %v, skipping", *entry.GroupName)
	}

	tokenInfo, err := group.GatherGroupTokenInfo(gitlabClient, info.ID)

	if errors.Is(err, group.ErrTooManyGroupsInSearch) {
		return fmt.Errorf(errorString, *entry.GroupName, index, group.ErrTooManyGroupsInSearch)
	} else if err != nil {
		return fmt.Errorf(errorString, *entry.GroupName, index, err)
	}

	tokenQueue := matchingGroupTokens(tokenInfo, entry, yamlConfig.Prefix, index)

	if len(tokenQueue) < 1 {
		l.Info(fmt.Sprintf("No token in group %v yet, we're free to create one as we please.", *entry.GroupName))
		token, errTokenCreation := group.CreateNewGroupToken(gitlabClient, info.ID, entry, yamlConfig.Prefix)
		if errTokenCreation != nil {
			return fmt.Errorf(errorString, *entry.GroupName, index, errTokenCreation)
		}
		groupToken = token
	}

	needsRenewal, err := group.CheckGroupTokensForRenewal(tokenQueue, entry)
	if err != nil {
		return fmt.Errorf(errorString, *entry.GroupName, index, err)
	}

	if needsRenewal {
		l.Info(fmt.Sprintf("Token for %v in Group %v is ready to be renewed.\n", entry.Name, *entry.GroupName))
		token, errRenewal := group.RenewGroupAccessToken(gitlabClient, info.ID, entry, yamlConfig.Prefix)
		if errRenewal != nil {
			return fmt.Errorf(errorString, entry.Name, index, errRenewal)
		}
		groupToken = token
	} else {
		l.Info(fmt.Sprintf("No tokens for %v in Group %v need renewal at this time.\n", entry.Name, *entry.GroupName))
	}

	if groupToken == nil {
		return nil
	}

	secret, err := secrets.ForRepository(entry, groupToken.Token)
	if err != nil {
		return err
	}
	return writeSecret(ctx, entry, secret)
}

func processProjectTokens(ctx context.Context, gitlabClient *gitlab.Client, entry *repository.Repository, index int, yamlConfig *repository.Config) error {
	var projectToken *gitlab.ProjectAccessToken

	l := logger.GetLogger()

	info, err := project.GatherProject(gitlabClient, entry)
	if err != nil {
		return fmt.Errorf(errorString, *entry.RepoName, index, err)
	}

	if info == nil {
		return fmt.Errorf("no project returned for %v, skipping", *entry.RepoName)
	}

	tokenInfo, err := project.GatherProjectTokenInfo(gitlabClient, info.ID)

	if errors.Is(err, project.ErrTooManyProjectsInSearch) {
		return fmt.Errorf(errorString, *entry.RepoName, index, project.ErrTooManyProjectsInSearch)
	} else if err != nil {
		return fmt.Errorf(errorString, *entry.RepoName, index, err)
	}

	tokenQueue := matchingProjectTokens(tokenInfo, entry, yamlConfig.Prefix, index)

	if len(tokenQueue) < 1 {
		l.Info(fmt.Sprintf("No token yet for repo %v, we're free to create one as we please.", *entry.RepoName))

		token, errTokenCreation := project.CreateNewProjectToken(gitlabClient, info.ID, entry, yamlConfig.Prefix)
		if errTokenCreation != nil {
			return fmt.Errorf(errorString, *entry.RepoName, index, errTokenCreation)
		}
		projectToken = token
	}

	needsRenewal, err := project.CheckProjectTokensForRenewal(tokenQueue, entry)
	if err != nil {
		return fmt.Errorf(errorString, *entry.RepoName, index, err)
	}

	if needsRenewal {
		l.Info(fmt.Sprintf("Token for %v in Repo %v is ready to be renewed.\n", entry.Name, *entry.RepoName))
		token, errRenewal := project.RenewProjectAccessToken(gitlabClient, info.ID, entry, yamlConfig.Prefix)
		if errRenewal != nil {
			return fmt.Errorf(errorString, entry.Name, index, errRenewal)
		}
		projectToken = token
	} else {
		l.Info(fmt.Sprintf("No tokens for %v in Repo %v need renewal at this time.\n", entry.Name, *entry.RepoName))
	}

	if projectToken == nil {
		return nil
	}

	secret, err := secrets.ForRepository(entry, projectToken.Token)
	if err != nil {
		return err
	}
	return writeSecret(ctx, entry, secret)
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
	if yamlConfig.UsesVault() {
		if err := checkEnvVars("APPROLE_ID", "APPROLE_SECRET"); err != nil {
			l.Fatal("the following error occurred:", zap.Error(err))
		}
	}

	gitlabClient, err := NewClient()
	if err != nil {
		l.Fatal("initialising the gitlab client failed", zap.Error(err))
	}

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
