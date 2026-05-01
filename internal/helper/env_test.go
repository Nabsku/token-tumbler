package helper

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckAndGetEnvVar_ShouldReturnTrue(t *testing.T) {
	t.Setenv("TOKEN_TUMBLER_TEST_ENV", "present")

	test := CheckAndGetEnvVar("TOKEN_TUMBLER_TEST_ENV")

	assert.True(t, test)
}

func TestCheckAndGetEnvVar_ShouldReturnFalseForEmptyEnvValue(t *testing.T) {
	t.Setenv("TOKEN_TUMBLER_TEST_EMPTY_ENV", "")

	test := CheckAndGetEnvVar("TOKEN_TUMBLER_TEST_EMPTY_ENV")

	assert.False(t, test)
}

func TestCheckAndGetEnvVar_ShouldReturnFalse(t *testing.T) {
	test := CheckAndGetEnvVar("TOKEN_TUMBLER_TEST_ENV_THAT_SHOULD_NOT_EXIST")

	assert.False(t, test)
}

func TestCheckAndGetEnvVar_ShouldReturnFalseNoName(t *testing.T) {
	test := CheckAndGetEnvVar("")

	assert.False(t, test)
}

func TestCheckEnvVars(t *testing.T) {
	t.Run("returns nil when all variables exist", func(t *testing.T) {
		t.Setenv("TOKEN_TUMBLER_MAIN_TEST_A", "a")
		t.Setenv("TOKEN_TUMBLER_MAIN_TEST_B", "b")

		err := CheckEnvVars("TOKEN_TUMBLER_MAIN_TEST_A", "TOKEN_TUMBLER_MAIN_TEST_B")

		require.NoError(t, err)
	})

	t.Run("returns joined missing variables", func(t *testing.T) {
		err := CheckEnvVars("TOKEN_TUMBLER_MAIN_TEST_MISSING_A", "TOKEN_TUMBLER_MAIN_TEST_MISSING_B")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "TOKEN_TUMBLER_MAIN_TEST_MISSING_A, TOKEN_TUMBLER_MAIN_TEST_MISSING_B")
	})

	t.Run("treats empty variables as missing", func(t *testing.T) {
		t.Setenv("TOKEN_TUMBLER_MAIN_TEST_EMPTY", "")

		err := CheckEnvVars("TOKEN_TUMBLER_MAIN_TEST_EMPTY")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "TOKEN_TUMBLER_MAIN_TEST_EMPTY")
	})
}
