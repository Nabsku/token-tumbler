package group

import (
	"github.com/nabsku/token-chaser/internal/types/repository"
	"gitlab.com/gitlab-org/api/client-go"
)

func CheckGroupTokensForRenewal(tokens []*gitlab.GroupAccessToken, entry *repository.Repository) (bool, error) {
	var tokensToRenew []*gitlab.GroupAccessToken

	for _, token := range tokens {
		needsRenewal, err := entry.ShouldBeRenewed(token)
		if err != nil {
			return false, err
		}

		if needsRenewal {
			tokensToRenew = append(tokensToRenew, token)
		}
	}

	return len(tokens) > 0 && len(tokensToRenew) == len(tokens), nil
}
