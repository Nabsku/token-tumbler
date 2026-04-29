package gitlabutil

import "gitlab.com/gitlab-org/api/client-go"

type PageFunc[T any] func() ([]T, *gitlab.Response, error)
type SetPageFunc func(page int)

func CollectPages[T any](fetch PageFunc[T], setPage SetPageFunc) ([]T, error) {
	var all []T
	for {
		pageItems, response, err := fetch()
		if err != nil {
			return nil, err
		}
		all = append(all, pageItems...)
		if response == nil || response.NextPage == 0 {
			break
		}
		setPage(response.NextPage)
	}
	return all, nil
}
