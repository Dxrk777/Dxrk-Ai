package filetools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Dxrk777/Dxrk/internal/strconst"
	"github.com/Dxrk777/Dxrk/internal/tools"
)

const (
	globName        = "glob"
	globDescription = "Recursively find files matching a glob pattern. Supports standard glob syntax including **, *, ?, and character classes."
	maxGlobResults  = 500
)

func registerGlob(reg *tools.Registry) error {
	t, err := tools.Build(tools.ToolDef{
		Name:        globName,
		Description: globDescription,
		InputSchema: map[string]any{
			"type": strconst.StrObject,
			strconst.StrProperties: map[string]any{
				strconst.StrPattern: map[string]any{
					"type":                  strconst.StrString,
					strconst.StrDescription: "The glob pattern to match files against (e.g. **/*.go, src/**/*.ts)",
				},
				"path": map[string]any{
					"type":                  strconst.StrString,
					strconst.StrDescription: "The directory to search in. Defaults to the current working directory.",
				},
			},
			strconst.StrRequired: []string{strconst.StrPattern},
		},
		Validate: func(input map[string]any) error {
			if input == nil || input[strconst.StrPattern] == nil {
				return fmt.Errorf("pattern is required")
			}
			pattern, ok := input[strconst.StrPattern].(string)
			if !ok || pattern == "" {
				return fmt.Errorf("pattern must be a non-empty string")
			}
			if v, ok := input["path"].(string); ok && v != "" {
				if !filepath.IsAbs(v) {
					return fmt.Errorf("path must be an absolute path")
				}
				info, err := os.Stat(v)
				if err != nil {
					return fmt.Errorf("path does not exist: %s", v)
				}
				if !info.IsDir() {
					return fmt.Errorf("path is not a directory: %s", v)
				}
			}
			return nil
		},
		Execute:    executeGlob,
		IsReadOnly: boolPtr(true),
	})
	if err != nil {
		return err
	}
	return reg.Register(t)
}

func executeGlob(_ tools.Context, input map[string]any) (any, error) {
	pattern := input[strconst.StrPattern].(string)

	searchPath := "."
	if v, ok := input["path"].(string); ok && v != "" {
		searchPath = v
	}

	if !strings.Contains(pattern, "**") {
		fullPattern := filepath.Join(searchPath, pattern)
		matches, err := filepath.Glob(fullPattern)
		if err != nil {
			return nil, fmt.Errorf("glob %q: %w", pattern, err)
		}

		files := make([]string, 0, len(matches))
		for _, m := range matches {
			info, err := os.Stat(m)
			if err != nil {
				continue
			}
			if !info.IsDir() {
				files = append(files, m)
			}
			if len(files) >= maxGlobResults {
				break
			}
		}

		return map[string]any{
			strconst.StrFiles:     files,
			strconst.StrCount:     len(files),
			strconst.StrTruncated: len(matches) > maxGlobResults,
		}, nil
	}

	var files []string
	err := filepath.Walk(searchPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			base := info.Name()
			if strings.HasPrefix(base, ".") && base != "." {
				return filepath.SkipDir
			}
			switch base {
			case "node_modules", "vendor", ".git", ".hg", ".svn", "__pycache__":
				return filepath.SkipDir
			}
			return nil
		}

		matched, err := filepath.Match(pattern, path)
		if err != nil {
			return err
		}

		if matched {
			files = append(files, path)
		}

		if len(files) >= maxGlobResults {
			return fmt.Errorf("limit reached")
		}
		return nil
	})
	if err != nil && err.Error() != "limit reached" {
		return nil, fmt.Errorf("walk: %w", err)
	}

	return map[string]any{
		strconst.StrFiles:     files,
		strconst.StrCount:     len(files),
		strconst.StrTruncated: len(files) >= maxGlobResults,
	}, nil
}
