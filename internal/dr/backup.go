// SPDX-License-Identifier: MIT
package dr

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

type BackupResult struct {
	Path      string
	Size      int64
	Checksum  string
	Timestamp time.Time
}

func Backup(ctx context.Context, src, dst string) (*BackupResult, error) {
	if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
		return nil, fmt.Errorf("create backup directory: %w", err)
	}

	srcFile, err := os.Open(src) //nolint:gosec // src is an internal backup path
	if err != nil {
		return nil, fmt.Errorf("open source: %w", err)
	}
	defer srcFile.Close() //nolint:errcheck,gosec // best-effort cleanup

	dstFile, err := os.Create(dst) //nolint:gosec // dst is an internal backup path
	if err != nil {
		return nil, fmt.Errorf("create destination: %w", err)
	}

	hash := sha256.New()
	writer := io.MultiWriter(dstFile, hash)

	written, err := io.Copy(writer, srcFile)
	if err != nil {
		dstFile.Close() //nolint:errcheck,gosec // best-effort cleanup
		os.Remove(dst)  //nolint:errcheck,gosec // best-effort cleanup
		return nil, fmt.Errorf("copy data: %w", err)
	}

	if err := dstFile.Close(); err != nil {
		os.Remove(dst) //nolint:errcheck,gosec // best-effort cleanup
		return nil, fmt.Errorf("close destination: %w", err)
	}

	checksum := hex.EncodeToString(hash.Sum(nil))

	return &BackupResult{
		Path:      dst,
		Size:      written,
		Checksum:  checksum,
		Timestamp: time.Now(),
	}, nil
}

func Restore(ctx context.Context, src, dst string) error {
	srcFile, err := os.Open(src) //nolint:gosec // src is an internal backup path
	if err != nil {
		return fmt.Errorf("open backup: %w", err)
	}
	defer srcFile.Close() //nolint:errcheck,gosec // best-effort cleanup

	if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
		return fmt.Errorf("create restore directory: %w", err)
	}

	dstFile, err := os.Create(dst) //nolint:gosec // dst is an internal backup path
	if err != nil {
		return fmt.Errorf("create destination: %w", err)
	}

	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		dstFile.Close() //nolint:errcheck,gosec // best-effort cleanup
		os.Remove(dst)  //nolint:errcheck,gosec // best-effort cleanup
		return fmt.Errorf("restore data: %w", err)
	}

	return dstFile.Close()
}

func CleanupBackups(ctx context.Context, dir string, maxAge time.Duration) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read backup directory: %w", err)
	}

	cutoff := time.Now().Add(-maxAge)
	var lastErr error

	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			lastErr = err
			continue
		}

		if info.ModTime().Before(cutoff) {
			path := filepath.Join(dir, entry.Name())
			if err := os.RemoveAll(path); err != nil {
				lastErr = err
			}
		}
	}

	return lastErr
}
