package filetools

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Dxrk777/Dxrk-Ai/internal/strconst"
	"github.com/Dxrk777/Dxrk-Ai/internal/tools"
)

const (
	grepName        = "grep"
	grepDescription = "Search file contents using regular expressions. Returns matching lines with file paths and line numbers."
	maxGrepResults  = 500
	maxLineLength   = 1000
)

type grepMatch struct {
	File string `json:"file"`
	Line int    `json:"line"`
	Text string `json:"text"`
}

func registerGrep(reg *tools.Registry) error {
	t, err := tools.Build(tools.ToolDef{
		Name:        grepName,
		Description: grepDescription,
		InputSchema: map[string]any{
			"type": strconst.StrObject,
			strconst.StrProperties: map[string]any{
				strconst.StrPattern: map[string]any{
					"type":                  strconst.StrString,
					strconst.StrDescription: "The regular expression pattern to search for in file contents",
				},
				"path": map[string]any{
					"type":                  strconst.StrString,
					strconst.StrDescription: "File or directory to search in. Defaults to current working directory.",
				},
				"include": map[string]any{
					"type":                  strconst.StrString,
					strconst.StrDescription: "Glob pattern to filter files (e.g. *.go, *.{ts,tsx})",
				},
				"-i": map[string]any{
					"type":                  "boolean",
					strconst.StrDescription: "Case insensitive search",
					"default":               false,
				},
				"max_results": map[string]any{
					"type":                  strconst.StrInteger,
					strconst.StrDescription: "Maximum number of matching lines to return (default 500)",
					strconst.StrMinimum:     1,
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
			flags := ""
			if caseInsensitive, ok := input["-i"].(bool); ok && caseInsensitive {
				flags = "(?i)"
			}
			if _, err := regexp.Compile(flags + pattern); err != nil {
				return fmt.Errorf("invalid regex pattern: %w", err)
			}
			if v, ok := input["path"].(string); ok && v != "" {
				if !filepath.IsAbs(v) {
					return fmt.Errorf("path must be an absolute path")
				}
				if _, err := os.Stat(v); err != nil {
					return fmt.Errorf("path does not exist: %s", v)
				}
			}
			return nil
		},
		Execute:    executeGrep,
		IsReadOnly: boolPtr(true),
	})
	if err != nil {
		return err
	}
	return reg.Register(t)
}

func executeGrep(_ tools.Context, input map[string]any) (any, error) {
	pattern := input[strconst.StrPattern].(string)

	searchPath := "."
	if v, ok := input["path"].(string); ok && v != "" {
		searchPath = v
	}

	include := ""
	if v, ok := input["include"].(string); ok {
		include = v
	}

	caseInsensitive := false
	if v, ok := input["-i"].(bool); ok {
		caseInsensitive = v
	}

	maxResults := maxGrepResults
	if v, ok := input["max_results"].(float64); ok {
		maxResults = int(v)
	} else if v, ok := input["max_results"].(int); ok {
		maxResults = v
	}

	flags := ""
	if caseInsensitive {
		flags = "(?i)"
	}
	re, err := regexp.Compile(flags + pattern)
	if err != nil {
		return nil, fmt.Errorf("compile regex: %w", err)
	}

	var includeRe *regexp.Regexp
	if include != "" {
		includePattern := include
		if !strings.Contains(includePattern, "*") {
			includePattern = "*." + includePattern
		}
		includeRe, err = regexp.Compile(includePattern)
		if err != nil {
			return nil, fmt.Errorf("compile include pattern: %w", err)
		}
	}

	info, err := os.Stat(searchPath)
	if err != nil {
		return nil, fmt.Errorf("stat %q: %w", searchPath, err)
	}

	var matches []grepMatch
	fileMatches := make(map[string]bool)

	if !info.IsDir() {
		matches, _ = searchFile(re, searchPath, searchPath, matches, fileMatches, maxResults)
	} else {
		err = filepath.Walk(searchPath, func(path string, fi os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if fi.IsDir() {
				base := fi.Name()
				if strings.HasPrefix(base, ".") && base != "." {
					return filepath.SkipDir
				}
				switch base {
				case "node_modules", "vendor", ".git", ".hg", ".svn", "__pycache__":
					return filepath.SkipDir
				}
				return nil
			}
			if len(matches) >= maxResults {
				return nil
			}
			if includeRe != nil && !includeRe.MatchString(fi.Name()) {
				return nil
			}
			matches, _ = searchFile(re, path, searchPath, matches, fileMatches, maxResults)
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("walk: %w", err)
		}
	}

	fileList := make([]string, 0, len(fileMatches))
	for f := range fileMatches {
		fileList = append(fileList, f)
	}

	return map[string]any{
		"matches":             matches,
		"num_matches":         len(matches),
		"num_files":           len(fileList),
		strconst.StrFiles:     fileList,
		strconst.StrTruncated: len(matches) >= maxResults,
	}, nil
}

func searchFile(re *regexp.Regexp, filePath, basePath string, matches []grepMatch, fileMatches map[string]bool, limit int) ([]grepMatch, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return matches, nil
	}
	defer func() { _ = f.Close() }()

	relPath, _ := filepath.Rel(basePath, filePath)
	if relPath == "" {
		relPath = filePath
	}

	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		if len(matches) >= limit {
			break
		}
		line := scanner.Text()
		if len(line) > maxLineLength {
			line = line[:maxLineLength] + "... [truncated]"
		}
		if re.MatchString(line) {
			matches = append(matches, grepMatch{
				File: relPath,
				Line: lineNum,
				Text: strings.TrimSpace(line),
			})
			fileMatches[relPath] = true
		}
	}
	return matches, nil
}
