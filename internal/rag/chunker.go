// SPDX-License-Identifier: MIT
package rag

import (
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

type Chunk struct {
	Text      string `json:"text"`
	FilePath  string `json:"file_path"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	Language  string `json:"language"`
}

type ChunkConfig struct {
	ChunkSize    int
	ChunkOverlap int
}

func DefaultChunkConfig() ChunkConfig {
	return ChunkConfig{ChunkSize: 512, ChunkOverlap: 64}
}

func IsCodeFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go", ".ts", ".tsx", ".js", ".jsx", ".py", ".rs", ".java",
		".c", ".h", ".cpp", ".hpp", ".cs", ".rb", ".php", ".swift",
		".kt", ".scala", ".ex", ".exs", ".clj", ".cljs", ".elm",
		".hs", ".lua", ".r", ".m", ".mm", ".sql", ".sh", ".bash",
		".zsh", ".fish", ".yaml", ".yml", ".json", ".toml", ".md",
		".css", ".scss", ".less", ".html", ".svelte", ".vue":
		return true
	}
	return false
}

func LanguageFromExt(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go":
		return "go"
	case ".ts", ".tsx":
		return "typescript"
	case ".js", ".jsx":
		return "javascript"
	case ".py":
		return "python"
	case ".rs":
		return "rust"
	case ".java":
		return "java"
	case ".c", ".h":
		return "c"
	case ".cpp", ".hpp", ".cc":
		return "cpp"
	case ".cs":
		return "csharp"
	case ".rb":
		return "ruby"
	case ".php":
		return "php"
	case ".swift":
		return "swift"
	case ".kt":
		return "kotlin"
	case ".sh", ".bash", ".zsh", ".fish":
		return "shell"
	case ".yaml", ".yml":
		return "yaml"
	case ".json":
		return "json"
	case ".toml":
		return "toml"
	case ".md":
		return "markdown"
	case ".html", ".svelte", ".vue":
		return "html"
	case ".css", ".scss", ".less":
		return "css"
	default:
		return "text"
	}
}

func ChunkFile(path string, cfg ChunkConfig) ([]Chunk, error) {
	data, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		return nil, err
	}

	if !utf8.Valid(data) {
		return nil, nil
	}

	text := string(data)
	lang := LanguageFromExt(path)
	lines := strings.Split(text, "\n")

	if len(lines) <= cfg.ChunkSize {
		return []Chunk{{
			Text:      text,
			FilePath:  path,
			StartLine: 1,
			EndLine:   len(lines),
			Language:  lang,
		}}, nil
	}

	var chunks []Chunk
	step := cfg.ChunkSize - cfg.ChunkOverlap
	if step < 1 {
		step = 1
	}

	for i := 0; i < len(lines); i += step {
		end := i + cfg.ChunkSize
		if end > len(lines) {
			end = len(lines)
		}

		chunkText := strings.Join(lines[i:end], "\n")
		chunks = append(chunks, Chunk{
			Text:      chunkText,
			FilePath:  path,
			StartLine: i + 1,
			EndLine:   end,
			Language:  lang,
		})

		if end >= len(lines) {
			break
		}
	}

	return chunks, nil
}

var DefaultIgnoreDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	"__pycache__":  true,
	".venv":        true,
	"vendor":       true,
	"target":       true,
	"dist":         true,
	"build":        true,
	".next":        true,
	".cache":       true,
	"third_party":  true,
	".dxrk":        true,
}
