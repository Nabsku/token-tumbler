package leaderelection

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigFromEnv(t *testing.T) {
	t.Run("disabled by default", func(t *testing.T) {
		cfg, err := ConfigFromEnv()

		require.NoError(t, err)
		assert.False(t, cfg.Enabled)
		assert.Equal(t, defaultLeaseName, cfg.LeaseName)
		assert.Equal(t, defaultLeaseDuration, cfg.LeaseDuration)
		assert.Equal(t, defaultRenewDeadline, cfg.RenewDeadline)
		assert.Equal(t, defaultRetryPeriod, cfg.RetryPeriod)
	})

	t.Run("enabled with required namespace", func(t *testing.T) {
		t.Setenv(EnabledEnvVar, "true")
		t.Setenv(NamespaceEnvVar, "default")
		t.Setenv(LeaseNameEnvVar, "token-tumbler-test")
		t.Setenv(IdentityEnvVar, "pod-1")
		t.Setenv(LeaseDurationEnvVar, "30s")
		t.Setenv(RenewDeadlineEnvVar, "20s")
		t.Setenv(RetryPeriodEnvVar, "5s")

		cfg, err := ConfigFromEnv()

		require.NoError(t, err)
		assert.True(t, cfg.Enabled)
		assert.Equal(t, "default", cfg.Namespace)
		assert.Equal(t, "token-tumbler-test", cfg.LeaseName)
		assert.Equal(t, "pod-1", cfg.Identity)
		assert.Equal(t, 30*time.Second, cfg.LeaseDuration)
		assert.Equal(t, 20*time.Second, cfg.RenewDeadline)
		assert.Equal(t, 5*time.Second, cfg.RetryPeriod)
	})

	t.Run("rejects missing namespace when enabled", func(t *testing.T) {
		t.Setenv(EnabledEnvVar, "true")

		_, err := ConfigFromEnv()

		require.Error(t, err)
		assert.Contains(t, err.Error(), NamespaceEnvVar)
	})

	t.Run("rejects invalid enabled value", func(t *testing.T) {
		t.Setenv(EnabledEnvVar, "maybe")

		_, err := ConfigFromEnv()

		require.Error(t, err)
		assert.Contains(t, err.Error(), EnabledEnvVar)
	})

	t.Run("rejects invalid lease duration ordering", func(t *testing.T) {
		t.Setenv(EnabledEnvVar, "true")
		t.Setenv(NamespaceEnvVar, "default")
		t.Setenv(LeaseDurationEnvVar, "10s")
		t.Setenv(RenewDeadlineEnvVar, "10s")

		_, err := ConfigFromEnv()

		require.Error(t, err)
		assert.Contains(t, err.Error(), LeaseDurationEnvVar)
	})

	t.Run("rejects invalid renew deadline ordering", func(t *testing.T) {
		t.Setenv(EnabledEnvVar, "true")
		t.Setenv(NamespaceEnvVar, "default")
		t.Setenv(RenewDeadlineEnvVar, "2s")
		t.Setenv(RetryPeriodEnvVar, "2s")

		_, err := ConfigFromEnv()

		require.Error(t, err)
		assert.Contains(t, err.Error(), RenewDeadlineEnvVar)
	})
}
