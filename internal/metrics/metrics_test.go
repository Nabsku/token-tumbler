package metrics

import (
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTokenRotations(t *testing.T) {
	require.NoError(t, testutil.CollectAndCompare(TokenRotations, strings.NewReader("")))

	TokenRotations.WithLabelValues("project", "deploy", "vault", "success").Inc()
	TokenRotations.WithLabelValues("group", "api", "aws", "error").Inc()

	count := testutil.ToFloat64(TokenRotations.WithLabelValues("project", "deploy", "vault", "success"))
	assert.Equal(t, 1.0, count)

	count = testutil.ToFloat64(TokenRotations.WithLabelValues("group", "api", "aws", "error"))
	assert.Equal(t, 1.0, count)
}

func TestSecretStoreOperations(t *testing.T) {
	SecretStoreOperations.WithLabelValues("vault", "write", "success").Inc()
	SecretStoreOperations.WithLabelValues("vault", "write", "success").Inc()

	count := testutil.ToFloat64(SecretStoreOperations.WithLabelValues("vault", "write", "success"))
	assert.Equal(t, 2.0, count)
}

func TestActiveTokens(t *testing.T) {
	ActiveTokens.WithLabelValues("project", "deploy").Set(3)

	value := testutil.ToFloat64(ActiveTokens.WithLabelValues("project", "deploy"))
	assert.Equal(t, 3.0, value)
}
