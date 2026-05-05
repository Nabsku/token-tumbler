package gitlabutil

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"gitlab.com/gitlab-org/api/client-go"
)

func TestCollectPages_RejectsNonAdvancingPage(t *testing.T) {
	call := 0

	_, err := CollectPages(
		context.Background(),
		func() ([]string, *gitlab.Response, error) {
			call++
			switch call {
			case 1:
				return []string{"first"}, &gitlab.Response{NextPage: 2}, nil
			default:
				return []string{"second"}, &gitlab.Response{NextPage: 1}, nil
			}
		},
		func(page int64) {},
	)

	require.Error(t, err)
	require.ErrorIs(t, err, ErrPaginationInvalidPage)
	require.Equal(t, 2, call)
}

func TestCollectPages_RejectsLoopingPage(t *testing.T) {
	call := 0

	_, err := CollectPages(
		context.Background(),
		func() ([]string, *gitlab.Response, error) {
			call++
			if call == 1 {
				return []string{"first"}, &gitlab.Response{NextPage: 2}, nil
			}
			if call == 2 {
				return []string{"second"}, &gitlab.Response{NextPage: 3}, nil
			}
			return []string{"third"}, &gitlab.Response{NextPage: 2}, nil
		},
		func(page int64) {},
	)

	require.Error(t, err)
	require.ErrorIs(t, err, ErrPaginationInvalidPage)
	require.Equal(t, 3, call)
}

func TestCollectPages_RejectsTooManyPages(t *testing.T) {
	call := int64(0)

	_, err := CollectPages(
		context.Background(),
		func() ([]string, *gitlab.Response, error) {
			call++
			return []string{"token"}, &gitlab.Response{NextPage: call + 1}, nil
		},
		func(page int64) {},
	)

	require.Error(t, err)
	require.ErrorIs(t, err, ErrPaginationPageLimit)
	require.Equal(t, maxPagesPerList, call)
}

func TestCollectPages_RejectsNilSetPage(t *testing.T) {
	_, err := CollectPages(context.Background(), func() ([]string, *gitlab.Response, error) {
		return []string{"token"}, &gitlab.Response{NextPage: 2}, nil
	}, nil)

	require.Error(t, err)
	require.Equal(t, "setPage callback is required", err.Error())
}

func TestCollectPages_PropagatesFetchError(t *testing.T) {
	fetchErr := errors.New("boom")

	_, err := CollectPages(
		context.Background(),
		func() ([]string, *gitlab.Response, error) {
			return nil, nil, fetchErr
		},
		func(page int64) {},
	)

	require.ErrorIs(t, err, fetchErr)
}
