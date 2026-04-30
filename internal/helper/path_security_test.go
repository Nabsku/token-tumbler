package helper

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateSecureFilePath_ShouldRejectBlankPath(t *testing.T) {
	err := ValidateSecureFilePath("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "blank")
}

func TestValidateSecureFilePath_ShouldRejectRelativePath(t *testing.T) {
	err := ValidateSecureFilePath("relative/path")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "absolute")
}

func TestValidateSecureFilePath_ShouldAcceptAbsolutePath(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "gitlab-token")

	err := ValidateSecureFilePath(path)
	require.NoError(t, err)
}

func TestValidateSecureFilePath_ShouldRejectSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink tests not supported on Windows")
	}

	tmpDir := t.TempDir()
	realDir := filepath.Join(tmpDir, "real")
	require.NoError(t, os.Mkdir(realDir, 0o700))

	symlinkDir := filepath.Join(tmpDir, "link")
	require.NoError(t, os.Symlink(realDir, symlinkDir))

	err := ValidateSecureFilePath(filepath.Join(symlinkDir, "token"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "symlink")
}

func TestValidateSecureFilePath_ShouldRejectSymlinkInParent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink tests not supported on Windows")
	}

	tmpDir := t.TempDir()
	realDir := filepath.Join(tmpDir, "real")
	require.NoError(t, os.Mkdir(realDir, 0o700))

	symlinkDir := filepath.Join(tmpDir, "link")
	require.NoError(t, os.Symlink(realDir, symlinkDir))

	// Create a subdirectory inside the symlink
	subDir := filepath.Join(symlinkDir, "subdir")
	require.NoError(t, os.Mkdir(subDir, 0o700))

	err := ValidateSecureFilePath(filepath.Join(subDir, "token"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "symlink")
}

func TestValidateSecureFilePath_ShouldRejectGroupWritableDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission tests not supported on Windows")
	}

	tmpDir := t.TempDir()
	insecureDir := filepath.Join(tmpDir, "insecure")
	require.NoError(t, os.Mkdir(insecureDir, 0o700))
	require.NoError(t, os.Chmod(insecureDir, 0o770))
	defer func() {
		require.NoError(t, os.Chmod(insecureDir, 0o700)) // restore for cleanup
	}()

	err := ValidateSecureFilePath(filepath.Join(insecureDir, "token"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "group-writable")
}

func TestValidateSecureFilePath_ShouldRejectWorldWritableDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission tests not supported on Windows")
	}

	tmpDir := t.TempDir()
	insecureDir := filepath.Join(tmpDir, "insecure")
	require.NoError(t, os.Mkdir(insecureDir, 0o700))
	require.NoError(t, os.Chmod(insecureDir, 0o703))
	defer func() {
		require.NoError(t, os.Chmod(insecureDir, 0o700)) // restore for cleanup
	}()

	err := ValidateSecureFilePath(filepath.Join(insecureDir, "token"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "world-writable")
}

func TestValidateSecureFilePath_ShouldRejectWorldWritableButNotGroupWritable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission tests not supported on Windows")
	}

	tmpDir := t.TempDir()
	insecureDir := filepath.Join(tmpDir, "insecure")
	require.NoError(t, os.Mkdir(insecureDir, 0o700))
	require.NoError(t, os.Chmod(insecureDir, 0o707))
	defer func() {
		require.NoError(t, os.Chmod(insecureDir, 0o700)) // restore for cleanup
	}()

	err := ValidateSecureFilePath(filepath.Join(insecureDir, "token"))
	require.Error(t, err)
	// 707 is world-writable but not group-writable
	assert.Contains(t, err.Error(), "world-writable")
}

func TestValidateSecureFilePath_ShouldAcceptSafeDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	safeDir := filepath.Join(tmpDir, "safe")
	require.NoError(t, os.Mkdir(safeDir, 0o700))

	path := filepath.Join(safeDir, "gitlab-token")
	err := ValidateSecureFilePath(path)
	require.NoError(t, err)
}

func TestValidateSecureFilePath_ShouldAllowNonExistentSubdirectory(t *testing.T) {
	tmpDir := t.TempDir()
	// The parent exists and is safe, but the target subdirectory does not.
	path := filepath.Join(tmpDir, "does-not-yet-exist", "gitlab-token")

	err := ValidateSecureFilePath(path)
	require.NoError(t, err)
}

func TestValidateSecureFilePath_ShouldRejectNonDirectoryParent(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "not-a-dir")
	require.NoError(t, os.WriteFile(filePath, []byte("x"), 0o600))

	targetPath := filepath.Join(filePath, "gitlab-token")
	err := ValidateSecureFilePath(targetPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a directory")
}

func TestValidateSecureFilePath_ShouldFollowRootOnUnix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix-specific test")
	}

	// Just verify it doesn't panic on root
	err := ValidateSecureFilePath("/token")
	// This may or may not error depending on root ownership/permissions
	// We just care that it doesn't panic and validates correctly
	_ = err
}

func TestValidateSecureFilePath_ErrorFormatting(t *testing.T) {
	tmpDir := t.TempDir()
	insecureDir := filepath.Join(tmpDir, "insecure")
	require.NoError(t, os.Mkdir(insecureDir, 0o700))
	require.NoError(t, os.Chmod(insecureDir, 0o777))
	defer func() {
		require.NoError(t, os.Chmod(insecureDir, 0o700)) // restore for cleanup
	}()

	err := ValidateSecureFilePath(filepath.Join(insecureDir, "token"))
	require.Error(t, err)
	// Should contain the directory path in the error for debugging
	assert.Contains(t, err.Error(), insecureDir)
}

func ExampleValidateSecureFilePath() {
	// This example shows basic usage
	err := ValidateSecureFilePath("/run/secrets/my-token")
	if err != nil {
		fmt.Println("Validation failed:", err)
	} else {
		fmt.Println("Path is secure")
	}
	// Output depends on filesystem state, so we don't assert output here.
}
