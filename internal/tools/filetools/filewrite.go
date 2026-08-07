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
	fileWriteName        = "file_write"
	fileWriteDescription = "Write content to a file, creating parent directories as needed. Overwrites existing files."
	maxWriteSizeBytes    = 50 * 1024 * 1024
)

func registerFileWrite(reg *tools.Registry) error {
	t, err := tools.Build(tools.ToolDef{
		Name:        fileWriteName,
		Description: fileWriteDescription,
		InputSchema: map[string]any{
			"type": strconst.StrObject,
			strconst.StrProperties: map[string]any{
				strconst.StrFilePath: map[string]any{
					"type":                  strconst.StrString,
					strconst.StrDescription: "The absolute path to the file to write (must be absolute, not relative)",
				},
				strconst.StrContent: map[string]any{
					"type":                  strconst.StrString,
					strconst.StrDescription: "The content to write to the file",
				},
			},
			strconst.StrRequired: []string{strconst.StrFilePath, strconst.StrContent},
		},
		Validate: func(input map[string]any) error {
			if input == nil || input[strconst.StrFilePath] == nil || input[strconst.StrContent] == nil {
				return fmt.Errorf("file_path and content are required")
			}
			path, ok := input[strconst.StrFilePath].(string)
			if !ok || path == "" {
				return fmt.Errorf("file_path must be a non-empty string")
			}
			if !filepath.IsAbs(path) {
				return fmt.Errorf("file_path must be an absolute path")
			}
			content, ok := input[strconst.StrContent].(string)
			if !ok {
				return fmt.Errorf("content must be a string")
			}
			if len(content) > maxWriteSizeBytes {
				return fmt.Errorf("content exceeds maximum write size (%d > %d bytes)", len(content), maxWriteSizeBytes)
			}
			return nil
		},
		Execute:    executeFileWrite,
		IsReadOnly: boolPtr(false),
	})
	if err != nil {
		return err
	}
	return reg.Register(t)
}

func executeFileWrite(_ tools.Context, input map[string]any) (any, error) {
	filePath := input[strconst.StrFilePath].(string)
	content := input[strconst.StrContent].(string)

	existed := true
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		existed = false
	}

	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir %q: %w", dir, err)
	}

	linesBefore := 0
	if existed {
		data, err := os.ReadFile(filePath)
		if err == nil {
			linesBefore = strings.Count(string(data), "\n") + 1
		}
	}

	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		return nil, fmt.Errorf("write %q: %w", filePath, err)
	}

	linesAfter := strings.Count(content, "\n") + 1

	op := "create"
	if existed {
		op = "update"
	}

	return map[string]any{
		"type":                op,
		"path":                filePath,
		strconst.StrSizeBytes: len(content),
		"lines":               linesAfter,
		"lines_before":        linesBefore,
	}, nil
}
