package group

import (
	"github.com/nabsku/token-tumbler/internal/types/repository"
	"gitlab.com/gitlab-org/api/client-go"
)

func CheckGroupTokensForRenewal(tokens []*gitlab.GroupAccessToken, entry *repository.Repository, vaultTokenID int64) (bool, error) {
	needsRenewalCount := 0
	activeTokenCount := 0
	prefilterOnVaultToken := preserveGroupVaultToken(tokens, vaultTokenID)

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

func preserveGroupVaultToken(tokens []*gitlab.GroupAccessToken, vaultTokenID int64) bool {
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
