package group

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nabsku/token-chaser/internal/types/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/gitlab-org/api/client-go"
)

func TestGatherGroup(t *testing.T) {
	tests := []struct {
		name      string
		response  string
		status    int
		wantID    int
		wantError error
	}{
		{name: "single group", status: http.StatusOK, response: `[{"id":42,"name":"platform","full_path":"org/platform"}]`, wantID: 42},
		{name: "too many groups", status: http.StatusOK, response: `[{"id":1},{"id":2}]`, wantError: ErrTooManyGroupsInSearch},
		{name: "no groups", status: http.StatusOK, response: `[]`, wantError: ErrNoGroupsInSearch},
		{name: "api error", status: http.StatusInternalServerError, response: `{"message":"boom"}`, wantError: assert.AnError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newGroupTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodGet, r.Method)
				assert.Equal(t, "/api/v4/groups", r.URL.Path)
				assert.Equal(t, "platform", r.URL.Query().Get("search"))
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.response))
			})
			groupName := "platform"

			got, err := GatherGroup(client, &repository.Repository{GroupName: &groupName})

			if tt.wantError != nil {
				require.Error(t, err)
				if !errors.Is(tt.wantError, assert.AnError) {
					assert.ErrorIs(t, err, tt.wantError)
				}
				return
			}
			require.NoError(t, err)
			require.NotNil(t, got)
			assert.Equal(t, tt.wantID, got.ID)
		})
	}
}

func TestGatherGroupTokenInfo(t *testing.T) {
	t.Run("returns group access tokens", func(t *testing.T) {
		client := newGroupTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Equal(t, "/api/v4/groups/42/access_tokens", r.URL.Path)
			_, _ = w.Write([]byte(`[{"id":1,"name":"tc-platform-old"},{"id":2,"name":"tc-platform-new"}]`))
		})

		got, err := GatherGroupTokenInfo(client, 42)

		require.NoError(t, err)
		require.Len(t, got, 2)
		assert.Equal(t, 1, got[0].ID)
		assert.Equal(t, "tc-platform-old", got[0].Name)
	})

	t.Run("propagates api error", func(t *testing.T) {
		client := newGroupTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"message":"boom"}`))
		})

		got, err := GatherGroupTokenInfo(client, 42)

		require.Error(t, err)
		assert.Nil(t, got)
	})
}

func TestGatherGroupTokenInfoByPrefix(t *testing.T) {
	client := newGroupTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/api/v4/groups/42/access_tokens", r.URL.Path)
		_, _ = w.Write([]byte(`[
			{"id":1,"name":"tc-platform-old"},
			{"id":2,"name":"foreign-token"},
			{"id":3,"name":"tc-platform-new"}
		]`))
	})

	got, err := GatherGroupTokenInfoByPrefix(client, 42, "tc", repository.Repository{Name: "platform"})

	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, []int{1, 3}, []int{got[0].ID, got[1].ID})
}

func newGroupTestClient(t *testing.T, handler http.HandlerFunc) *gitlab.Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	client, err := gitlab.NewClient("token", gitlab.WithBaseURL(server.URL+"/api/v4"))
	require.NoError(t, err)
	return client
}
