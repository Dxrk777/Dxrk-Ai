package autodream

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"
)

type DreamConfig struct {
	Enabled         bool
	TimeGapMinutes  int
	SessionCountGap int
	MaxLockAttempts int
	LockPath        string
}

func defaultConfig() DreamConfig {
	return DreamConfig{
		Enabled:         true,
		TimeGapMinutes:  60,
		SessionCountGap: 5,
		MaxLockAttempts: 3,
		LockPath:        ".consolidate-lock",
	}
}

type DreamState struct {
	mu            sync.Mutex
	lastDreamTime time.Time
	sessionCount  int
	locked        bool
}

type DreamResult struct {
	Success           bool
	Consolidated      bool
	MemoriesExtracted []string
	Duration          time.Duration
	Error             error
}

type AutoDreamService struct {
	config DreamConfig
	state  DreamState
}

func NewAutoDreamService(config DreamConfig) *AutoDreamService {
	cfg := defaultConfig()
	if config.TimeGapMinutes > 0 {
		cfg.TimeGapMinutes = config.TimeGapMinutes
	}
	if config.SessionCountGap > 0 {
		cfg.SessionCountGap = config.SessionCountGap
	}
	if config.MaxLockAttempts > 0 {
		cfg.MaxLockAttempts = config.MaxLockAttempts
	}
	if config.LockPath != "" {
		cfg.LockPath = config.LockPath
	}
	cfg.Enabled = config.Enabled

	lastTime := readLockMtime(cfg.LockPath)

	return &AutoDreamService{
		config: cfg,
		state: DreamState{
			lastDreamTime: lastTime,
		},
	}
}

func (s *AutoDreamService) ShouldDream() bool {
	s.state.mu.Lock()
	defer s.state.mu.Unlock()

	if !s.config.Enabled {
		return false
	}

	if s.state.locked {
		return false
	}

	gap := time.Duration(s.config.TimeGapMinutes) * time.Minute
	if time.Since(s.state.lastDreamTime) < gap {
		return false
	}

	if s.state.sessionCount < s.config.SessionCountGap {
		return false
	}

	return true
}

func (s *AutoDreamService) TryLock() bool {
	s.state.mu.Lock()
	defer s.state.mu.Unlock()

	if s.state.locked {
		return false
	}

	for attempt := 0; attempt < s.config.MaxLockAttempts; attempt++ {
		if tryAcquireLock(s.config.LockPath) {
			s.state.locked = true
			s.state.sessionCount++
			return true
		}
		time.Sleep(time.Duration(attempt+1) * 50 * time.Millisecond)
	}

	return false
}

func (s *AutoDreamService) ReleaseLock() {
	s.state.mu.Lock()
	defer s.state.mu.Unlock()

	if !s.state.locked {
		return
	}

	releaseFileLock(s.config.LockPath)
	s.state.locked = false
	s.state.lastDreamTime = time.Now()
}

func (s *AutoDreamService) Dream(ctx context.Context) (*DreamResult, error) {
	start := time.Now()

	if !s.TryLock() {
		return &DreamResult{
			Success:  false,
			Duration: time.Since(start),
			Error:    fmt.Errorf("could not acquire lock"),
		}, nil
	}

	defer s.ReleaseLock()

	if err := ctx.Err(); err != nil {
		return &DreamResult{
			Success:  false,
			Duration: time.Since(start),
			Error:    err,
		}, nil
	}

	_ = s.BuildConsolidationPrompt()

	select {
	case <-ctx.Done():
		return &DreamResult{
			Success:  false,
			Duration: time.Since(start),
			Error:    ctx.Err(),
		}, nil
	default:
	}

	return &DreamResult{
		Success:           true,
		Consolidated:      true,
		MemoriesExtracted: []string{},
		Duration:          time.Since(start),
		Error:             nil,
	}, nil
}

func (s *AutoDreamService) BuildConsolidationPrompt() string {
	return `# Dream: Memory Consolidation

You are performing a dream — a reflective pass over your memory files. Synthesize what you've learned recently into durable, well-organized memories so that future sessions can orient quickly.

---

## Phase 1 — Orient

- List the memory directory to see what already exists
- Read the entrypoint index to understand the current structure
- Skim existing topic files so you improve them rather than creating duplicates
- If logs/ or sessions/ subdirectories exist, review recent entries there

## Phase 2 — Gather recent signal

Look for new information worth persisting. Sources in rough priority order:

1. Daily logs (logs/YYYY/MM/YYYY-MM-DD.md) if present — these are the append-only stream
2. Existing memories that drifted — facts that contradict something you see in the codebase now
3. Transcript search — grep the JSONL transcripts for narrow terms if you need specific context

Don't exhaustively read transcripts. Look only for things you already suspect matter.

## Phase 3 — Consolidate

For each thing worth remembering, write or update a memory file at the top level of the memory directory.

Focus on:
- Merging new signal into existing topic files rather than creating near-duplicates
- Converting relative dates ("yesterday", "last week") to absolute dates so they remain interpretable after time passes
- Deleting contradicted facts — if today's investigation disproves an old memory, fix it at the source

## Phase 4 — Prune and index

Update the entrypoint index so it stays compact. It's an index, not a dump — each entry should be one line under ~150 characters.

- Remove pointers to memories that are now stale, wrong, or superseded
- Demote verbose entries: if an index line is over ~200 chars, move the detail to the topic file
- Add pointers to newly important memories
- Resolve contradictions — if two files disagree, fix the wrong one

---

Return a brief summary of what you consolidated, updated, or pruned. If nothing changed (memories are already tight), say so.`
}

func (s *AutoDreamService) IncrementSession() {
	s.state.mu.Lock()
	defer s.state.mu.Unlock()

	s.state.sessionCount++

	gap := time.Duration(s.config.TimeGapMinutes) * time.Minute
	if s.state.sessionCount >= s.config.SessionCountGap && time.Since(s.state.lastDreamTime) >= gap {
		s.state.lastDreamTime = time.Now()
		s.state.sessionCount = 0
	}
}

func readLockMtime(path string) time.Time {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}

func tryAcquireLock(path string) bool {
	flag := os.O_CREATE | os.O_EXCL | os.O_WRONLY
	f, err := os.OpenFile(path, flag, 0644)
	if err != nil {
		return false
	}

	pid := os.Getpid()
	_, writeErr := fmt.Fprintf(f, "%d", pid)
	_ = f.Close()

	if writeErr != nil {
		_ = os.Remove(path)
		return false
	}

	data, readErr := os.ReadFile(path)
	if readErr != nil {
		return false
	}

	var storedPid int
	if _, scanErr := fmt.Sscanf(string(data), "%d", &storedPid); scanErr != nil {
		return false
	}

	return storedPid == pid
}

func releaseFileLock(path string) {
	_ = os.Remove(path)
}
