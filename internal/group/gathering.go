package group

import (
	"errors"
	"github.com/nabsku/token-chaser/internal/types/repository"

	"gitlab.com/gitlab-org/api/client-go"
)

var ErrTooManyGroupsInSearch = errors.New("there are too many groups in your query. please narrow the group down by including the full path")
var ErrNoGroupsInSearch = errors.New("no groups found in your query")

func GatherGroup(gitlabClient *gitlab.Client, entry *repository.Repository) (*gitlab.Group, error) {
	opts := &gitlab.ListGroupsOptions{
		Search: gitlab.Ptr(*entry.GroupName),
	}
	groups, _, err := gitlabClient.Groups.ListGroups(opts)
	if err != nil {
		return nil, err
	}

	if len(groups) > 1 {
		return nil, ErrTooManyGroupsInSearch
	}
	if len(groups) == 0 {
		return nil, ErrNoGroupsInSearch
	}

	return groups[0], nil
}

func GatherGroupTokenInfo(gitlabClient *gitlab.Client, groupID int) ([]*gitlab.GroupAccessToken, error) {
	groupTokens, _, err := gitlabClient.GroupAccessTokens.ListGroupAccessTokens(groupID, nil)
	if err != nil {
		return nil, err
	}

	return groupTokens, nil
}

func GatherGroupTokenInfoByPrefix(gitlabClient *gitlab.Client, groupID int, prefix string, entry repository.Repository) ([]*gitlab.GroupAccessToken, error) {
	groupTokens, _, err := gitlabClient.GroupAccessTokens.ListGroupAccessTokens(groupID, nil)
	if err != nil {
		return nil, err
	}

	var prefixedTokens []*gitlab.GroupAccessToken

	for _, token := range groupTokens {
		if ok, _ := entry.ParseTokenName(prefix, token.Name); ok {
			prefixedTokens = append(prefixedTokens, token)
		}
	}

	return prefixedTokens, nil
}
