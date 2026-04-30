//go:build !windows

package helper

import (
	"fmt"
	"os"
	"syscall"
)

func validateDirectorySecurity(dir string) error {
	info, err := os.Lstat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			// Directory doesn't exist yet; we'll validate at write time.
			return nil
		}
		return fmt.Errorf("checking directory %s: %w", dir, err)
	}

	if !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", dir)
	}

	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("could not get system stat for %s", dir)
	}

	if int(stat.Uid) != os.Getuid() {
		return fmt.Errorf("directory %s is not owned by current user (uid %d)", dir, os.Getuid())
	}

	mode := info.Mode().Perm()
	if mode&0o020 != 0 {
		return fmt.Errorf("directory %s is group-writable", dir)
	}
	if mode&0o002 != 0 {
		return fmt.Errorf("directory %s is world-writable", dir)
	}

	return nil
}

// isTrustedSymlink reports whether a symlink is safe to traverse.
// On Unix, a symlink is trusted if it is owned by root and not
// group- or world-writable.
func isTrustedSymlink(info os.FileInfo) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return false
	}
	if int(stat.Uid) != 0 {
		return false
	}
	mode := info.Mode().Perm()
	if mode&0o022 != 0 {
		return false
	}
	return true
}
