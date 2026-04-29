package project

import (
	"github.com/nabsku/token-chaser/internal/types/repository"
	"gitlab.com/gitlab-org/api/client-go"
)

func CheckProjectTokensForRenewal(tokens []*gitlab.ProjectAccessToken, entry *repository.Repository) (bool, error) {
	needsRenewalCount := 0
	for _, token := range tokens {
		needsRenewal, err := entry.ShouldBeRenewed(token)
		if err != nil {
			return false, err
		}

		if needsRenewal {
			needsRenewalCount++
		}
	}

	return len(tokens) > 0 && needsRenewalCount == len(tokens), nil
}
