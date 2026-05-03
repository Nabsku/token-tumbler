package project

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

func TestGatherProject(t *testing.T) {
	tests := []struct {
		name      string
		response  string
		status    int
		wantID    int64
		wantError error
	}{
		{name: "single project", status: http.StatusOK, response: `{"id":42,"name":"service","path_with_namespace":"org/service"}`, wantID: 42},
		{name: "no project", status: http.StatusNotFound, response: `{"message":"404 Project Not Found"}`, wantError: ErrNoProjectsInSearch},
		{name: "api error", status: http.StatusInternalServerError, response: `{"message":"boom"}`, wantError: assert.AnError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newProjectTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodGet, r.Method)
				assert.Equal(t, "/api/v4/projects/service", r.URL.Path)
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.response))
			})
			repoName := "service"

			got, err := GatherProject(context.Background(), client, &repository.Repository{RepoName: &repoName})

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

func TestGatherProjectTokenInfo(t *testing.T) {
	t.Run("returns project access tokens", func(t *testing.T) {
		client := newProjectTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Equal(t, "/api/v4/projects/42/access_tokens", r.URL.Path)
			assert.Equal(t, "100", r.URL.Query().Get("per_page"))
			_, _ = w.Write([]byte(`[{"id":1,"name":"tt-service-2026-01-01T00:00:00Z"},{"id":2,"name":"tt-service-2026-01-02T00:00:00Z"}]`))
		})

		got, err := GatherProjectTokenInfo(context.Background(), client, 42)

		require.NoError(t, err)
		require.Len(t, got, 2)
		assert.Equal(t, int64(1), got[0].ID)
		assert.Equal(t, "tt-service-2026-01-01T00:00:00Z", got[0].Name)
	})

	t.Run("returns all paginated project access tokens", func(t *testing.T) {
		client := newProjectTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Equal(t, "/api/v4/projects/42/access_tokens", r.URL.Path)
			assert.Equal(t, "100", r.URL.Query().Get("per_page"))

			switch r.URL.Query().Get("page") {
			case "", "1":
				w.Header().Set("X-Next-Page", "2")
				_, _ = w.Write([]byte(`[{"id":1,"name":"tt-service-2026-01-01T00:00:00Z"}]`))
			case "2":
				_, _ = w.Write([]byte(`[{"id":2,"name":"tt-service-2026-01-02T00:00:00Z"}]`))
			default:
				t.Errorf("unexpected page %q", r.URL.Query().Get("page"))
				http.Error(w, "unexpected page", http.StatusBadRequest)
			}
		})

		got, err := GatherProjectTokenInfo(context.Background(), client, 42)

		require.NoError(t, err)
		require.Len(t, got, 2)
		assert.Equal(t, []int64{1, 2}, []int64{got[0].ID, got[1].ID})
	})

	t.Run("propagates api error", func(t *testing.T) {
		client := newProjectTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"message":"boom"}`))
		})

		got, err := GatherProjectTokenInfo(context.Background(), client, 42)

		require.Error(t, err)
		assert.Nil(t, got)
	})
}

func newProjectTestClient(t *testing.T, handler http.HandlerFunc) *gitlab.Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	client, err := gitlab.NewClient("token", gitlab.WithBaseURL(server.URL+"/api/v4"))
	require.NoError(t, err)
	return client
}
