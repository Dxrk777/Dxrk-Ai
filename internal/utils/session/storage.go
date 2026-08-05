package session

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Storage defines the persistence contract for sessions.
type Storage interface {
	Save(session *Session) error
	Load(id string) (*Session, error)
	Delete(id string) error
	List(opts ListOpts) ([]SessionSummary, error)
	Exists(id string) (bool, error)
}

// SessionSummary is a lightweight view of a session for listing.
type SessionSummary struct {
	ID           string        `json:"id"`
	Title        string        `json:"title"`
	CreatedAt    time.Time     `json:"created_at"`
	MessageCount int           `json:"message_count"`
	TokenCount   int           `json:"token_count"`
	Status       SessionStatus `json:"status"`
}

// ListOpts controls the List query.
type ListOpts struct {
	Limit       int
	Offset      int
	Status      SessionStatus // -1 = any
	SortBy      string
	SortDir     string
	After       *time.Time
	Before      *time.Time
	SearchQuery string
}

type indexEntry struct {
	ID           string        `json:"id"`
	Title        string        `json:"title"`
	CreatedAt    time.Time     `json:"created_at"`
	UpdatedAt    time.Time     `json:"updated_at"`
	MessageCount int           `json:"message_count"`
	TokenCount   int           `json:"token_count"`
	Status       SessionStatus `json:"status"`
	Compressed   bool          `json:"compressed,omitempty"`
}

// FileStorage persists sessions as JSON files on disk with an index.
type FileStorage struct {
	baseDir string
	mu      sync.RWMutex
	index   map[string]indexEntry
}

// NewFileStorage creates a FileStorage rooted at baseDir. Empty defaults to ~/.dxrk/sessions/.
func NewFileStorage(baseDir string) *FileStorage {
	if baseDir == "" {
		home, _ := os.UserHomeDir()
		baseDir = filepath.Join(home, ".dxrk", "sessions")
	}
	fs := &FileStorage{baseDir: baseDir, index: make(map[string]indexEntry)}
	_ = os.MkdirAll(baseDir, 0o700)
	fs.loadIndex()
	return fs
}

func (fs *FileStorage) sessionPath(id string) string { return filepath.Join(fs.baseDir, id+".json") }
func (fs *FileStorage) compressedPath(id string) string {
	return filepath.Join(fs.baseDir, id+".json.gz")
}
func (fs *FileStorage) indexPath() string { return filepath.Join(fs.baseDir, ".index.json") }

// Save persists a session to disk with atomic write and updates the index.
func (fs *FileStorage) Save(s *Session) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	s.UpdatedAt = time.Now()
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}
	target := fs.sessionPath(s.ID)
	tmp := target + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("write temp: %w", err)
	}
	if err := os.Rename(tmp, target); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("atomic rename: %w", err)
	}
	fs.index[s.ID] = indexEntry{ID: s.ID, Title: s.Title, CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt,
		MessageCount: s.MessageCount, TokenCount: s.TokenCount, Status: s.Status}
	return fs.writeIndex()
}

// Load reads a session from disk by ID, falling back to gzipped files.
func (fs *FileStorage) Load(id string) (*Session, error) {
	fs.mu.RLock()
	_ = fs.index[id]
	fs.mu.RUnlock()

	path := fs.sessionPath(id)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			data, err = readGzFile(fs.compressedPath(id))
			if err != nil {
				return nil, fmt.Errorf("session %q not found", id)
			}
		} else {
			return nil, fmt.Errorf("read session: %w", err)
		}
	}
	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("unmarshal session: %w", err)
	}
	return &s, nil
}

func (fs *FileStorage) Delete(id string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	_ = os.Remove(fs.sessionPath(id))
	_ = os.Remove(fs.compressedPath(id))
	delete(fs.index, id)
	return fs.writeIndex()
}

func (fs *FileStorage) Exists(id string) (bool, error) {
	fs.mu.RLock()
	_, ok := fs.index[id]
	fs.mu.RUnlock()
	if ok {
		return true, nil
	}
	_, err := os.Stat(fs.sessionPath(id))
	if err == nil {
		return true, nil
	}
	_, err = os.Stat(fs.compressedPath(id))
	return err == nil, nil
}

// List returns sessions matching the given filters.
func (fs *FileStorage) List(opts ListOpts) ([]SessionSummary, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	var entries []indexEntry
	for _, e := range fs.index {
		if opts.Status >= 0 && e.Status != opts.Status {
			continue
		}
		if opts.After != nil && e.CreatedAt.Before(*opts.After) {
			continue
		}
		if opts.Before != nil && e.CreatedAt.After(*opts.Before) {
			continue
		}
		if opts.SearchQuery != "" && !strings.Contains(strings.ToLower(e.Title), strings.ToLower(opts.SearchQuery)) {
			continue
		}
		entries = append(entries, e)
	}

	asc := opts.SortDir == "asc"
	sort.Slice(entries, func(i, j int) bool {
		switch opts.SortBy {
		case "token_count":
			return cmpInt(entries[i].TokenCount, entries[j].TokenCount, asc)
		case "message_count":
			return cmpInt(entries[i].MessageCount, entries[j].MessageCount, asc)
		case "updated_at":
			return cmpTime(entries[i].UpdatedAt, entries[j].UpdatedAt, asc)
		default:
			return cmpTime(entries[i].CreatedAt, entries[j].CreatedAt, asc)
		}
	})

	if opts.Offset > 0 {
		if opts.Offset >= len(entries) {
			return nil, nil
		}
		entries = entries[opts.Offset:]
	}
	if opts.Limit > 0 && opts.Limit < len(entries) {
		entries = entries[:opts.Limit]
	}

	result := make([]SessionSummary, len(entries))
	for i, e := range entries {
		result[i] = SessionSummary{ID: e.ID, Title: e.Title, CreatedAt: e.CreatedAt,
			MessageCount: e.MessageCount, TokenCount: e.TokenCount, Status: e.Status}
	}
	return result, nil
}

func cmpInt(a, b int, asc bool) bool {
	if asc {
		return a < b
	}
	return a > b
}
func cmpTime(a, b time.Time, asc bool) bool {
	if asc {
		return a.Before(b)
	}
	return a.After(b)
}

func (fs *FileStorage) loadIndex() {
	data, err := os.ReadFile(fs.indexPath())
	if err != nil {
		return
	}
	var entries []indexEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return
	}
	fs.index = make(map[string]indexEntry, len(entries))
	for _, e := range entries {
		fs.index[e.ID] = e
	}
}

func (fs *FileStorage) writeIndex() error {
	entries := make([]indexEntry, 0, len(fs.index))
	for _, e := range fs.index {
		entries = append(entries, e)
	}
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(fs.indexPath(), data, 0o600)
}

func (fs *FileStorage) CompressSession(id string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	src := fs.sessionPath(id)
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	dst, err := os.Create(fs.compressedPath(id))
	if err != nil {
		return err
	}
	defer func() { _ = dst.Close() }()
	gz := gzip.NewWriter(dst)
	if _, err := io.Copy(gz, f); err != nil {
		_ = gz.Close()
		return err
	}
	if err := gz.Close(); err != nil {
		return err
	}
	_ = os.Remove(src)
	if e, ok := fs.index[id]; ok {
		e.Compressed = true
		fs.index[id] = e
		_ = fs.writeIndex()
	}
	return nil
}

func readGzFile(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer func() { _ = gz.Close() }()
	return io.ReadAll(gz)
}

// MemoryStorage keeps sessions in memory. Pass 0 for unlimited capacity.
type MemoryStorage struct {
	mu          sync.RWMutex
	sessions    map[string]*Session
	order       []string
	maxSessions int
}

func NewMemoryStorage(maxSessions int) *MemoryStorage {
	return &MemoryStorage{sessions: make(map[string]*Session), maxSessions: maxSessions}
}

func (ms *MemoryStorage) Save(s *Session) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	if _, exists := ms.sessions[s.ID]; !exists {
		ms.order = append(ms.order, s.ID)
	}
	ms.sessions[s.ID] = s
	for ms.maxSessions > 0 && len(ms.order) > ms.maxSessions {
		ms.order = ms.order[1:]
		delete(ms.sessions, ms.order[0])
	}
	return nil
}

func (ms *MemoryStorage) Load(id string) (*Session, error) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	s, ok := ms.sessions[id]
	if !ok {
		return nil, fmt.Errorf("session %q not found", id)
	}
	cp := *s
	cp.Messages = make([]Message, len(s.Messages))
	copy(cp.Messages, s.Messages)
	return &cp, nil
}

func (ms *MemoryStorage) Delete(id string) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	if _, ok := ms.sessions[id]; !ok {
		return fmt.Errorf("session %q not found", id)
	}
	delete(ms.sessions, id)
	for i, oid := range ms.order {
		if oid == id {
			ms.order = append(ms.order[:i], ms.order[i+1:]...)
			break
		}
	}
	return nil
}

func (ms *MemoryStorage) List(opts ListOpts) ([]SessionSummary, error) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	var result []SessionSummary
	for _, s := range ms.sessions {
		if opts.Status >= 0 && s.Status != opts.Status {
			continue
		}
		if opts.SearchQuery != "" && !strings.Contains(strings.ToLower(s.Title), strings.ToLower(opts.SearchQuery)) {
			continue
		}
		result = append(result, SessionSummary{ID: s.ID, Title: s.Title, CreatedAt: s.CreatedAt,
			MessageCount: s.MessageCount, TokenCount: s.TokenCount, Status: s.Status})
	}
	sort.Slice(result, func(i, j int) bool {
		switch opts.SortBy {
		case "token_count":
			return result[i].TokenCount > result[j].TokenCount
		case "message_count":
			return result[i].MessageCount > result[j].MessageCount
		default:
			return result[i].CreatedAt.After(result[j].CreatedAt)
		}
	})
	if opts.Offset > 0 && opts.Offset < len(result) {
		result = result[opts.Offset:]
	}
	if opts.Limit > 0 && opts.Limit < len(result) {
		result = result[:opts.Limit]
	}
	return result, nil
}

func (ms *MemoryStorage) Exists(id string) (bool, error) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	_, ok := ms.sessions[id]
	return ok, nil
}
