package project

import (
	"github.com/nabsku/token-chaser/internal/types/repository"
	"gitlab.com/gitlab-org/api/client-go"
)

func CheckProjectTokensForRenewal(tokens []*gitlab.ProjectAccessToken, entry *repository.Repository) (bool, error) {
	var tokensToRenew []*gitlab.ProjectAccessToken

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

//func checkProjectTokenRenewal(entry *repository.Repository, token *gitlab.ProjectAccessToken) (bool, error) {
//	shouldBeRenewed, err := entry.ShouldBeRenewed(token)
//	if err != nil {
//		return false, err
//	}
//
//	renewalDate, err := entry.GetRenewalDate(token.ExpiresAt)
//	if err != nil {
//		return false, err
//	}
//
//	currentTime := time.Now()
//
//	if currentTime.AddDate(0, 0, -*entry.RotationThreshold).After(token.CreatedAt.AddDate(0, 0, *entry.RotationThreshold)) {
//		return true, nil
//	}
//
//	if token.CreatedAt.AddDate(0, 0, *entry.Lifetime).Before(time.Now()) {
//		return true, nil
//	}
//
//	if time.Time(*token.ExpiresAt).AddDate(0, 0, -*entry.RotationThreshold).Before(currentTime) {
//		return true, nil
//	}
//
//	if time.Time(*token.ExpiresAt).Before(time.Time(renewalDate)) {
//		return true, nil
//	}
//
//	if token.CreatedAt.After(token.CreatedAt.AddDate(0, 0, *entry.Lifetime)) {
//		return true, nil
//	}
//
//	return false, nil
//}
