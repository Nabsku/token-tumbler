package project

import (
	"context"
	"errors"
	"fmt"
	"github.com/nabsku/token-tumbler/internal/logger"
	"github.com/nabsku/token-tumbler/internal/metrics"
	"github.com/nabsku/token-tumbler/internal/types/repository"
	"time"

	"gitlab.com/gitlab-org/api/client-go"
	"go.uber.org/zap"
)

func DeleteProjectTokens(ctx context.Context, gitlabClient *gitlab.Client, repo *repository.Repository, prefix string, vaultTokenID int64) error {
	l := logger.GetLogger()

	l.Info("checking for old tokens", zap.String("repo", *repo.RepoName), zap.Int64("vault_token_id", vaultTokenID))

	project, err := GatherProject(ctx, gitlabClient, repo)
	if err != nil {
		l.Error(fmt.Errorf("error fetching project %s: %v", *repo.RepoName, err).Error())
		return err
	}
	if project == nil {
		return fmt.Errorf("no projects found for %s", *repo.RepoName)
	}

	tokens, err := GatherProjectTokenInfo(ctx, gitlabClient, project.ID)
	if err != nil {
		l.Error(fmt.Errorf("error fetching project tokens for %s: %v", *repo.RepoName, err).Error())
		return err
	}

	var prefixedTokens []*gitlab.ProjectAccessToken
	for _, token := range tokens {
		if token.Revoked || !token.Active {
			continue
		}
		if parseOk, errTokenParse := repo.ParseTokenName(prefix, token.Name); parseOk {
			prefixedTokens = append(prefixedTokens, token)
		} else if errTokenParse != nil {
			l.Debug(fmt.Errorf("error parsing token name for %s: %v", token.Name, errTokenParse).Error())
		}
	}

	if len(prefixedTokens) <= 1 {
		l.Debug("only one token found, not revoking", zap.String("repo", *repo.RepoName))
		return nil
	}

	// Determine which token to preserve: Vault-aware if metadata exists, else newest
	preserveToken := resolvePreserveToken(prefixedTokens, vaultTokenID, l, *repo.RepoName)
	if preserveToken == nil {
		l.Debug("no dated token found, not revoking", zap.String("repo", *repo.RepoName))
		return nil
	}

	detectProjectOrphans(prefixedTokens, preserveToken, vaultTokenID, l, repo.Name)

	var revokeErr error
	now := repo.CurrentTime()
	for _, token := range prefixedTokens {
		l.Debug("checking token for deletion", zap.String("token_name", token.Name), zap.Int64("token_id", token.ID))
		shouldDelete := checkProjectTokenDeletionAt(repo, token, preserveToken, now)

		if shouldDelete {
			l.Debug("deleting token", zap.String("token_name", token.Name), zap.String("repo", *repo.RepoName))
			_, err := gitlabClient.ProjectAccessTokens.RevokeProjectAccessToken(project.ID, token.ID, gitlab.WithContext(ctx))
			if err != nil {
				l.Error(fmt.Errorf("error deleting token %s: %v", token.Name, err).Error())
				revokeErr = errors.Join(revokeErr, fmt.Errorf("deleting token %s from project %s: %w", token.Name, *repo.RepoName, err))
			} else {
				l.Info("deleted token", zap.String("token_name", token.Name), zap.String("repo", *repo.RepoName))
			}
		}
	}
	return revokeErr
}

func resolvePreserveToken(tokens []*gitlab.ProjectAccessToken, vaultTokenID int64, l *zap.Logger, repoName string) *gitlab.ProjectAccessToken {
	if vaultTokenID > 0 {
		for _, token := range tokens {
			if token.ID == vaultTokenID {
				l.Debug("preserving Vault-stored token", zap.String("token_name", token.Name), zap.Int64("token_id", token.ID), zap.String("repo", repoName))
				return token
			}
		}
		// Vault token ID not found in GitLab — could be an orphan or stale metadata
		l.Warn("Vault-stored token ID not found in GitLab; falling back to newest token", zap.Int64("vault_token_id", vaultTokenID), zap.String("repo", repoName))
	}

	var newestToken *gitlab.ProjectAccessToken
	for _, token := range tokens {
		if token.CreatedAt == nil {
			l.Debug("token has no creation date, skipping as newest candidate", zap.String("token_name", token.Name))
			continue
		}
		if newestToken == nil || token.CreatedAt.After(*newestToken.CreatedAt) {
			newestToken = token
		}
	}
	return newestToken
}

func detectProjectOrphans(tokens []*gitlab.ProjectAccessToken, preserveToken *gitlab.ProjectAccessToken, vaultTokenID int64, l *zap.Logger, repoName string) {
	if vaultTokenID <= 0 || preserveToken == nil {
		return
	}
	for _, token := range tokens {
		if token.ID == preserveToken.ID {
			continue
		}
		if token.CreatedAt == nil || preserveToken.CreatedAt == nil {
			continue
		}
		if token.CreatedAt.After(*preserveToken.CreatedAt) {
			l.Warn("orphan token detected: newer token exists than the vault-stored token",
				zap.String("token_name", token.Name),
				zap.Int64("token_id", token.ID),
				zap.String("repo", repoName),
			)
			metrics.OrphanTokensDetected.WithLabelValues("project", repoName).Inc()
		}
	}
}

func checkProjectTokenDeletion(entry *repository.Repository, token *gitlab.ProjectAccessToken, preserveToken *gitlab.ProjectAccessToken) bool {
	return checkProjectTokenDeletionAt(entry, token, preserveToken, entry.CurrentTime())
}

func checkProjectTokenDeletionAt(entry *repository.Repository, token *gitlab.ProjectAccessToken, preserveToken *gitlab.ProjectAccessToken, now time.Time) bool {
	l := logger.GetLogger()

	l.Debug("checking token for deletion", zap.String("token_name", token.Name), zap.Int64("token_id", token.ID))
	if token.CreatedAt == nil || preserveToken == nil || preserveToken.CreatedAt == nil || entry.GracePeriod == nil {
		l.Debug("missing token creation date or grace period, not deleting")
		return false
	}
	l.Debug("token creation date", zap.String("token_name", token.Name), zap.Int64("token_id", token.ID), zap.Time("created_at", *token.CreatedAt))
	l.Debug("preserve token date", zap.Time("created_at", *preserveToken.CreatedAt))

	if token.ID == preserveToken.ID {
		l.Debug("Token is the preserved token, not deleting")
		return false
	}
	l.Debug("checking if token is older than grace period", zap.String("token_name", token.Name), zap.Int64("token_id", token.ID))

	if now.After(preserveToken.CreatedAt.Add(entry.GracePeriod.ToDuration())) {
		return true
	}

	return false
}
