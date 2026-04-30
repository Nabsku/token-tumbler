//go:build windows

package helper

import (
	"fmt"
	"os"
)

func validateDirectorySecurity(dir string) error {
	info, err := os.Lstat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("checking directory %s: %w", dir, err)
	}

	if !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", dir)
	}

	return nil
}

// isTrustedSymlink reports whether a symlink is safe to traverse.
// On Windows, we conservatively reject all symlinks because ownership
// and permission checks are more complex.
func isTrustedSymlink(_ os.FileInfo) bool {
	return false
}
