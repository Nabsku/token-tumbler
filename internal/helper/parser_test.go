package helper

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestIsLetter_ShouldAllowValidTokenPrefixes(t *testing.T) {
	assert.True(t, IsLetter("prefix"))
	assert.True(t, IsLetter("prefix-test_123"))
}

func TestIsLetter_ShouldRejectInvalidTokenPrefixes(t *testing.T) {
	assert.False(t, IsLetter(""))
	assert.False(t, IsLetter("prefix/test"))
	assert.False(t, IsLetter("prefix.test"))
}
