package sessionmemory

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

type SessionMemoryConfig struct {
	Enabled              bool
	ExtractionInterval   int
	MaxNotesLength       int
	BackgroundExtraction bool
}

func defaultConfig() SessionMemoryConfig {
	return SessionMemoryConfig{
		Enabled:              true,
		ExtractionInterval:   20,
		MaxNotesLength:       4096,
		BackgroundExtraction: true,
	}
}

func resolveConfig(cfg SessionMemoryConfig) SessionMemoryConfig {
	d := defaultConfig()
	if cfg.ExtractionInterval == 0 {
		cfg.ExtractionInterval = d.ExtractionInterval
	}
	if cfg.MaxNotesLength == 0 {
		cfg.MaxNotesLength = d.MaxNotesLength
	}
	return cfg
}

type SessionNote struct {
	ID        string    `json:"id"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	Source    string    `json:"source"`
}

type SessionMemory struct {
	mu                          sync.Mutex
	sessionID                   string
	notesPath                   string
	notes                       []SessionNote
	messageCountSinceExtraction int
	config                      SessionMemoryConfig
}

func NewSessionMemory(sessionID, basePath string, config SessionMemoryConfig) *SessionMemory {
	cfg := resolveConfig(config)
	notesDir := filepath.Join(basePath, "session-memory", sessionID)
	_ = os.MkdirAll(notesDir, 0o700)
	notesPath := filepath.Join(notesDir, "notes.md")
	sm := &SessionMemory{
		sessionID: sessionID,
		notesPath: notesPath,
		config:    cfg,
	}
	_ = sm.LoadFromFile()
	return sm
}

func (sm *SessionMemory) GetSessionNotes() string {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return sm.buildMarkdownLocked()
}

func (sm *SessionMemory) AddNote(content, source string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if len(sm.notes) >= sm.config.MaxNotesLength/64 {
		sm.notes = sm.notes[len(sm.notes)-sm.config.MaxNotesLength/64:]
	}
	sm.notes = append(sm.notes, SessionNote{
		ID:        fmt.Sprintf("note-%d-%d", time.Now().UnixNano(), len(sm.notes)),
		Content:   content,
		CreatedAt: time.Now(),
		Source:    source,
	})
}

func (sm *SessionMemory) OnPostSampling(messageCount int) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.messageCountSinceExtraction++
	if sm.config.ExtractionInterval > 0 && sm.messageCountSinceExtraction >= sm.config.ExtractionInterval {
		sm.messageCountSinceExtraction = 0
	}
}

func (sm *SessionMemory) ExtractNotes(ctx context.Context, conversationSummary string) ([]SessionNote, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if conversationSummary == "" {
		return nil, nil
	}

	lines := strings.Split(conversationSummary, "\n")
	var notes []SessionNote
	var current strings.Builder

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if current.Len() > 0 {
				content := strings.TrimSpace(current.String())
				if content != "" {
					notes = append(notes, SessionNote{
						ID:        fmt.Sprintf("note-%d-%d", time.Now().UnixNano(), len(notes)),
						Content:   content,
						CreatedAt: time.Now(),
						Source:    "observation",
					})
				}
				current.Reset()
			}
			continue
		}
		current.WriteString(line)
		current.WriteString("\n")
	}
	if current.Len() > 0 {
		content := strings.TrimSpace(current.String())
		if content != "" {
			notes = append(notes, SessionNote{
				ID:        fmt.Sprintf("note-%d-%d", time.Now().UnixNano(), len(notes)),
				Content:   content,
				CreatedAt: time.Now(),
				Source:    "observation",
			})
		}
	}

	if len(notes) > 0 {
		sm.notes = append(sm.notes, notes...)
		if len(sm.notes) > sm.config.MaxNotesLength/64 {
			sm.notes = sm.notes[len(sm.notes)-sm.config.MaxNotesLength/64:]
		}
	}

	return notes, ctx.Err()
}

func (sm *SessionMemory) SaveToFile() error {
	sm.mu.Lock()
	data := sm.buildMarkdownLocked()
	sm.mu.Unlock()

	dir := filepath.Dir(sm.notesPath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create session memory dir: %w", err)
	}
	return os.WriteFile(sm.notesPath, []byte(data), 0o600)
}

func (sm *SessionMemory) LoadFromFile() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	data, err := os.ReadFile(sm.notesPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read session memory file: %w", err)
	}

	content := string(data)
	sm.notes = parseNotesFromMarkdown(content)
	return nil
}

func (sm *SessionMemory) Reset() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.notes = nil
	sm.messageCountSinceExtraction = 0
}

func (sm *SessionMemory) NotesMarkdown() string {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return sm.buildMarkdownLocked()
}

func (sm *SessionMemory) buildMarkdownLocked() string {
	var sb strings.Builder

	sb.WriteString("---\n")
	sb.WriteString("session_id: ")
	sb.WriteString(sm.sessionID)
	sb.WriteString("\n")
	sb.WriteString("notes_count: ")
	fmt.Fprintf(&sb, "%d", len(sm.notes))
	sb.WriteString("\n")
	sb.WriteString("updated_at: ")
	sb.WriteString(time.Now().UTC().Format(time.RFC3339))
	sb.WriteString("\n---\n\n")

	for _, n := range sm.notes {
		sb.WriteString("## ")
		sb.WriteString(n.Source)
		sb.WriteString(" — ")
		sb.WriteString(n.CreatedAt.Format("2006-01-02 15:04:05"))
		sb.WriteString("\n\n")
		sb.WriteString(n.Content)
		sb.WriteString("\n\n")
	}

	return sb.String()
}

var (
	frontmatterBlock = regexp.MustCompile(`(?s)^---\n(.*?)\n---\n`)
	noteBlock        = regexp.MustCompile(`(?s)^## (\S+) — (\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2})\n\n(.*?)(\n## |\z)`)
)

func parseNotesFromMarkdown(content string) []SessionNote {
	remaining := content
	if m := frontmatterBlock.FindStringIndex(remaining); m != nil {
		remaining = remaining[m[1]:]
	}

	matches := noteBlock.FindAllStringSubmatch(remaining, -1)
	notes := make([]SessionNote, 0, len(matches))
	for i, m := range matches {
		source := m[1]
		ts, err := time.Parse("2006-01-02 15:04:05", m[2])
		if err != nil {
			ts = time.Now()
		}
		notes = append(notes, SessionNote{
			ID:        fmt.Sprintf("loaded-%d", i),
			Content:   strings.TrimSpace(m[3]),
			CreatedAt: ts,
			Source:    source,
		})
	}
	return notes
}
