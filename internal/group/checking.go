package group

import (
	"github.com/nabsku/token-chaser/internal/types/repository"
	"gitlab.com/gitlab-org/api/client-go"
)

func CheckGroupTokensForRenewal(tokens []*gitlab.GroupAccessToken, entry *repository.Repository) (bool, error) {
	needsRenewalCount := 0
	activeTokenCount := 0
	for _, token := range tokens {
		if token.Revoked || !token.Active {
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
