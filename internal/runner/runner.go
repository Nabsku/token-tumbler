package runner

import (
	"context"
	"errors"
	"time"

	"github.com/nabsku/token-tumbler/internal/group"
	"github.com/nabsku/token-tumbler/internal/logger"
	"github.com/nabsku/token-tumbler/internal/project"
	"github.com/nabsku/token-tumbler/internal/types/repository"
	"gitlab.com/gitlab-org/api/client-go"
	"go.uber.org/zap"
)

var (
	ErrGroupAndRepoDefined = errors.New("you cannot define both a Repository and Group name. Choose one or the other")
)

const (
	cleanupTimeout = 15 * time.Second
)

type Runner struct {
	GitLab *gitlab.Client
	Config *repository.Config
	Logger *zap.Logger
}

func NewRunner(gitlabClient *gitlab.Client, yamlConfig *repository.Config, l *zap.Logger) *Runner {
	return &Runner{
		GitLab: gitlabClient,
		Config: yamlConfig,
		Logger: l,
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
		r.processGroupRepository(ctx, &repo, index, l)
		return
	}
	if repo.RepoName != nil {
		r.processProjectRepository(ctx, &repo, index, l)
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
	vaultTokenID, err := processGroupTokens(ctx, r.GitLab, repo, index, r.Config)
	if err != nil {
		l.Error("Group token processing failed",
			zap.String("operation", "token_process"),
			zap.String("outcome", "failed"),
			zap.Error(err),
		)
		// Continue to cleanup even after token processing failed so we can
		// preserve the vault-stored token if one exists.
	}
	if err := ctx.Err(); err != nil {
		l.Error("Group token processing context ended",
			zap.String("operation", "token_process"),
			zap.String("outcome", "canceled"),
			zap.Error(err),
		)
		return
	}

	cleanupCtx, cancel := context.WithTimeout(ctx, cleanupTimeout)
	defer cancel()

	if err := group.DeleteGroupTokens(cleanupCtx, r.GitLab, repo, r.Config.Prefix, vaultTokenID); err != nil {
		l.Error("Group token deletion failed",
			zap.String("operation", "token_delete"),
			zap.String("outcome", "failed"),
			zap.Error(err),
		)
		return
	}
}

func (r *Runner) processProjectRepository(ctx context.Context, repo *repository.Repository, index int, l *zap.Logger) {
	vaultTokenID, err := processProjectTokens(ctx, r.GitLab, repo, index, r.Config)
	if err != nil {
		l.Error("Project token processing failed",
			zap.String("operation", "token_process"),
			zap.String("outcome", "failed"),
			zap.Error(err),
		)
		// Continue to cleanup even after token processing failed so we can
		// preserve the vault-stored token if one exists.
	}
	if err := ctx.Err(); err != nil {
		l.Error("Project token processing context ended",
			zap.String("operation", "token_process"),
			zap.String("outcome", "canceled"),
			zap.Error(err),
		)
		return
	}

	cleanupCtx, cancel := context.WithTimeout(ctx, cleanupTimeout)
	defer cancel()

	if err := project.DeleteProjectTokens(cleanupCtx, r.GitLab, repo, r.Config.Prefix, vaultTokenID); err != nil {
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
