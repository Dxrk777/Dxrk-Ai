package extractmemories

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Dxrk777/Dxrk-Ai/internal/strconst"
)

type ExtractConfig struct {
	Enabled          bool
	AutoMemoryPath   string
	ExtractionPrompt string
	ManifestPath     string
}

func defaultConfig() ExtractConfig {
	home, _ := os.UserHomeDir()
	autoPath := filepath.Join(home, ".config", "claude-code", "auto-memory")
	return ExtractConfig{
		Enabled:        true,
		AutoMemoryPath: autoPath,
		ManifestPath:   filepath.Join(autoPath, "manifest.json"),
	}
}

func resolveConfig(cfg ExtractConfig) ExtractConfig {
	d := defaultConfig()
	if cfg.AutoMemoryPath == "" {
		cfg.AutoMemoryPath = d.AutoMemoryPath
	}
	if cfg.ManifestPath == "" {
		cfg.ManifestPath = filepath.Join(cfg.AutoMemoryPath, "manifest.json")
	}
	return cfg
}

type MemoryEntry struct {
	ID        string    `json:"id"`
	Key       string    `json:"key"`
	Content   string    `json:"content"`
	Source    string    `json:"source"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type MemoryManifest struct {
	mu          sync.Mutex
	Entries     []MemoryEntry `json:"entries"`
	LastUpdated time.Time     `json:"last_updated"`
	Version     string        `json:"version"`
}

type ExtractService struct {
	config   ExtractConfig
	manifest MemoryManifest
}

func NewExtractService(config ExtractConfig) *ExtractService {
	cfg := resolveConfig(config)
	s := &ExtractService{config: cfg}
	_ = s.LoadManifest()
	return s
}

func generateID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("mem-%x", b)
}

func (s *ExtractService) ExtractMemories(ctx context.Context, conversationSummary string) ([]MemoryEntry, error) {
	if conversationSummary == "" {
		return nil, nil
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	entries := parseConversationSummary(conversationSummary)
	if len(entries) == 0 {
		return nil, nil
	}

	s.manifest.mu.Lock()
	defer s.manifest.mu.Unlock()

	var newEntries []MemoryEntry
	for _, e := range entries {
		if s.isDuplicate(e) {
			continue
		}
		e.ID = generateID()
		e.CreatedAt = time.Now()
		e.UpdatedAt = e.CreatedAt
		s.manifest.Entries = append(s.manifest.Entries, e)
		newEntries = append(newEntries, e)
	}

	s.manifest.LastUpdated = time.Now()
	if s.manifest.Version == "" {
		s.manifest.Version = "1.0"
	}

	return newEntries, nil
}

func (s *ExtractService) SaveManifest() error {
	s.manifest.mu.Lock()
	data, err := json.MarshalIndent(&s.manifest, "", "  ")
	s.manifest.mu.Unlock()
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}

	dir := filepath.Dir(s.config.ManifestPath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create manifest dir: %w", err)
	}

	return os.WriteFile(s.config.ManifestPath, data, 0o600)
}

func (s *ExtractService) LoadManifest() error {
	s.manifest.mu.Lock()
	defer s.manifest.mu.Unlock()

	data, err := os.ReadFile(s.config.ManifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			s.manifest.Entries = nil
			s.manifest.Version = "1.0"
			return nil
		}
		return fmt.Errorf("read manifest: %w", err)
	}

	var m MemoryManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return fmt.Errorf("unmarshal manifest: %w", err)
	}

	s.manifest.Entries = m.Entries
	s.manifest.LastUpdated = m.LastUpdated
	s.manifest.Version = m.Version
	if s.manifest.Version == "" {
		s.manifest.Version = "1.0"
	}
	return nil
}

func (s *ExtractService) GetMemories() []MemoryEntry {
	s.manifest.mu.Lock()
	defer s.manifest.mu.Unlock()
	out := make([]MemoryEntry, len(s.manifest.Entries))
	copy(out, s.manifest.Entries)
	return out
}

func (s *ExtractService) SearchMemories(query string) []MemoryEntry {
	s.manifest.mu.Lock()
	defer s.manifest.mu.Unlock()

	q := strings.ToLower(query)
	var results []MemoryEntry
	for _, e := range s.manifest.Entries {
		if strings.Contains(strings.ToLower(e.Key), q) || strings.Contains(strings.ToLower(e.Content), q) {
			results = append(results, e)
		}
	}
	return results
}

func (s *ExtractService) DeleteMemory(id string) error {
	s.manifest.mu.Lock()
	defer s.manifest.mu.Unlock()

	for i, e := range s.manifest.Entries {
		if e.ID == id {
			s.manifest.Entries = append(s.manifest.Entries[:i], s.manifest.Entries[i+1:]...)
			s.manifest.LastUpdated = time.Now()
			return nil
		}
	}
	return fmt.Errorf("memory %s not found", id)
}

func (s *ExtractService) BuildExtractionPrompt() string {
	if s.config.ExtractionPrompt != "" {
		return s.config.ExtractionPrompt
	}

	return `You are a memory extraction subagent. Analyze the conversation and extract durable, reusable knowledge worth persisting.

## What to save
- User preferences and project conventions
- Key decisions and their rationale
- Repeated patterns or corrections
- Tool/library choices and configurations
- Architecture decisions and constraints

## What NOT to save
- One-off commands or trivial interactions
- Sensitive data (API keys, credentials, tokens)
- Temporary debugging information
- Generic programming knowledge

## How to save
Write each memory as a structured entry with:
- Key: short descriptive identifier (e.g. "user_pref_typescript_strict")
- Content: the actual knowledge worth remembering
- Source: where this memory was extracted from

Organize semantically by topic, not chronologically.
Update or remove memories that turn out to be wrong or outdated.
Do not create duplicates — check existing memories first.`
}

func (s *ExtractService) MergeMemories(newEntries []MemoryEntry) int {
	s.manifest.mu.Lock()
	defer s.manifest.mu.Unlock()

	count := 0
	for _, e := range newEntries {
		if s.isDuplicate(e) {
			continue
		}
		if e.ID == "" {
			e.ID = generateID()
		}
		if e.CreatedAt.IsZero() {
			e.CreatedAt = time.Now()
		}
		e.UpdatedAt = time.Now()
		s.manifest.Entries = append(s.manifest.Entries, e)
		count++
	}

	if count > 0 {
		s.manifest.LastUpdated = time.Now()
		if s.manifest.Version == "" {
			s.manifest.Version = "1.0"
		}
	}

	return count
}

func (s *ExtractService) isDuplicate(entry MemoryEntry) bool {
	for _, existing := range s.manifest.Entries {
		if existing.Key == entry.Key && existing.Source == entry.Source {
			return true
		}
	}
	return false
}

func parseConversationSummary(summary string) []MemoryEntry {
	var entries []MemoryEntry
	lines := strings.Split(summary, "\n")

	var currentKey, currentContent strings.Builder
	state := "scan"

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		switch {
		case strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* "):
			item := strings.TrimPrefix(strings.TrimPrefix(trimmed, "- "), "* ")
			if item == "" {
				continue
			}

			parts := strings.SplitN(item, ":", 2)
			if len(parts) == 2 {
				if currentKey.Len() > 0 && currentContent.Len() > 0 {
					entries = append(entries, MemoryEntry{
						Key:     strings.TrimSpace(currentKey.String()),
						Content: strings.TrimSpace(currentContent.String()),
						Source:  strconst.StrConversation,
					})
					currentKey.Reset()
					currentContent.Reset()
				}
				currentKey.WriteString(strings.TrimSpace(parts[0]))
				currentContent.WriteString(strings.TrimSpace(parts[1]))
				state = strconst.StrContent
			} else if state == strconst.StrContent {
				currentContent.WriteString(" ")
				currentContent.WriteString(item)
			}
		case trimmed == "":
			if currentKey.Len() > 0 && currentContent.Len() > 0 {
				entries = append(entries, MemoryEntry{
					Key:     strings.TrimSpace(currentKey.String()),
					Content: strings.TrimSpace(currentContent.String()),
					Source:  strconst.StrConversation,
				})
				currentKey.Reset()
				currentContent.Reset()
				state = "scan"
			}
		case state == strconst.StrContent:
			currentContent.WriteString(" ")
			currentContent.WriteString(trimmed)
		}
	}

	if currentKey.Len() > 0 && currentContent.Len() > 0 {
		entries = append(entries, MemoryEntry{
			Key:     strings.TrimSpace(currentKey.String()),
			Content: strings.TrimSpace(currentContent.String()),
			Source:  strconst.StrConversation,
		})
	}

	return entries
}
