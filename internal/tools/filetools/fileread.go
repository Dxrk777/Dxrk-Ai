package filetools

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/Dxrk777/Dxrk-Ai/internal/strconst"
	"github.com/Dxrk777/Dxrk-Ai/internal/tools"
)

const (
	fileReadName        = "file_read"
	fileReadDescription = "Read the contents of a file from the local filesystem. Supports text files with offset/limit for line-based reading, and binary files (images, PDFs) returned as base64."
	maxReadSizeBytes    = 10 * 1024 * 1024
)

func registerFileRead(reg *tools.Registry) error {
	t, err := tools.Build(tools.ToolDef{
		Name:        fileReadName,
		Description: fileReadDescription,
		InputSchema: map[string]any{
			"type": strconst.StrObject,
			strconst.StrProperties: map[string]any{
				strconst.StrFilePath: map[string]any{
					"type":                  strconst.StrString,
					strconst.StrDescription: "The absolute path to the file to read",
				},
				"offset": map[string]any{
					"type":                  strconst.StrInteger,
					strconst.StrDescription: "The line number to start reading from (1-indexed). Only provide if the file is too large to read at once",
					strconst.StrMinimum:     0,
				},
				"limit": map[string]any{
					"type":                  strconst.StrInteger,
					strconst.StrDescription: "The number of lines to read. Only provide if the file is too large to read at once.",
					strconst.StrMinimum:     1,
				},
			},
			strconst.StrRequired: []string{strconst.StrFilePath},
		},
		Validate: func(input map[string]any) error {
			if input == nil || input[strconst.StrFilePath] == nil {
				return fmt.Errorf("file_path is required")
			}
			path, ok := input[strconst.StrFilePath].(string)
			if !ok || path == "" {
				return fmt.Errorf("file_path must be a non-empty string")
			}
			if !filepath.IsAbs(path) {
				return fmt.Errorf("file_path must be an absolute path")
			}
			return nil
		},
		Execute:    executeFileRead,
		IsReadOnly: boolPtr(true),
	})
	if err != nil {
		return err
	}
	return reg.Register(t)
}

func executeFileRead(_ tools.Context, input map[string]any) (any, error) {
	filePath := input[strconst.StrFilePath].(string)

	info, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file does not exist: %s", filePath)
		}
		return nil, fmt.Errorf("stat %q: %w", filePath, err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("%q is a directory, not a file", filePath)
	}
	if info.Size() > maxReadSizeBytes {
		return nil, fmt.Errorf("file %q exceeds maximum read size (%d bytes)", filePath, maxReadSizeBytes)
	}

	ext := strings.ToLower(filepath.Ext(filePath))
	if isImageExtension(ext) {
		return readImageFile(filePath, ext)
	}
	if ext == ".pdf" {
		return readPDFFile(filePath)
	}

	return readTextFile(filePath, input)
}

func readTextFile(filePath string, input map[string]any) (map[string]any, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read %q: %w", filePath, err)
	}

	encoding := detectEncoding(data)
	content := string(data)
	if !utf8.ValidString(content) {
		content = string(data)
	}

	lines := strings.Split(content, "\n")
	totalLines := len(lines)

	offset := 0
	if v, ok := input["offset"].(float64); ok {
		offset = int(v)
	} else if v, ok := input["offset"].(int); ok {
		offset = v
	}
	if offset < 0 {
		offset = 0
	}

	limit := 0
	if v, ok := input["limit"].(float64); ok {
		limit = int(v)
	} else if v, ok := input["limit"].(int); ok {
		limit = v
	}

	startLine := offset
	if startLine > totalLines {
		return map[string]any{
			"path":                filePath,
			strconst.StrContent:   "",
			"start_line":          totalLines + 1,
			"total_lines":         totalLines,
			"encoding":            encoding,
			strconst.StrTruncated: false,
			"warning":             fmt.Sprintf("offset %d exceeds file length (%d lines)", offset, totalLines),
		}, nil
	}

	if limit > 0 {
		endLine := startLine + limit
		if endLine > totalLines {
			endLine = totalLines
		}
		lines = lines[startLine:endLine]
	} else {
		lines = lines[startLine:]
	}

	outputContent := strings.Join(lines, "\n")
	startNum := startLine + 1

	return map[string]any{
		"path":                filePath,
		strconst.StrContent:   outputContent,
		"start_line":          startNum,
		"num_lines":           len(lines),
		"total_lines":         totalLines,
		"encoding":            encoding,
		strconst.StrTruncated: false,
	}, nil
}

func readImageFile(filePath, ext string) (map[string]any, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read image %q: %w", filePath, err)
	}

	mediaType := imageMediaType(ext)
	return map[string]any{
		"type":                "image",
		"path":                filePath,
		"base64":              encodeBase64(data),
		"media_type":          mediaType,
		strconst.StrSizeBytes: len(data),
	}, nil
}

func readPDFFile(filePath string) (map[string]any, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read pdf %q: %w", filePath, err)
	}

	return map[string]any{
		"type":                "pdf",
		"path":                filePath,
		"base64":              encodeBase64(data),
		strconst.StrSizeBytes: len(data),
	}, nil
}

func detectEncoding(data []byte) string {
	if len(data) >= 2 && data[0] == 0xFF && data[1] == 0xFE {
		return "utf16le"
	}
	if len(data) >= 2 && data[0] == 0xFE && data[1] == 0xFF {
		return "utf16be"
	}
	if len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
		return "utf8-bom"
	}
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := scanner.Text()
		if !utf8.ValidString(line) {
			return "binary"
		}
	}
	return "utf8"
}

func isImageExtension(ext string) bool {
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp", ".svg":
		return true
	}
	return false
}

func imageMediaType(ext string) string {
	switch ext {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".bmp":
		return "image/bmp"
	case ".svg":
		return "image/svg+xml"
	default:
		return "application/octet-stream"
	}
}

func encodeBase64(data []byte) string {
	import_base64 := __import_base64()
	return import_base64.EncodeToString(data)
}

type base64Encoder struct {
}

func __import_base64() base64Encoder {
	return base64Encoder{}
}

func (base64Encoder) EncodeToString(data []byte) string {
	const encoder = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	const padding = '='

	result := make([]byte, 0, (len(data)+2)/3*4)
	for i := 0; i < len(data); i += 3 {
		var b0, b1, b2 byte
		b0 = data[i]
		if i+1 < len(data) {
			b1 = data[i+1]
		}
		if i+2 < len(data) {
			b2 = data[i+2]
		}

		result = append(result, encoder[b0>>2])
		result = append(result, encoder[((b0&0x03)<<4)|(b1>>4)])

		if i+1 < len(data) {
			result = append(result, encoder[((b1&0x0F)<<2)|(b2>>6)])
		} else {
			result = append(result, padding)
		}

		if i+2 < len(data) {
			result = append(result, encoder[b2&0x3F])
		} else {
			result = append(result, padding)
		}
	}
	return string(result)
}
