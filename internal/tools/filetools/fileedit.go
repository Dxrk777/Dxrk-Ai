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
	fileEditName        = "file_edit"
	fileEditDescription = "Edit a file by performing exact string search-and-replace. The old_string must match exactly (including whitespace and indentation). Use replace_all to replace all occurrences."
	maxEditFileSize     = 100 * 1024 * 1024
)

func registerFileEdit(reg *tools.Registry) error {
	t, err := tools.Build(tools.ToolDef{
		Name:        fileEditName,
		Description: fileEditDescription,
		InputSchema: map[string]any{
			"type": strconst.StrObject,
			strconst.StrProperties: map[string]any{
				strconst.StrFilePath: map[string]any{
					"type":                  strconst.StrString,
					strconst.StrDescription: "The absolute path to the file to edit",
				},
				"old_string": map[string]any{
					"type":                  strconst.StrString,
					strconst.StrDescription: "The exact string to find and replace (must match exactly including whitespace)",
				},
				"new_string": map[string]any{
					"type":                  strconst.StrString,
					strconst.StrDescription: "The string to replace old_string with",
				},
				"replace_all": map[string]any{
					"type":                  "boolean",
					strconst.StrDescription: "Replace all occurrences (default: false)",
					"default":               false,
				},
			},
			strconst.StrRequired: []string{strconst.StrFilePath, "old_string", "new_string"},
		},
		Validate: func(input map[string]any) error {
			if input == nil || input[strconst.StrFilePath] == nil || input["old_string"] == nil || input["new_string"] == nil {
				return fmt.Errorf("file_path, old_string, and new_string are required")
			}
			path, ok := input[strconst.StrFilePath].(string)
			if !ok || path == "" {
				return fmt.Errorf("file_path must be a non-empty string")
			}
			if !filepath.IsAbs(path) {
				return fmt.Errorf("file_path must be an absolute path")
			}
			oldStr, _ := input["old_string"].(string)
			newStr, _ := input["new_string"].(string)
			if oldStr == newStr {
				return fmt.Errorf("old_string and new_string are identical; no changes to make")
			}
			return nil
		},
		Execute:    executeFileEdit,
		IsReadOnly: boolPtr(false),
	})
	if err != nil {
		return err
	}
	return reg.Register(t)
}

func executeFileEdit(_ tools.Context, input map[string]any) (any, error) {
	filePath := input[strconst.StrFilePath].(string)
	oldString := input["old_string"].(string)
	newString := input["new_string"].(string)

	replaceAll := false
	if v, ok := input["replace_all"].(bool); ok {
		replaceAll = v
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file does not exist: %s", filePath)
		}
		return nil, fmt.Errorf("read %q: %w", filePath, err)
	}

	if info, err := os.Stat(filePath); err == nil && info.Size() > maxEditFileSize {
		return nil, fmt.Errorf("file %q is too large to edit (%d bytes)", filePath, info.Size())
	}

	content := string(data)

	if !strings.Contains(content, oldString) {
		return nil, fmt.Errorf("old_string not found in %q", filePath)
	}

	count := strings.Count(content, oldString)
	if count > 1 && !replaceAll {
		return nil, fmt.Errorf("found %d matches of old_string in %q; set replace_all=true or provide more context to uniquely identify the instance", count, filePath)
	}

	var updated string
	if replaceAll {
		updated = strings.ReplaceAll(content, oldString, newString)
	} else {
		updated = strings.Replace(content, oldString, newString, 1)
	}

	if err := os.WriteFile(filePath, []byte(updated), 0o644); err != nil {
		return nil, fmt.Errorf("write %q: %w", filePath, err)
	}

	linesBefore := strings.Count(content, "\n") + 1
	linesAfter := strings.Count(updated, "\n") + 1

	return map[string]any{
		"path":         filePath,
		"replacements": count,
		"replace_all":  replaceAll,
		"lines_before": linesBefore,
		"lines_after":  linesAfter,
	}, nil
}
