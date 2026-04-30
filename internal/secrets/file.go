package secrets

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const fileSecretMode = 0o600

type FileSecret struct {
	Path string
}

func (fs *FileSecret) InitClient(_ context.Context) error {
	return nil
}

func (fs *FileSecret) Read(_ context.Context) (string, error) {
	path := strings.TrimSpace(fs.Path)
	if path == "" {
		return "", fmt.Errorf("filePath must not be blank")
	}

	contents, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading file secret %s: %w", path, err)
	}
	return string(contents), nil
}

func (fs *FileSecret) Write(_ context.Context, value string) error {
	path := strings.TrimSpace(fs.Path)
	if path == "" {
		return fmt.Errorf("filePath must not be blank")
	}

	dir := filepath.Dir(path)
	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("checking file secret parent directory %s: %w", dir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("file secret parent path %s is not a directory", dir)
	}

	tmp, err := os.CreateTemp(dir, ".token-tumbler-*")
	if err != nil {
		return fmt.Errorf("creating temporary file secret in %s: %w", dir, err)
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	if err := tmp.Chmod(fileSecretMode); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("setting permissions on temporary file secret %s: %w", tmpPath, err)
	}
	if _, err := tmp.WriteString(value); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("writing temporary file secret %s: %w", tmpPath, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing temporary file secret %s: %w", tmpPath, err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("renaming temporary file secret %s to %s: %w", tmpPath, path, err)
	}

	cleanup = false
	return nil
}
