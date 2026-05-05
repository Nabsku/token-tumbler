package runner

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/nabsku/token-tumbler/internal/group"
	"github.com/nabsku/token-tumbler/internal/logger"
	"github.com/nabsku/token-tumbler/internal/metrics"
	"github.com/nabsku/token-tumbler/internal/project"
	"github.com/nabsku/token-tumbler/internal/secrets"
	"github.com/nabsku/token-tumbler/internal/types/repository"
	"gitlab.com/gitlab-org/api/client-go"
	"go.uber.org/zap"
)

const (
	gatherTimeout     = 15 * time.Second
	createTimeout     = 15 * time.Second
	vaultWriteTimeout = 10 * time.Second
	rollbackTimeout   = 10 * time.Second
	errorString       = "while processing %v at index %v, the following error occurred: %w"
)

type tokenState struct {
	name    string
	active  bool
	revoked bool
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

func readVaultMetadata(ctx context.Context, secret secrets.SecretStore) (secrets.TokenMetadata, error) {
	if secret == nil {
		return secrets.TokenMetadata{}, nil
	}
	return secret.ReadMetadata(ctx)
}

func persistToken(ctx context.Context, entry *repository.Repository, secret secrets.SecretStore, token string, meta secrets.TokenMetadata) error {
	if secret == nil {
		return nil
	}

	// Capture current secret value so we can restore it if metadata write fails.
	// This prevents consumers from receiving a revoked token after rollback.
	oldValue, readErr := secret.Read(ctx)

	if err := secret.Write(ctx, token); err != nil {
		return fmt.Errorf("writing token: %w", err)
	}

	if err := secret.WriteMetadata(ctx, meta); err != nil {
		// Restore previous token value so consumers don't receive a revoked token.
		if readErr == nil {
			if restoreErr := secret.Write(ctx, oldValue); restoreErr != nil {
				return fmt.Errorf("writing metadata failed and unable to restore previous token: %w (restore error: %v)", err, restoreErr)
			}
		} else if cleaner, ok := secret.(interface{ DeleteCreatedSecret(context.Context) error }); ok {
			if cleanupErr := cleaner.DeleteCreatedSecret(ctx); cleanupErr != nil {
				return fmt.Errorf("writing metadata failed and unable to delete newly created token secret: %w (cleanup error: %v)", err, cleanupErr)
			}
		}
		return fmt.Errorf("writing metadata: %w", err)
	}
	return nil
}

func rollbackGroupToken(ctx context.Context, gitlabClient *gitlab.Client, groupID int64, tokenID int64) error {
	_, err := gitlabClient.GroupAccessTokens.RevokeGroupAccessToken(groupID, tokenID, gitlab.WithContext(ctx))
	return err
}

func rollbackProjectToken(ctx context.Context, gitlabClient *gitlab.Client, projectID int64, tokenID int64) error {
	_, err := gitlabClient.ProjectAccessTokens.RevokeProjectAccessToken(projectID, tokenID, gitlab.WithContext(ctx))
	return err
}

func processGroupTokens(ctx context.Context, gitlabClient *gitlab.Client, entry *repository.Repository, index int, yamlConfig *repository.Config) (int64, error) {
	start := time.Now()
	l := logger.GetLogger()
	store := strings.ToLower(strings.TrimSpace(entry.SecretStore))

	secret, err := secrets.ForRepository(entry)
	if err != nil {
		return 0, err
	}
	if secret != nil {
		if err := secret.InitClient(ctx); err != nil {
			return 0, fmt.Errorf("initializing secret store for %s: %w", entry.Name, err)
		}
	}

	// Read existing vault metadata so cleanup can preserve the currently persisted token on failure.
	existingMeta, _ := readVaultMetadata(ctx, secret)
	vaultTokenID := existingMeta.TokenID

	// Gather phase
	gatherCtx, cancel := context.WithTimeout(ctx, gatherTimeout)
	defer cancel()

	info, err := group.GatherGroup(gatherCtx, gitlabClient, entry)
	if err != nil {
		metrics.TokenRotations.WithLabelValues("group", entry.Name, store, "error").Inc()
		return vaultTokenID, fmt.Errorf(errorString, *entry.GroupName, index, err)
	}

	if info == nil {
		metrics.TokenRotations.WithLabelValues("group", entry.Name, store, "error").Inc()
		return vaultTokenID, fmt.Errorf("no group returned for %v, skipping", *entry.GroupName)
	}

	tokenInfo, err := group.GatherGroupTokenInfo(gatherCtx, gitlabClient, info.ID)
	if err != nil {
		metrics.TokenRotations.WithLabelValues("group", entry.Name, store, "error").Inc()
		return vaultTokenID, fmt.Errorf(errorString, *entry.GroupName, index, err)
	}

	tokenQueue := matchingGroupTokens(tokenInfo, entry, yamlConfig.Prefix, index)
	metrics.ActiveTokens.WithLabelValues("group", entry.Name).Set(float64(len(tokenQueue)))

	var groupToken *gitlab.GroupAccessToken

	if len(tokenQueue) < 1 {
		l.Info("no tokens found in group, creating new token", zap.String("group", *entry.GroupName))

		createCtx, cancel := context.WithTimeout(ctx, createTimeout)
		defer cancel()

		token, errTokenCreation := group.CreateNewGroupToken(createCtx, gitlabClient, info.ID, entry, yamlConfig.Prefix)
		if errTokenCreation != nil {
			metrics.TokenRotations.WithLabelValues("group", entry.Name, store, "error").Inc()
			return vaultTokenID, fmt.Errorf(errorString, *entry.GroupName, index, errTokenCreation)
		}
		groupToken = token
	}

	needsRenewal, err := group.CheckGroupTokensForRenewal(tokenQueue, entry, vaultTokenID)
	if err != nil {
		metrics.TokenRotations.WithLabelValues("group", entry.Name, store, "error").Inc()
		return vaultTokenID, fmt.Errorf(errorString, *entry.GroupName, index, err)
	}

	if needsRenewal {
		l.Info("token ready for renewal", zap.String("token_name", entry.Name), zap.String("group", *entry.GroupName))

		createCtx, cancel := context.WithTimeout(ctx, createTimeout)
		defer cancel()

		token, errRenewal := group.RenewGroupAccessToken(createCtx, gitlabClient, info.ID, entry, yamlConfig.Prefix)
		if errRenewal != nil {
			metrics.TokenRotations.WithLabelValues("group", entry.Name, store, "error").Inc()
			return vaultTokenID, fmt.Errorf(errorString, entry.Name, index, errRenewal)
		}
		groupToken = token
	} else {
		l.Info("no tokens need renewal", zap.String("token_name", entry.Name), zap.String("group", *entry.GroupName))
	}

	if groupToken == nil {
		return vaultTokenID, nil
	}

	// Vault write phase
	vaultCtx, cancel := context.WithTimeout(ctx, vaultWriteTimeout)
	defer cancel()

	meta := secrets.TokenMetadata{
		TokenID:   groupToken.ID,
		TokenName: groupToken.Name,
	}
	if groupToken.CreatedAt != nil {
		meta.CreatedAt = *groupToken.CreatedAt
	}

	if err := persistToken(vaultCtx, entry, secret, groupToken.Token, meta); err != nil {
		metrics.TokenRotations.WithLabelValues("group", entry.Name, store, "error").Inc()

		// Rollback phase: revoke newly created token so it does not become an orphan.
		rollbackCtx, cancel := context.WithTimeout(ctx, rollbackTimeout)
		defer cancel()

		metrics.TokenRollbackAttempts.WithLabelValues("group", entry.Name).Inc()
		if revokeErr := rollbackGroupToken(rollbackCtx, gitlabClient, info.ID, groupToken.ID); revokeErr != nil {
			metrics.TokenRollbackOutcomes.WithLabelValues("group", entry.Name, "failure").Inc()
			l.Error("rollback revoke failed after vault write failure; orphan token may exist",
				zap.String("token_name", groupToken.Name),
				zap.Int64("token_id", groupToken.ID),
				zap.Error(revokeErr),
			)
			return vaultTokenID, fmt.Errorf("vault write failed after token creation: %w; rollback revoke also failed: %v", err, revokeErr)
		}
		metrics.TokenRollbackOutcomes.WithLabelValues("group", entry.Name, "success").Inc()
		l.Info("rollback revoke succeeded after vault write failure",
			zap.String("token_name", groupToken.Name),
			zap.Int64("token_id", groupToken.ID),
		)
		return vaultTokenID, fmt.Errorf("vault write failed after token creation; new token revoked: %w", err)
	}

	metrics.TokenRotations.WithLabelValues("group", entry.Name, store, "success").Inc()
	metrics.TokenRotationDuration.WithLabelValues("group", entry.Name).Observe(time.Since(start).Seconds())

	return groupToken.ID, nil
}

func processProjectTokens(ctx context.Context, gitlabClient *gitlab.Client, entry *repository.Repository, index int, yamlConfig *repository.Config) (int64, error) {
	start := time.Now()
	var projectToken *gitlab.ProjectAccessToken
	store := strings.ToLower(strings.TrimSpace(entry.SecretStore))

	l := logger.GetLogger()

	secret, err := secrets.ForRepository(entry)
	if err != nil {
		return 0, err
	}
	if secret != nil {
		if err := secret.InitClient(ctx); err != nil {
			return 0, fmt.Errorf("initializing secret store for %s: %w", entry.Name, err)
		}
	}

	// Read existing vault metadata so cleanup can preserve the currently persisted token on failure.
	existingMeta, _ := readVaultMetadata(ctx, secret)
	vaultTokenID := existingMeta.TokenID

	// Gather phase
	gatherCtx, cancel := context.WithTimeout(ctx, gatherTimeout)
	defer cancel()

	info, err := project.GatherProject(gatherCtx, gitlabClient, entry)
	if err != nil {
		metrics.TokenRotations.WithLabelValues("project", entry.Name, store, "error").Inc()
		return vaultTokenID, fmt.Errorf(errorString, *entry.RepoName, index, err)
	}

	if info == nil {
		metrics.TokenRotations.WithLabelValues("project", entry.Name, store, "error").Inc()
		return vaultTokenID, fmt.Errorf("no project returned for %v, skipping", *entry.RepoName)
	}

	tokenInfo, err := project.GatherProjectTokenInfo(gatherCtx, gitlabClient, info.ID)
	if err != nil {
		metrics.TokenRotations.WithLabelValues("project", entry.Name, store, "error").Inc()
		return vaultTokenID, fmt.Errorf(errorString, *entry.RepoName, index, err)
	}

	tokenQueue := matchingProjectTokens(tokenInfo, entry, yamlConfig.Prefix, index)
	metrics.ActiveTokens.WithLabelValues("project", entry.Name).Set(float64(len(tokenQueue)))

	if len(tokenQueue) < 1 {
		l.Info("no tokens found in repo, creating new token", zap.String("repo", *entry.RepoName))

		createCtx, cancel := context.WithTimeout(ctx, createTimeout)
		defer cancel()

		token, errTokenCreation := project.CreateNewProjectToken(createCtx, gitlabClient, info.ID, entry, yamlConfig.Prefix)
		if errTokenCreation != nil {
			metrics.TokenRotations.WithLabelValues("project", entry.Name, store, "error").Inc()
			return vaultTokenID, fmt.Errorf(errorString, *entry.RepoName, index, errTokenCreation)
		}
		projectToken = token
	}

	needsRenewal, err := project.CheckProjectTokensForRenewal(tokenQueue, entry, vaultTokenID)
	if err != nil {
		metrics.TokenRotations.WithLabelValues("project", entry.Name, store, "error").Inc()
		return vaultTokenID, fmt.Errorf(errorString, *entry.RepoName, index, err)
	}

	if needsRenewal {
		l.Info("token ready for renewal", zap.String("token_name", entry.Name), zap.String("repo", *entry.RepoName))

		createCtx, cancel := context.WithTimeout(ctx, createTimeout)
		defer cancel()

		token, errRenewal := project.RenewProjectAccessToken(createCtx, gitlabClient, info.ID, entry, yamlConfig.Prefix)
		if errRenewal != nil {
			metrics.TokenRotations.WithLabelValues("project", entry.Name, store, "error").Inc()
			return vaultTokenID, fmt.Errorf(errorString, entry.Name, index, errRenewal)
		}
		projectToken = token
	} else {
		l.Info("no tokens need renewal", zap.String("token_name", entry.Name), zap.String("repo", *entry.RepoName))
	}

	if projectToken == nil {
		return vaultTokenID, nil
	}

	// Vault write phase
	vaultCtx, cancel := context.WithTimeout(ctx, vaultWriteTimeout)
	defer cancel()

	meta := secrets.TokenMetadata{
		TokenID:   projectToken.ID,
		TokenName: projectToken.Name,
	}
	if projectToken.CreatedAt != nil {
		meta.CreatedAt = *projectToken.CreatedAt
	}

	if err := persistToken(vaultCtx, entry, secret, projectToken.Token, meta); err != nil {
		metrics.TokenRotations.WithLabelValues("project", entry.Name, store, "error").Inc()

		// Rollback phase: revoke newly created token so it does not become an orphan.
		rollbackCtx, cancel := context.WithTimeout(ctx, rollbackTimeout)
		defer cancel()

		metrics.TokenRollbackAttempts.WithLabelValues("project", entry.Name).Inc()
		if revokeErr := rollbackProjectToken(rollbackCtx, gitlabClient, info.ID, projectToken.ID); revokeErr != nil {
			metrics.TokenRollbackOutcomes.WithLabelValues("project", entry.Name, "failure").Inc()
			l.Error("rollback revoke failed after vault write failure; orphan token may exist",
				zap.String("token_name", projectToken.Name),
				zap.Int64("token_id", projectToken.ID),
				zap.Error(revokeErr),
			)
			return vaultTokenID, fmt.Errorf("vault write failed after token creation: %w; rollback revoke also failed: %v", err, revokeErr)
		}
		metrics.TokenRollbackOutcomes.WithLabelValues("project", entry.Name, "success").Inc()
		l.Info("rollback revoke succeeded after vault write failure",
			zap.String("token_name", projectToken.Name),
			zap.Int64("token_id", projectToken.ID),
		)
		return vaultTokenID, fmt.Errorf("vault write failed after token creation; new token revoked: %w", err)
	}

	metrics.TokenRotations.WithLabelValues("project", entry.Name, store, "success").Inc()
	metrics.TokenRotationDuration.WithLabelValues("project", entry.Name).Observe(time.Since(start).Seconds())

	return projectToken.ID, nil
}
