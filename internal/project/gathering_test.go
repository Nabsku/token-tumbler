package project

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

func TestGatherProject(t *testing.T) {
	tests := []struct {
		name      string
		response  string
		status    int
		wantID    int
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

			got, err := GatherProject(client, &repository.Repository{RepoName: &repoName})

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
			_, _ = w.Write([]byte(`[{"id":1,"name":"tc-service-old"},{"id":2,"name":"tc-service-new"}]`))
		})

		got, err := GatherProjectTokenInfo(client, 42)

		require.NoError(t, err)
		require.Len(t, got, 2)
		assert.Equal(t, 1, got[0].ID)
		assert.Equal(t, "tc-service-old", got[0].Name)
	})

	t.Run("returns all paginated project access tokens", func(t *testing.T) {
		client := newProjectTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Equal(t, "/api/v4/projects/42/access_tokens", r.URL.Path)
			assert.Equal(t, "100", r.URL.Query().Get("per_page"))

			switch r.URL.Query().Get("page") {
			case "", "1":
				w.Header().Set("X-Next-Page", "2")
				_, _ = w.Write([]byte(`[{"id":1,"name":"tc-service-old"}]`))
			case "2":
				_, _ = w.Write([]byte(`[{"id":2,"name":"tc-service-new"}]`))
			default:
				t.Errorf("unexpected page %q", r.URL.Query().Get("page"))
				http.Error(w, "unexpected page", http.StatusBadRequest)
			}
		})

		got, err := GatherProjectTokenInfo(client, 42)

		require.NoError(t, err)
		require.Len(t, got, 2)
		assert.Equal(t, []int{1, 2}, []int{got[0].ID, got[1].ID})
	})

	t.Run("propagates api error", func(t *testing.T) {
		client := newProjectTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"message":"boom"}`))
		})

		got, err := GatherProjectTokenInfo(client, 42)

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
