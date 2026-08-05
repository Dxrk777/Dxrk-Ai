package fileops

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// WriteOpts configures how WriteFile behaves.
type WriteOpts struct {
	CreateDirs bool
	Mode       os.FileMode
	Backup     bool
	Overwrite  bool
	Encoding   string
}

// DefaultWriteOpts returns sensible defaults.
func DefaultWriteOpts() WriteOpts {
	return WriteOpts{
		CreateDirs: true,
		Mode:       0644,
		Overwrite:  true,
	}
}

// WriteFile writes content to path atomically. When CreateDirs is true parent
// directories are created automatically. When Backup is true the existing file
// is renamed to path.bak before writing.
func WriteFile(path string, content string, opts WriteOpts) error {
	if err := ValidatePath(path); err != nil {
		return err
	}
	if opts.Mode == 0 {
		opts.Mode = 0644
	}

	if opts.CreateDirs {
		if err := EnsureDir(path); err != nil {
			return err
		}
	}

	if !opts.Overwrite {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("fileops: file exists and Overwrite is false: %s", path)
		}
	}

	if opts.Backup {
		if _, err := os.Stat(path); err == nil {
			if _, err := BackupFile(path); err != nil {
				return fmt.Errorf("fileops: backup failed: %w", err)
			}
		}
	}

	return WriteAtomic(path, []byte(content))
}

// WriteAtomic writes data to a temporary file in the same directory then
// renames it to the target path. This guarantees the target is never in a
// half-written state.
func WriteAtomic(path string, data []byte) error {
	if err := ValidatePath(path); err != nil {
		return err
	}
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".fileops-tmp-*")
	if err != nil {
		return fmt.Errorf("fileops: create temp: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpName)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("fileops: write temp: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("fileops: sync temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("fileops: close temp: %w", err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("fileops: rename: %w", err)
	}
	cleanup = false
	return nil
}

// BackupFile creates a backup of path by copying it to path.bak.
// It returns the backup path.
func BackupFile(path string) (string, error) {
	if err := ValidatePath(path); err != nil {
		return "", err
	}
	bak := path + ".bak"
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("fileops: read for backup: %w", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(bak, data, info.Mode()); err != nil {
		return "", fmt.Errorf("fileops: write backup: %w", err)
	}
	return bak, nil
}

// WriteLines writes each string as a line (joined by newlines) to path atomically.
func WriteLines(path string, lines []string, opts WriteOpts) error {
	var b strings.Builder
	for i, line := range lines {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(line)
	}
	if len(lines) > 0 {
		b.WriteByte('\n')
	}
	return WriteFile(path, b.String(), opts)
}

// AppendFile appends content to the file at path, creating it if necessary.
func AppendFile(path string, content string) error {
	if err := ValidatePath(path); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	_, err = f.WriteString(content)
	return err
}

// EnsureDir creates parent directories for the given file path if they do not
// already exist.
func EnsureDir(path string) error {
	dir := filepath.Dir(path)
	return os.MkdirAll(dir, 0755)
}
