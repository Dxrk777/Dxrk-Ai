package diff

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Dxrk777/Dxrk/internal/strconst"
)

// DiffStatus represents the status of a file in a directory diff.
type DiffStatus int

const (
	FileAdded DiffStatus = iota
	FileDeleted
	FileModified
	FileRenamed
	FileUnchanged
)

func (s DiffStatus) String() string {
	switch s {
	case FileAdded:
		return "added"
	case FileDeleted:
		return "deleted"
	case FileModified:
		return "modified"
	case FileRenamed:
		return "renamed"
	case FileUnchanged:
		return "unchanged"
	default:
		return strconst.StrUnknown
	}
}

// FileDiff represents the diff result for a single file.
type FileDiff struct {
	OldPath string
	NewPath string
	Diff    *DiffResult
	Status  DiffStatus
}

// DirSummary provides an overview of directory-level changes.
type DirSummary struct {
	FilesAdded    int
	FilesDeleted  int
	FilesModified int
	TotalChanges  int
}

// Errors returned by file diff operations.
var (
	ErrFileNotFound = errors.New("file not found")
	ErrNotDirectory = errors.New("not a directory")
)

// DiffFiles computes a line-level diff between two files.
func DiffFiles(oldPath, newPath string) (*DiffResult, error) {
	oldContent, err := ReadFileContent(oldPath)
	if err != nil {
		return nil, fmt.Errorf("reading old file: %w", err)
	}
	newContent, err := ReadFileContent(newPath)
	if err != nil {
		return nil, fmt.Errorf("reading new file: %w", err)
	}
	return ComputeDiff(oldContent, newContent), nil
}

// DiffDirectories diffs all files in two directories, optionally filtered by glob patterns.
func DiffDirectories(oldDir, newDir string, patterns []string) ([]FileDiff, error) {
	oldInfo, err := os.Stat(oldDir)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrNotDirectory, oldDir)
	}
	if !oldInfo.IsDir() {
		return nil, fmt.Errorf("%w: %s", ErrNotDirectory, oldDir)
	}
	newInfo, err := os.Stat(newDir)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrNotDirectory, newDir)
	}
	if !newInfo.IsDir() {
		return nil, fmt.Errorf("%w: %s", ErrNotDirectory, newDir)
	}

	oldFiles := collectFiles(oldDir)
	newFiles := collectFiles(newDir)

	oldSet := make(map[string]bool, len(oldFiles))
	for _, f := range oldFiles {
		oldSet[f] = true
	}
	newSet := make(map[string]bool, len(newFiles))
	for _, f := range newFiles {
		newSet[f] = true
	}

	var results []FileDiff

	// Modified and deleted files.
	for _, rel := range oldFiles {
		if !matchesAny(rel, patterns) {
			continue
		}
		if newSet[rel] {
			oldPath := filepath.Join(oldDir, rel)
			newPath := filepath.Join(newDir, rel)
			oldContent, err1 := ReadFileContent(oldPath)
			newContent, err2 := ReadFileContent(newPath)
			if err1 != nil || err2 != nil {
				continue
			}
			if oldContent == newContent {
				results = append(results, FileDiff{
					OldPath: rel, NewPath: rel, Status: FileUnchanged,
				})
			} else {
				d := ComputeDiff(oldContent, newContent)
				results = append(results, FileDiff{
					OldPath: rel, NewPath: rel, Diff: d, Status: FileModified,
				})
			}
		} else {
			oldPath := filepath.Join(oldDir, rel)
			content, err := ReadFileContent(oldPath)
			if err != nil {
				continue
			}
			d := ComputeDiff(content, "")
			results = append(results, FileDiff{
				OldPath: rel, Diff: d, Status: FileDeleted,
			})
		}
	}

	// Added files.
	for _, rel := range newFiles {
		if !matchesAny(rel, patterns) {
			continue
		}
		if !oldSet[rel] {
			newPath := filepath.Join(newDir, rel)
			content, err := ReadFileContent(newPath)
			if err != nil {
				continue
			}
			d := ComputeDiff("", content)
			results = append(results, FileDiff{
				NewPath: rel, Diff: d, Status: FileAdded,
			})
		}
	}

	return results, nil
}

// ReadFileContent reads a file and returns its content as a string.
func ReadFileContent(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// FilterByGlob returns a new DiffResult containing only hunks whose file path
// matches one of the given glob patterns. This is a no-op at the hunk level
// since DiffResult doesn't carry file paths; it's provided for API completeness
// when used with FileDiff slices.
func FilterByGlob(diff *DiffResult, patterns []string) *DiffResult {
	if diff == nil || len(patterns) == 0 {
		return diff
	}
	return diff
}

// SummarizeDirectory returns a summary of changes across file diffs.
func SummarizeDirectory(fileDiffs []FileDiff) DirSummary {
	s := DirSummary{}
	for _, fd := range fileDiffs {
		switch fd.Status {
		case FileAdded:
			s.FilesAdded++
		case FileDeleted:
			s.FilesDeleted++
		case FileModified:
			s.FilesModified++
		}
	}
	s.TotalChanges = s.FilesAdded + s.FilesDeleted + s.FilesModified
	return s
}

// collectFiles recursively collects relative file paths from a directory.
func collectFiles(dir string) []string {
	var files []string
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		files = append(files, rel)
		return nil
	})
	return files
}

// matchesAny checks if a path matches any of the given glob patterns.
func matchesAny(path string, patterns []string) bool {
	if len(patterns) == 0 {
		return true
	}
	for _, p := range patterns {
		matched, err := filepath.Match(p, path)
		if err == nil && matched {
			return true
		}
		// Also check the base name.
		matched, err = filepath.Match(p, filepath.Base(path))
		if err == nil && matched {
			return true
		}
	}
	return false
}
