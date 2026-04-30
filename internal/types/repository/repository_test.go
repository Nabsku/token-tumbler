package repository

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/gitlab-org/api/client-go"
)

func TestDuration_UnmarshalYAML_ShouldParseSupportedUnits(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  time.Duration
	}{
		{name: "seconds", input: "5s", want: 5 * time.Second},
		{name: "minutes", input: "5m", want: 5 * time.Minute},
		{name: "hours", input: "5h", want: 5 * time.Hour},
		{name: "days", input: "5d", want: 5 * 24 * time.Hour},
		{name: "weeks", input: "5w", want: 5 * 7 * 24 * time.Hour},
		{name: "months", input: "5M", want: 5 * 30 * 24 * time.Hour},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got Duration

			err := got.UnmarshalYAML(func(v interface{}) error {
				out := v.(*string)
				*out = tt.input
				return nil
			})

			require.NoError(t, err)
			assert.Equal(t, tt.want, got.ToDuration())
		})
	}
}

func TestDuration_UnmarshalYAML_ShouldRejectInvalidDurations(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{name: "non numeric value", input: "soon", wantErr: ErrMissingDuration.Error()},
		{name: "empty value", input: "", wantErr: ErrMissingDuration.Error()},
		{name: "invalid unit", input: "10x", wantErr: "invalid duration unit"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got Duration

			err := got.UnmarshalYAML(func(v interface{}) error {
				out := v.(*string)
				*out = tt.input
				return nil
			})

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestConfig_ValidatePrefix(t *testing.T) {
	tests := []struct {
		name    string
		prefix  string
		wantErr error
	}{
		{name: "letters", prefix: "token", wantErr: nil},
		{name: "letters numbers hyphen underscore", prefix: "token-123_test", wantErr: nil},
		{name: "empty", prefix: "", wantErr: ErrInvalidPrefix},
		{name: "slash", prefix: "token/test", wantErr: ErrInvalidPrefix},
		{name: "dot", prefix: "token.test", wantErr: ErrInvalidPrefix},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := (&Config{Prefix: tt.prefix}).ValidatePrefix()

			if tt.wantErr == nil {
				require.NoError(t, err)
				return
			}
			assert.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestConfig_Validate(t *testing.T) {
	t.Run("accepts valid project config with vault", func(t *testing.T) {
		cfg := &Config{Prefix: "tt", Repos: []Repository{validRepositoryConfig()}}

		err := cfg.Validate()

		require.NoError(t, err)
	})

	t.Run("rejects empty repositories", func(t *testing.T) {
		err := (&Config{Prefix: "tt"}).Validate()

		require.ErrorIs(t, err, ErrInvalidRepositoryConfig)
		assert.Contains(t, err.Error(), "repositories cannot be empty")
	})

	t.Run("rejects missing target", func(t *testing.T) {
		repo := validRepositoryConfig()
		repo.RepoName = nil

		err := (&Config{Prefix: "tt", Repos: []Repository{repo}}).Validate()

		require.ErrorIs(t, err, ErrInvalidRepositoryConfig)
		assert.Contains(t, err.Error(), "repoName or groupName is required")
	})

	t.Run("rejects both project and group target", func(t *testing.T) {
		repo := validRepositoryConfig()
		repo.GroupName = gitlab.Ptr("group")

		err := (&Config{Prefix: "tt", Repos: []Repository{repo}}).Validate()

		require.ErrorIs(t, err, ErrInvalidRepositoryConfig)
		assert.Contains(t, err.Error(), "define either repoName or groupName")
	})

	t.Run("rejects incomplete vault config", func(t *testing.T) {
		repo := validRepositoryConfig()
		repo.VaultKey = nil

		err := (&Config{Prefix: "tt", Repos: []Repository{repo}}).Validate()

		require.ErrorIs(t, err, ErrInvalidRepositoryConfig)
		assert.Contains(t, err.Error(), "vaultKey is required")
	})

	t.Run("requires explicit secret store", func(t *testing.T) {
		repo := validRepositoryConfig()
		repo.SecretStore = ""

		err := (&Config{Prefix: "tt", Repos: []Repository{repo}}).Validate()

		require.ErrorIs(t, err, ErrInvalidRepositoryConfig)
		assert.Contains(t, err.Error(), "secretStore is required")
	})

	t.Run("allows explicit no persistence mode", func(t *testing.T) {
		repo := validRepositoryConfig()
		repo.SecretStore = "none"
		repo.VaultPath = nil
		repo.VaultKey = nil
		repo.Mount = nil

		err := (&Config{Prefix: "tt", Repos: []Repository{repo}}).Validate()

		require.NoError(t, err)
	})

	t.Run("accepts valid file secret store config", func(t *testing.T) {
		repo := validRepositoryConfig()
		filePath := "/run/secrets/gitlab-token"
		repo.SecretStore = "file"
		repo.FilePath = &filePath
		repo.VaultPath = nil
		repo.VaultKey = nil
		repo.Mount = nil

		err := (&Config{Prefix: "tt", Repos: []Repository{repo}}).Validate()

		require.NoError(t, err)
	})

	t.Run("rejects file secret store without file path", func(t *testing.T) {
		repo := validRepositoryConfig()
		repo.SecretStore = "file"
		repo.VaultPath = nil
		repo.VaultKey = nil
		repo.Mount = nil

		err := (&Config{Prefix: "tt", Repos: []Repository{repo}}).Validate()

		require.ErrorIs(t, err, ErrInvalidRepositoryConfig)
		assert.Contains(t, err.Error(), "filePath is required")
	})

	t.Run("rejects non-positive lifetime", func(t *testing.T) {
		repo := validRepositoryConfig()
		repo.Lifetime = Duration{Duration: -1 * time.Hour}

		err := (&Config{Prefix: "tt", Repos: []Repository{repo}}).Validate()

		require.ErrorIs(t, err, ErrInvalidRepositoryConfig)
		assert.Contains(t, err.Error(), "lifetime must be greater than zero")
	})

	t.Run("rejects non-positive rotation threshold", func(t *testing.T) {
		repo := validRepositoryConfig()
		repo.RotationThreshold = &Duration{Duration: 0}

		err := (&Config{Prefix: "tt", Repos: []Repository{repo}}).Validate()

		require.ErrorIs(t, err, ErrInvalidRepositoryConfig)
		assert.Contains(t, err.Error(), "rotationThreshold must be greater than zero")
	})

	t.Run("rejects negative grace period", func(t *testing.T) {
		repo := validRepositoryConfig()
		repo.GracePeriod = &Duration{Duration: -1 * time.Hour}

		err := (&Config{Prefix: "tt", Repos: []Repository{repo}}).Validate()

		require.ErrorIs(t, err, ErrInvalidRepositoryConfig)
		assert.Contains(t, err.Error(), "gracePeriod cannot be negative")
	})

	t.Run("rejects duplicate project token target", func(t *testing.T) {
		first := validRepositoryConfig()
		second := validRepositoryConfig()

		err := (&Config{Prefix: "tt", Repos: []Repository{first, second}}).Validate()

		require.ErrorIs(t, err, ErrInvalidRepositoryConfig)
		assert.Contains(t, err.Error(), "duplicate token target")
	})

	t.Run("allows same token name on different target types", func(t *testing.T) {
		project := validRepositoryConfig()
		group := validRepositoryConfig()
		group.RepoName = nil
		group.GroupName = gitlab.Ptr("service")

		err := (&Config{Prefix: "tt", Repos: []Repository{project, group}}).Validate()

		require.NoError(t, err)
	})
}

func TestConfig_UsesVault(t *testing.T) {
	assert.False(t, (&Config{Repos: []Repository{{SecretStore: "none"}}}).UsesVault())
	assert.True(t, (&Config{Repos: []Repository{{SecretStore: " VaUlT "}}}).UsesVault())
}

func TestRepository_ParseTokenName(t *testing.T) {
	tests := []struct {
		name      string
		repoName  string
		prefix    string
		tokenName string
		wantOK    bool
	}{
		{name: "matches expected prefix repo format", repoName: "service", prefix: "tt", tokenName: "tt-service-2026-01-01T00:00:00Z", wantOK: true},
		{name: "rejects expected format as substring", repoName: "service", prefix: "tt", tokenName: "old-tt-service-2026", wantOK: false},
		{name: "rejects missing dash after prefix", repoName: "service", prefix: "tt", tokenName: "ttservice-2026", wantOK: false},
		{name: "rejects different repository name", repoName: "service", prefix: "tt", tokenName: "tt-other-2026", wantOK: false},
		{name: "trailing dash prefix still matches single dash format", repoName: "service", prefix: "tt-", tokenName: "tt-service-2026", wantOK: true},
		{name: "trailing dash prefix rejects double dash format", repoName: "service", prefix: "tt-", tokenName: "tt--service-2026", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &Repository{Name: tt.repoName}

			got, err := repo.ParseTokenName(tt.prefix, tt.tokenName)

			assert.Equal(t, tt.wantOK, got)
			if tt.wantOK {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestRepository_ParseTokenName_ShouldRecognizeGeneratedTokenNames(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
	}{
		{name: "prefix without trailing dash", prefix: "tt"},
		{name: "prefix with trailing dash", prefix: "tt-"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &Repository{Name: "service"}
			tokenName, err := repo.NewTokenName(tt.prefix)
			require.NoError(t, err)

			got, err := repo.ParseTokenName(tt.prefix, tokenName)

			require.NoError(t, err)
			assert.True(t, got)
		})
	}
}

func TestRepository_GetExpiryDate(t *testing.T) {
	repo := &Repository{Lifetime: Duration{Duration: 72 * time.Hour}}
	before := time.Now().Add(72 * time.Hour)

	expiry, err := repo.GetExpiryDate()
	after := time.Now().Add(72 * time.Hour)

	require.NoError(t, err)
	require.NotNil(t, expiry)
	assert.False(t, expiry.Before(before.Add(-time.Second)), "expiry %s should be near now+lifetime", expiry)
	assert.False(t, expiry.After(after.Add(time.Second)), "expiry %s should be near now+lifetime", expiry)
}

func TestRepository_NewTokenName(t *testing.T) {
	t.Run("builds name with RFC3339 timestamp", func(t *testing.T) {
		repo := &Repository{Name: "service"}

		name, err := repo.NewTokenName("tt-")

		require.NoError(t, err)
		require.True(t, strings.HasPrefix(name, "tt-service-"))
		_, err = time.Parse(time.RFC3339, strings.TrimPrefix(name, "tt-service-"))
		require.NoError(t, err)
	})

	t.Run("normalizes prefix without trailing dash", func(t *testing.T) {
		repo := &Repository{Name: "service"}

		name, err := repo.NewTokenName("tt")

		require.NoError(t, err)
		assert.True(t, strings.HasPrefix(name, "tt-service-"))
	})

	t.Run("rejects empty prefix", func(t *testing.T) {
		name, err := (&Repository{Name: "service"}).NewTokenName("")

		assert.Empty(t, name)
		assert.ErrorIs(t, err, ErrInvalidPrefix)
	})

	t.Run("rejects empty token name", func(t *testing.T) {
		name, err := (&Repository{}).NewTokenName("tt-")

		assert.Empty(t, name)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be empty")
	})
}

func TestParseISOTime(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "date only", input: "2026-01-13", want: "2026-01-13T00:00:00Z"},
		{name: "rfc3339", input: "2026-01-13T12:30:00Z", want: "2026-01-13T12:30:00Z"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseISOTime(tt.input)

			require.NoError(t, err)
			assert.Equal(t, tt.want, got.UTC().Format(time.RFC3339))
		})
	}
}

func TestRepository_CheckKeyRotationAndTokenAge(t *testing.T) {
	tests := []struct {
		name     string
		lifetime time.Duration
		rotation time.Duration
		wantErr  error
	}{
		{name: "valid when lifetime is greater", lifetime: 30 * 24 * time.Hour, rotation: 7 * 24 * time.Hour, wantErr: nil},
		{name: "rejects equal lifetime and rotation", lifetime: 7 * 24 * time.Hour, rotation: 7 * 24 * time.Hour, wantErr: ErrKeyAgeRotationSame},
		{name: "rejects lifetime lower than rotation", lifetime: 24 * time.Hour, rotation: 7 * 24 * time.Hour, wantErr: ErrTokenAgeLowerThanKeyRotation},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &Repository{Lifetime: Duration{Duration: tt.lifetime}, RotationThreshold: &Duration{Duration: tt.rotation}}

			err := repo.CheckKeyRotationAndTokenAge()

			if tt.wantErr == nil {
				require.NoError(t, err)
				return
			}
			assert.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestRepository_ShouldBeRenewed(t *testing.T) {
	repo := &Repository{RotationThreshold: &Duration{Duration: 48 * time.Hour}}

	tests := []struct {
		name    string
		token   any
		want    bool
		wantErr error
	}{
		{name: "project token within threshold", token: projectTokenExpiring(t, time.Now().Add(24*time.Hour)), want: true},
		{name: "project token after threshold", token: projectTokenExpiring(t, time.Now().Add(10*24*time.Hour)), want: false},
		{name: "group token within threshold", token: groupTokenExpiring(t, time.Now().Add(24*time.Hour)), want: true},
		{name: "group token after threshold", token: groupTokenExpiring(t, time.Now().Add(10*24*time.Hour)), want: false},
		{name: "personal token unsupported", token: &gitlab.PersonalAccessToken{}, wantErr: ErrInvalidTokenType},
		{name: "unknown token type", token: struct{}{}, wantErr: ErrInvalidTokenType},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := repo.ShouldBeRenewed(tt.token)

			if tt.wantErr != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tt.wantErr), "got %v, want errors.Is(_, %v)", err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func mustISODate(t *testing.T, value time.Time) *gitlab.ISOTime {
	t.Helper()
	iso, err := gitlab.ParseISOTime(value.Format(time.DateOnly))
	require.NoError(t, err)
	return &iso
}

func projectTokenExpiring(t *testing.T, value time.Time) *gitlab.ProjectAccessToken {
	t.Helper()
	token := &gitlab.ProjectAccessToken{}
	token.ExpiresAt = mustISODate(t, value)
	return token
}

func groupTokenExpiring(t *testing.T, value time.Time) *gitlab.GroupAccessToken {
	t.Helper()
	token := &gitlab.GroupAccessToken{}
	token.ExpiresAt = mustISODate(t, value)
	return token
}

func validRepositoryConfig() Repository {
	return Repository{
		RepoName:          gitlab.Ptr("service"),
		Name:              "token",
		Permissions:       []string{"api"},
		RotationThreshold: &Duration{Duration: 24 * time.Hour},
		GracePeriod:       &Duration{Duration: 24 * time.Hour},
		Lifetime:          Duration{Duration: 48 * time.Hour},
		SecretStore:       "vault",
		VaultPath:         gitlab.Ptr("path"),
		VaultKey:          gitlab.Ptr("key"),
		Mount:             gitlab.Ptr("kv"),
	}
}
