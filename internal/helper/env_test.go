package helper

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckAndGetEnvVar_ShouldReturnTrue(t *testing.T) {
	t.Setenv("TOKEN_CHASER_TEST_ENV", "present")

	test := CheckAndGetEnvVar("TOKEN_CHASER_TEST_ENV")

	assert.True(t, test)
}

func TestCheckAndGetEnvVar_ShouldReturnTrueForEmptyEnvValue(t *testing.T) {
	t.Setenv("TOKEN_CHASER_TEST_EMPTY_ENV", "")

	test := CheckAndGetEnvVar("TOKEN_CHASER_TEST_EMPTY_ENV")

	assert.True(t, test)
}

func TestCheckAndGetEnvVar_ShouldReturnFalse(t *testing.T) {
	test := CheckAndGetEnvVar("TOKEN_CHASER_TEST_ENV_THAT_SHOULD_NOT_EXIST")

	assert.False(t, test)
}

func TestCheckAndGetEnvVar_ShouldReturnFalseNoName(t *testing.T) {
	test := CheckAndGetEnvVar("")

	assert.False(t, test)
}
