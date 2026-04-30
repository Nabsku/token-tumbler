package group

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nabsku/token-tumbler/internal/types/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/gitlab-org/api/client-go"
)

func TestGatherGroup(t *testing.T) {
	tests := []struct {
		name      string
		response  string
		status    int
		wantID    int64
		wantError error
	}{
		{name: "single group", status: http.StatusOK, response: `{"id":42,"name":"platform","full_path":"org/platform"}`, wantID: 42},
		{name: "no group", status: http.StatusNotFound, response: `{"message":"404 Group Not Found"}`, wantError: ErrNoGroupsInSearch},
		{name: "api error", status: http.StatusInternalServerError, response: `{"message":"boom"}`, wantError: assert.AnError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newGroupTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodGet, r.Method)
				assert.Equal(t, "/api/v4/groups/platform", r.URL.Path)
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.response))
			})
			groupName := "platform"

			got, err := GatherGroup(context.Background(), client, &repository.Repository{GroupName: &groupName})

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
			assert.Equal(t, "100", r.URL.Query().Get("per_page"))
			_, _ = w.Write([]byte(`[{"id":1,"name":"tt-platform-old"},{"id":2,"name":"tt-platform-new"}]`))
		})

		got, err := GatherGroupTokenInfo(context.Background(), client, 42)

		require.NoError(t, err)
		require.Len(t, got, 2)
		assert.Equal(t, int64(1), got[0].ID)
		assert.Equal(t, "tt-platform-old", got[0].Name)
	})

	t.Run("returns all paginated group access tokens", func(t *testing.T) {
		client := newGroupTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Equal(t, "/api/v4/groups/42/access_tokens", r.URL.Path)
			assert.Equal(t, "100", r.URL.Query().Get("per_page"))

			switch r.URL.Query().Get("page") {
			case "", "1":
				w.Header().Set("X-Next-Page", "2")
				_, _ = w.Write([]byte(`[{"id":1,"name":"tt-platform-old"}]`))
			case "2":
				_, _ = w.Write([]byte(`[{"id":2,"name":"tt-platform-new"}]`))
			default:
				t.Errorf("unexpected page %q", r.URL.Query().Get("page"))
				http.Error(w, "unexpected page", http.StatusBadRequest)
			}
		})

		got, err := GatherGroupTokenInfo(context.Background(), client, 42)

		require.NoError(t, err)
		require.Len(t, got, 2)
		assert.Equal(t, []int64{1, 2}, []int64{got[0].ID, got[1].ID})
	})

	t.Run("propagates api error", func(t *testing.T) {
		client := newGroupTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"message":"boom"}`))
		})

		got, err := GatherGroupTokenInfo(context.Background(), client, 42)

		require.Error(t, err)
		assert.Nil(t, got)
	})
}

func TestGatherGroupTokenInfoByPrefix(t *testing.T) {
	client := newGroupTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/api/v4/groups/42/access_tokens", r.URL.Path)
		_, _ = w.Write([]byte(`[
			{"id":1,"name":"tt-platform-old","active":true},
			{"id":2,"name":"foreign-token","active":true},
			{"id":3,"name":"tt-platform-new","active":true},
			{"id":4,"name":"tt-platform-revoked","active":false,"revoked":true},
			{"id":5,"name":"tt-platform-inactive","active":false}
		]`))
	})

	got, err := GatherGroupTokenInfoByPrefix(context.Background(), client, 42, "tt", repository.Repository{Name: "platform"})

	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, []int64{1, 3}, []int64{got[0].ID, got[1].ID})
}

func newGroupTestClient(t *testing.T, handler http.HandlerFunc) *gitlab.Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	client, err := gitlab.NewClient("token", gitlab.WithBaseURL(server.URL+"/api/v4"))
	require.NoError(t, err)
	return client
}
