package project

import (
	"github.com/nabsku/token-tumbler/internal/types/repository"
	"gitlab.com/gitlab-org/api/client-go"
)

func CheckProjectTokensForRenewal(tokens []*gitlab.ProjectAccessToken, entry *repository.Repository, vaultTokenID int64) (bool, error) {
	needsRenewalCount := 0
	activeTokenCount := 0
	prefilterOnVaultToken := preserveProjectVaultToken(tokens, vaultTokenID)

	for _, token := range tokens {
		if token.Revoked || !token.Active {
			continue
		}
		if prefilterOnVaultToken && token.ID != vaultTokenID {
			continue
		}
		activeTokenCount++

		needsRenewal, err := entry.ShouldBeRenewed(token)
		if err != nil {
			return false, err
		}

		if needsRenewal {
			needsRenewalCount++
		}
	}

	return activeTokenCount > 0 && needsRenewalCount == activeTokenCount, nil
}

func preserveProjectVaultToken(tokens []*gitlab.ProjectAccessToken, vaultTokenID int64) bool {
	if vaultTokenID <= 0 {
		return false
	}
	for _, token := range tokens {
		if token == nil {
			continue
		}
		if token.ID == vaultTokenID && token.Active && !token.Revoked {
			return true
		}
	}
	return false
}
