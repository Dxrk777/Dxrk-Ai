// SPDX-License-Identifier: MIT
// Package memory provides file-based memory system with frontmatter,
// auto-dream consolidation, session memory, and age-based staleness tracking.
package memory

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Dxrk777/Dxrk/internal/strconst"
)

// MemType represents the 4 memory types from Claude Code CLI.
type MemType string

const (
	MemTypeUser      MemType = "user"              // Learned user preferences
	MemTypeFeedback  MemType = "feedback"          // User corrections and feedback
	MemTypeProject   MemType = strconst.StrProject // Project-specific knowledge
	MemTypeReference MemType = "reference"         // Reference docs and API info
)

// FrontmatterLine represents a parsed frontmatter key-value pair.
type FrontmatterLine struct {
	Key   string
	Value string
}

// MemoryHeader contains metadata from a memory file's frontmatter.
type MemoryHeader struct {
	Filename    string
	FilePath    string
	MtimeMs     int64
	Description string
	Type        MemType
}

// MemoryManifest is a formatted listing of memory headers.
type MemoryManifest string

const (
	maxMemoryFiles      = 200
	frontmatterMaxLines = 30
	entrypointName      = "MEMORY.md"
)

var frontmatterLineRe = regexp.MustCompile(`^(\w+):\s*(.*)$`)

// ParseFrontmatter extracts name, description, and body from a markdown file
// with optional YAML frontmatter (--- delimited).
func ParseFrontmatter(source string) (name, description, body string) {
	if !strings.HasPrefix(source, "---\n") {
		return "", "", source
	}
	end := strings.Index(source[4:], "\n---")
	if end == -1 {
		return "", "", source
	}
	end += 4
	fm := source[4:end]
	body = strings.TrimPrefix(source[end+4:], "\n")

	for _, line := range strings.Split(fm, "\n") {
		m := frontmatterLineRe.FindStringSubmatch(line)
		if len(m) != 3 {
			continue
		}
		value := strings.TrimSpace(m[2])
		value = strings.Trim(value, `"'`)
		switch m[1] {
		case "name":
			name = value
		case strconst.StrDescription:
			description = value
		case "type":
			switch MemType(strings.ToLower(value)) {
			case MemTypeUser, MemTypeFeedback, MemTypeProject, MemTypeReference:
				// valid
			}
		}
	}
	return name, description, body
}

// ParseMemoryType parses a string into a MemType.
func ParseMemoryType(s string) MemType {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "user":
		return MemTypeUser
	case "feedback":
		return MemTypeFeedback
	case strconst.StrProject:
		return MemTypeProject
	case "reference":
		return MemTypeReference
	default:
		return ""
	}
}

// MemoryAgeDays returns days elapsed since mtime (floor-rounded).
func MemoryAgeDays(mtimeMs int64) int {
	days := (time.Now().UnixMilli() - mtimeMs) / 86_400_000
	if days < 0 {
		return 0
	}
	return int(days)
}

// MemoryAge returns a human-readable age string.
func MemoryAge(mtimeMs int64) string {
	d := MemoryAgeDays(mtimeMs)
	if d == 0 {
		return "today"
	}
	if d == 1 {
		return "yesterday"
	}
	return fmt.Sprintf("%d days ago", d)
}

// MemoryFreshnessText returns a staleness warning for memories >1 day old.
func MemoryFreshnessText(mtimeMs int64) string {
	d := MemoryAgeDays(mtimeMs)
	if d <= 1 {
		return ""
	}
	return fmt.Sprintf(
		"This memory is %d days old. Memories are point-in-time observations, not live state — "+
			"claims about code behavior or file:line citations may be outdated. "+
			"Verify against current code before asserting as fact.", d)
}

// MemoryFreshnessNote returns a system-reminder wrapped freshness note.
func MemoryFreshnessNote(mtimeMs int64) string {
	text := MemoryFreshnessText(mtimeMs)
	if text == "" {
		return ""
	}
	return fmt.Sprintf("<system-reminder>%s</system-reminder>\n", text)
}

// ScanMemoryFiles scans a memory directory for .md files, reads their
// frontmatter, and returns headers sorted newest-first (capped at maxMemoryFiles).
func ScanMemoryFiles(memoryDir string) ([]MemoryHeader, error) {
	entries, err := os.ReadDir(memoryDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read memory dir: %w", err)
	}

	var headers []MemoryHeader
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		if entry.Name() == entrypointName {
			continue
		}
		filePath := filepath.Join(memoryDir, entry.Name())
		data, err := os.ReadFile(filePath) //nolint:gosec
		if err != nil {
			continue
		}

		// Read only first N lines for frontmatter.
		lines := strings.SplitN(string(data), "\n", frontmatterMaxLines+1)
		content := strings.Join(lines, "\n")

		_, desc, body := ParseFrontmatter(content)
		_ = body

		info, err := os.Stat(filePath)
		if err != nil {
			continue
		}

		// Extract type from frontmatter.
		var memType MemType
		for _, line := range lines {
			m := frontmatterLineRe.FindStringSubmatch(line)
			if len(m) == 3 && m[1] == "type" {
				memType = ParseMemoryType(m[2])
				break
			}
		}

		headers = append(headers, MemoryHeader{
			Filename:    entry.Name(),
			FilePath:    filePath,
			MtimeMs:     info.ModTime().UnixMilli(),
			Description: desc,
			Type:        memType,
		})
	}

	sort.Slice(headers, func(i, j int) bool {
		return headers[i].MtimeMs > headers[j].MtimeMs
	})

	if len(headers) > maxMemoryFiles {
		headers = headers[:maxMemoryFiles]
	}
	return headers, nil
}

// FormatMemoryManifest formats memory headers as a text manifest.
func FormatMemoryManifest(memories []MemoryHeader) string {
	var lines []string
	for _, m := range memories {
		tag := ""
		if m.Type != "" {
			tag = fmt.Sprintf("[%s] ", m.Type)
		}
		ts := time.UnixMilli(m.MtimeMs).UTC().Format(time.RFC3339)
		if m.Description != "" {
			lines = append(lines, fmt.Sprintf("- %s%s (%s): %s", tag, m.Filename, ts, m.Description))
		} else {
			lines = append(lines, fmt.Sprintf("- %s%s (%s)", tag, m.Filename, ts))
		}
	}
	return strings.Join(lines, "\n")
}

// FindRelevantMemories returns memory headers relevant to a query.
// Uses simple keyword matching (LLM-based scoring would be added in production).
func FindRelevantMemories(query string, memoryDir string, limit int) ([]MemoryHeader, error) {
	memories, err := ScanMemoryFiles(memoryDir)
	if err != nil {
		return nil, err
	}
	if len(memories) == 0 {
		return nil, nil
	}

	queryLower := strings.ToLower(query)
	type scored struct {
		memory MemoryHeader
		score  int
	}

	var scoredMemories []scored
	for _, m := range memories {
		score := 0
		descLower := strings.ToLower(m.Description)
		nameLower := strings.ToLower(m.Filename)

		// Score based on keyword overlap.
		for _, word := range strings.Fields(queryLower) {
			if len(word) < 3 {
				continue
			}
			if strings.Contains(descLower, word) {
				score += 2
			}
			if strings.Contains(nameLower, word) {
				score++
			}
		}

		// Boost recent memories.
		ageDays := MemoryAgeDays(m.MtimeMs)
		if ageDays <= 1 {
			score += 3
		} else if ageDays <= 7 {
			score++
		}

		if score > 0 {
			scoredMemories = append(scoredMemories, scored{memory: m, score: score})
		}
	}

	sort.Slice(scoredMemories, func(i, j int) bool {
		return scoredMemories[i].score > scoredMemories[j].score
	})

	if limit <= 0 {
		limit = 5
	}
	if limit > len(scoredMemories) {
		limit = len(scoredMemories)
	}

	result := make([]MemoryHeader, limit)
	for i := 0; i < limit; i++ {
		result[i] = scoredMemories[i].memory
	}
	return result, nil
}
