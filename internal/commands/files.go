// SPDX-License-Identifier: MIT
package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func RegisterFilesCommand(reg *Registry) {
	reg.AddCommand(&cobra.Command{
		Use:   "files [dir]",
		Short: "List recently accessed or modified files",
		Long:  "Show files in the current or specified directory sorted by modification time.",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) > 0 {
				dir = args[0]
			}
			limit := 20
			return runFiles(dir, limit)
		},
	})
}

func runFiles(dir string, limit int) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read directory %q: %w", dir, err)
	}

	type fileEntry struct {
		name    string
		modTime time.Time
		size    int64
		isDir   bool
	}

	var files []fileEntry
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, fileEntry{
			name:    e.Name(),
			modTime: info.ModTime(),
			size:    info.Size(),
			isDir:   e.IsDir(),
		})
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].modTime.After(files[j].modTime)
	})

	if limit > len(files) {
		limit = len(files)
	}
	files = files[:limit]

	absDir, _ := filepath.Abs(dir)
	fmt.Fprintf(os.Stderr, "Recent files in %s\n", absDir)
	fmt.Fprintf(os.Stderr, "──────────────────\n")

	for _, f := range files {
		icon := "  "
		if f.isDir {
			icon = "/ "
		}
		rel := f.name
		fmt.Fprintf(os.Stderr, "  %s%-40s  %s  %s\n",
			icon,
			rel,
			f.modTime.Format("Jan 02 15:04"),
			formatFileSize(f.size),
		)
	}

	return nil
}

func formatFileSize(b int64) string {
	const (
		kb = 1024
		mb = kb * 1024
		gb = mb * 1024
	)
	switch {
	case b >= gb:
		return fmt.Sprintf("%.1f GB", float64(b)/float64(gb))
	case b >= mb:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(mb))
	case b >= kb:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(kb))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

// RecentProjectFiles returns files modified within the given duration.
func RecentProjectFiles(dir string, within time.Duration) ([]string, error) {
	cutoff := time.Now().Add(-within)

	var result []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			name := info.Name()
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if info.ModTime().After(cutoff) {
			result = append(result, path)
		}
		return nil
	})
	return result, err
}
