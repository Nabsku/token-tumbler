package helper

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ValidateSecureFilePath checks that a file path is safe for storing secrets.
// It validates that the path is absolute, contains no untrusted symlinks, and that
// the parent directory is owned by the current user and not group/world-writable.
func ValidateSecureFilePath(path string) error {
	if path == "" {
		return fmt.Errorf("path must not be blank")
	}
	if !filepath.IsAbs(path) {
		return fmt.Errorf("path must be absolute: %s", path)
	}

	dir := filepath.Dir(path)
	if err := validateNoSymlinksInPath(dir); err != nil {
		return err
	}

	if err := validateDirectorySecurity(dir); err != nil {
		return err
	}

	return nil
}

func validateNoSymlinksInPath(path string) error {
	cleanPath := filepath.Clean(path)

	// Walk from root to target, checking each component with Lstat.
	// Lstat does not follow symlinks, so we can detect them directly.
	current := string(filepath.Separator)
	parts := strings.Split(cleanPath, string(filepath.Separator))

	for i, part := range parts {
		if part == "" {
			continue
		}
		current = filepath.Join(current, part)

		info, err := os.Lstat(current)
		if err != nil {
			if os.IsNotExist(err) {
				// Path doesn't exist yet; existing parent components have
				// already been verified. Stop here.
				return nil
			}
			return fmt.Errorf("checking path component %s: %w", current, err)
		}

		if info.Mode()&os.ModeSymlink != 0 {
			// Allow system symlinks (e.g., /var on macOS) that are owned by
			// root and not writable by group/other. If trusted, resolve and
			// continue validation from the target path.
			if isTrustedSymlink(info) {
				resolved, err := filepath.EvalSymlinks(current)
				if err != nil {
					return fmt.Errorf("resolving symlink %s: %w", current, err)
				}
				remaining := filepath.Join(parts[i+1:]...)
				if remaining != "" {
					resolved = filepath.Join(resolved, remaining)
				}
				return validateNoSymlinksInPath(resolved)
			}
			return fmt.Errorf("path contains symlink: %s", current)
		}
	}

	return nil
}
