package group

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

func DeleteGroupTokens(ctx context.Context, gitlabClient *gitlab.Client, repo *repository.Repository, prefix string, vaultTokenID int64) error {
	l := logger.GetLogger()

	l.Debug("checking for old tokens", zap.String("group", *repo.GroupName), zap.Int64("vault_token_id", vaultTokenID))

	group, err := GatherGroup(ctx, gitlabClient, repo)
	if err != nil {
		return err
	}

	tokens, err := GatherGroupTokenInfoByPrefix(ctx, gitlabClient, group.ID, prefix, *repo)
	if err != nil {
		return err
	}
	l.Debug("found matching active tokens", zap.Int("count", len(tokens)), zap.String("group", *repo.GroupName))

	if len(tokens) <= 1 {
		l.Debug("only one token found, not revoking", zap.String("group", *repo.GroupName))
		return nil
	}

	preserveToken := resolvePreserveToken(tokens, vaultTokenID, l, *repo.GroupName)
	if preserveToken == nil {
		l.Debug("no dated token found, not revoking", zap.String("group", *repo.GroupName))
		return nil
	}

	detectGroupOrphans(tokens, preserveToken, vaultTokenID, l, repo.Name)

	var revokeErr error
	now := repo.CurrentTime()
	for _, token := range tokens {
		if token.Revoked || !token.Active {
			continue
		}
		if parseOk, errTokenParse := repo.ParseTokenName(prefix, token.Name); parseOk {
			l.Debug("checking token for deletion", zap.String("token_name", token.Name), zap.Int64("token_id", token.ID))
			shouldDelete := checkGroupTokenDeletionAt(repo, token, preserveToken, now)

			if shouldDelete {
				l.Debug("deleting token", zap.String("token_name", token.Name), zap.String("group", *repo.GroupName))
				_, err := gitlabClient.GroupAccessTokens.RevokeGroupAccessToken(group.ID, token.ID, gitlab.WithContext(ctx))
				if err != nil {
					l.Error(fmt.Errorf("error deleting token %s: %v", token.Name, err).Error())
					revokeErr = errors.Join(revokeErr, fmt.Errorf("deleting token %s from group %s: %w", token.Name, *repo.GroupName, err))
				} else {
					l.Info("deleted token", zap.String("token_name", token.Name), zap.String("group", *repo.GroupName))
				}
			}
		} else if errTokenParse != nil {
			l.Error(fmt.Errorf("error parsing token name for %s: %v", token.Name, errTokenParse).Error())
		}
	}
	return revokeErr
}

func resolvePreserveToken(tokens []*gitlab.GroupAccessToken, vaultTokenID int64, l *zap.Logger, groupName string) *gitlab.GroupAccessToken {
	if vaultTokenID > 0 {
		for _, token := range tokens {
			if token.ID == vaultTokenID {
				l.Debug("preserving Vault-stored token", zap.String("token_name", token.Name), zap.Int64("token_id", token.ID), zap.String("group", groupName))
				return token
			}
		}
		l.Warn("Vault-stored token ID not found in GitLab; falling back to newest token", zap.Int64("vault_token_id", vaultTokenID), zap.String("group", groupName))
	}

	var newestToken *gitlab.GroupAccessToken
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

func detectGroupOrphans(tokens []*gitlab.GroupAccessToken, preserveToken *gitlab.GroupAccessToken, vaultTokenID int64, l *zap.Logger, groupName string) {
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
				zap.String("group", groupName),
			)
			metrics.OrphanTokensDetected.WithLabelValues("group", groupName).Inc()
		}
	}
}

func checkGroupTokenDeletion(entry *repository.Repository, token *gitlab.GroupAccessToken, preserveToken *gitlab.GroupAccessToken) bool {
	return checkGroupTokenDeletionAt(entry, token, preserveToken, entry.CurrentTime())
}

func checkGroupTokenDeletionAt(entry *repository.Repository, token *gitlab.GroupAccessToken, preserveToken *gitlab.GroupAccessToken, now time.Time) bool {
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
	if token.CreatedAt.After(*preserveToken.CreatedAt) {
		l.Debug("Token is newer than the preserved token, not deleting")
		return false
	}

	l.Debug("checking if token is older than grace period", zap.String("token_name", token.Name), zap.Int64("token_id", token.ID))

	if now.After(preserveToken.CreatedAt.Add(entry.GracePeriod.ToDuration())) {
		return true
	}

	return false
}
