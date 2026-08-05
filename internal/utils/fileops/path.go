package fileops

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

var (
	ErrNotAbsolute = errors.New("fileops: path is not absolute")
	ErrOutsideDir  = errors.New("fileops: path is outside allowed directory")
)

// ResolvePath resolves a potentially relative path against workingDir.
func ResolvePath(path string, workingDir string) (string, error) {
	if err := ValidatePath(path); err != nil {
		return "", err
	}
	if filepath.IsAbs(path) {
		return filepath.Clean(path), nil
	}
	if workingDir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		workingDir = wd
	}
	return filepath.Join(workingDir, path), nil
}

// IsWithinDir checks that path is inside dir (or equals it).
func IsWithinDir(path, dir string) (bool, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false, err
	}
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return false, err
	}
	rel, err := filepath.Rel(absDir, absPath)
	if err != nil {
		return false, err
	}
	if rel == "." {
		return true, nil
	}
	return !strings.HasPrefix(rel, ".."), nil
}

// SafeJoin joins path elements and rejects any result that escapes the first
// element via "..".
func SafeJoin(elem ...string) string {
	joined := filepath.Join(elem...)
	cleaned := filepath.Clean(joined)
	// If the cleaned path is not under the first element, it's a traversal
	if len(elem) > 0 {
		base := filepath.Clean(elem[0])
		if rel, err := filepath.Rel(base, cleaned); err == nil {
			if strings.HasPrefix(rel, "..") {
				return base
			}
		}
	}
	return cleaned
}

// RelativePath returns the relative path from base to path.
func RelativePath(path, base string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	absBase, err := filepath.Abs(base)
	if err != nil {
		return "", err
	}
	return filepath.Rel(absBase, absPath)
}

// ExpandHome replaces a leading ~ with the user's home directory.
func ExpandHome(path string) (string, error) {
	if !strings.HasPrefix(path, "~") {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("fileops: cannot determine home dir: %w", err)
	}
	if path == "~" {
		return home, nil
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, path[2:]), nil
	}
	// ~username form — not supported on all platforms
	return path, nil
}

// IsHidden returns true if the file or directory name starts with a dot.
func IsHidden(path string) bool {
	base := filepath.Base(path)
	return strings.HasPrefix(base, ".")
}

// IsSymlink reports whether path is a symbolic link.
func IsSymlink(path string) (bool, error) {
	fi, err := os.Lstat(path)
	if err != nil {
		return false, err
	}
	return fi.Mode()&os.ModeSymlink != 0, nil
}

// RealPath resolves all symlinks and returns the canonical path.
func RealPath(path string) (string, error) {
	return filepath.EvalSymlinks(path)
}

// Glob performs a recursive glob starting at rootDir matching pattern.
func Glob(pattern string, rootDir string) ([]string, error) {
	if rootDir == "" {
		rootDir = "."
	}
	fullPattern := filepath.Join(rootDir, pattern)
	matches, err := filepath.Glob(fullPattern)
	if err != nil {
		return nil, err
	}
	return matches, nil
}

// WalkDir recursively walks root calling fn for each file or directory.
func WalkDir(root string, fn filepath.WalkFunc) error {
	return filepath.Walk(root, fn)
}

// GetExtension returns the file extension including the leading dot.
func GetExtension(path string) string {
	return filepath.Ext(path)
}

// GetBaseName returns the file name without extension.
func GetBaseName(path string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	if ext == "" {
		return base
	}
	return base[:len(base)-len(ext)]
}

// ChangeExtension replaces the file extension.
func ChangeExtension(path, ext string) string {
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	dir := filepath.Dir(path)
	base := GetBaseName(path)
	return filepath.Join(dir, base+ext)
}

// TmpDir returns the OS temporary directory.
func TmpDir() string {
	return os.TempDir()
}

// TmpFile creates a temporary file with the given extension and returns its
// path and a cleanup function that removes it.
func TmpFile(ext string) (string, func(), error) {
	if !strings.HasPrefix(ext, ".") && ext != "" {
		ext = "." + ext
	}
	f, err := os.CreateTemp("", "fileops-*"+ext)
	if err != nil {
		return "", nil, err
	}
	name := f.Name()
	_ = f.Close()
	cleanup := func() { _ = os.Remove(name) }
	return name, cleanup, nil
}

// MkdirTemp creates a temporary directory and returns its path and a cleanup
// function.
func MkdirTemp(pattern string) (string, func(), error) {
	dir, err := os.MkdirTemp("", pattern)
	if err != nil {
		return "", nil, err
	}
	return dir, func() { _ = os.RemoveAll(dir) }, nil
}

// NormalizePath cleans and resolves a path, returning the absolute canonical form.
func NormalizePath(path string) (string, error) {
	path = filepath.Clean(path)
	if !filepath.IsAbs(path) {
		wd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		path = filepath.Join(wd, path)
	}
	return filepath.Abs(path)
}

// SplitPath splits path into directory and file components.
func SplitPath(path string) (dir, file string) {
	return filepath.Split(path)
}

// DirExists reports whether dir exists and is a directory.
func DirExists(dir string) (bool, error) {
	fi, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return fi.IsDir(), nil
}

// FileExists reports whether path exists and is a regular file.
func FileExists(path string) (bool, error) {
	fi, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return fi.Mode().IsRegular(), nil
}

// IsDir reports whether path is a directory.
func IsDir(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}

// IsFile reports whether path is a regular file.
func IsFile(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.Mode().IsRegular()
}

// ExecutableDir returns the directory of the currently running executable.
func ExecutableDir() (string, error) {
	ex, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Dir(ex), nil
}

// HomeDir returns the user's home directory.
func HomeDir() (string, error) {
	return os.UserHomeDir()
}

// UserCacheDir returns the per-user cache directory for the current platform.
func UserCacheDir() (string, error) {
	return os.UserCacheDir()
}

// UserConfigDir returns the per-user configuration directory.
func UserConfigDir() (string, error) {
	return os.UserConfigDir()
}

// Platform returns the runtime GOOS value.
func Platform() string {
	return runtime.GOOS
}

// CleanPath cleans a path and returns it, handling edge cases like empty strings.
func CleanPath(path string) string {
	if path == "" {
		return "."
	}
	return filepath.Clean(path)
}

// AbsPath returns an absolute version of path.
func AbsPath(path string) (string, error) {
	return filepath.Abs(path)
}

// EnsureTrailingSep ensures the path ends with the OS path separator.
func EnsureTrailingSep(path string) string {
	if !strings.HasSuffix(path, string(os.PathSeparator)) {
		return path + string(os.PathSeparator)
	}
	return path
}
