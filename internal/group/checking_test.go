package group

import (
	"testing"
	"time"

	"github.com/nabsku/token-chaser/internal/types/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/gitlab-org/api/client-go"
)

func TestCheckGroupTokensForRenewal(t *testing.T) {
	entry := &repository.Repository{RotationThreshold: &repository.Duration{Duration: 48 * time.Hour}}

	tests := []struct {
		name   string
		tokens []*gitlab.GroupAccessToken
		want   bool
	}{
		{name: "empty token list", tokens: nil, want: false},
		{name: "all tokens need renewal", tokens: []*gitlab.GroupAccessToken{
			groupTokenWithExpiry(t, time.Now().Add(24*time.Hour)),
			groupTokenWithExpiry(t, time.Now().Add(36*time.Hour)),
		}, want: true},
		{name: "only some tokens need renewal", tokens: []*gitlab.GroupAccessToken{
			groupTokenWithExpiry(t, time.Now().Add(24*time.Hour)),
			groupTokenWithExpiry(t, time.Now().Add(10*24*time.Hour)),
		}, want: false},
		{name: "no tokens need renewal", tokens: []*gitlab.GroupAccessToken{
			groupTokenWithExpiry(t, time.Now().Add(10*24*time.Hour)),
			groupTokenWithExpiry(t, time.Now().Add(11*24*time.Hour)),
		}, want: false},
		{name: "ignores revoked and inactive tokens", tokens: []*gitlab.GroupAccessToken{
			groupTokenWithExpiry(t, time.Now().Add(24*time.Hour)),
			revokedGroupTokenWithExpiry(t, time.Now().Add(24*time.Hour)),
			inactiveGroupTokenWithExpiry(t, time.Now().Add(24*time.Hour)),
		}, want: true},
		{name: "only revoked or inactive tokens do not trigger renewal", tokens: []*gitlab.GroupAccessToken{
			revokedGroupTokenWithExpiry(t, time.Now().Add(24*time.Hour)),
			inactiveGroupTokenWithExpiry(t, time.Now().Add(24*time.Hour)),
		}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CheckGroupTokensForRenewal(tt.tokens, entry)

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func groupTokenWithExpiry(t *testing.T, expiry time.Time) *gitlab.GroupAccessToken {
	t.Helper()
	iso, err := gitlab.ParseISOTime(expiry.Format(time.DateOnly))
	require.NoError(t, err)
	token := &gitlab.GroupAccessToken{}
	token.ExpiresAt = &iso
	token.Active = true
	return token
}

func revokedGroupTokenWithExpiry(t *testing.T, expiry time.Time) *gitlab.GroupAccessToken {
	t.Helper()
	token := groupTokenWithExpiry(t, expiry)
	token.Revoked = true
	token.Active = false
	return token
}

func inactiveGroupTokenWithExpiry(t *testing.T, expiry time.Time) *gitlab.GroupAccessToken {
	t.Helper()
	token := groupTokenWithExpiry(t, expiry)
	token.Active = false
	return token
}
