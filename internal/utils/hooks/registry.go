package hooks

import (
	"context"
	"errors"
	"path/filepath"
	"regexp"
	"sync"
	"time"
)

var (
	ErrHookNotFound    = errors.New("hooks: hook not found")
	ErrHookDisabled    = errors.New("hooks: hook is disabled")
	ErrInvalidConfig   = errors.New("hooks: invalid configuration")
	ErrRegistryClosed  = errors.New("hooks: registry is closed")
	ErrDuplicateHookID = errors.New("hooks: duplicate hook ID")
)

// HookRegistry manages hook registrations and lookups.
type HookRegistry struct {
	mu       sync.RWMutex
	hooks    map[string]*HookConfig
	byType   map[HookType][]string
	closed   bool
	watchers []chan struct{}
}

// NewHookRegistry creates a new hook registry.
func NewHookRegistry() *HookRegistry {
	return &HookRegistry{
		hooks:  make(map[string]*HookConfig),
		byType: make(map[HookType][]string),
	}
}

// Register adds a hook to the registry. Returns ErrDuplicateHookID if ID exists.
func (r *HookRegistry) Register(config HookConfig) error {
	if config.ID == "" {
		return ErrInvalidConfig
	}
	if config.Command == "" {
		return ErrInvalidConfig
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return ErrRegistryClosed
	}

	if _, exists := r.hooks[config.ID]; exists {
		return ErrDuplicateHookID
	}

	cfg := config
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	if cfg.MaxRetries < 0 {
		cfg.MaxRetries = 0
	}
	if cfg.RetryDelay == 0 {
		cfg.RetryDelay = 1 * time.Second
	}

	r.hooks[cfg.ID] = &cfg
	r.byType[cfg.Type] = append(r.byType[cfg.Type], cfg.ID)

	r.notifyWatchers()
	return nil
}

// Unregister removes a hook by ID.
func (r *HookRegistry) Unregister(id string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return false
	}

	cfg, exists := r.hooks[id]
	if !exists {
		return false
	}

	delete(r.hooks, id)
	r.removeFromTypeIndex(cfg.Type, id)
	r.notifyWatchers()
	return true
}

// Get retrieves a hook by ID.
func (r *HookRegistry) Get(id string) (*HookConfig, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	cfg, ok := r.hooks[id]
	return cfg, ok
}

// GetByType returns all hook IDs for a given type.
func (r *HookRegistry) GetByType(ht HookType) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := make([]string, len(r.byType[ht]))
	copy(ids, r.byType[ht])
	return ids
}

// List returns all registered hooks.
func (r *HookRegistry) List() []*HookConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*HookConfig, 0, len(r.hooks))
	for _, cfg := range r.hooks {
		result = append(result, cfg)
	}
	return result
}

// Match returns hooks matching the given event.
func (r *HookRegistry) Match(event HookEvent) []*HookConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.closed {
		return nil
	}

	var matched []*HookConfig
	for _, id := range r.byType[event.Type] {
		if cfg, ok := r.hooks[id]; ok && cfg.Enabled {
			if matchesHook(cfg, event) {
				matched = append(matched, cfg)
			}
		}
	}
	return matched
}

// Watch returns a channel that receives notifications on registry changes.
func (r *HookRegistry) Watch(ctx context.Context) <-chan struct{} {
	ch := make(chan struct{}, 1)
	r.mu.Lock()
	r.watchers = append(r.watchers, ch)
	r.mu.Unlock()

	go func() {
		<-ctx.Done()
		r.mu.Lock()
		for i, w := range r.watchers {
			if w == ch {
				r.watchers = append(r.watchers[:i], r.watchers[i+1:]...)
				break
			}
		}
		r.mu.Unlock()
		close(ch)
	}()

	return ch
}

// Close marks the registry as closed, preventing further modifications.
func (r *HookRegistry) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.closed = true
	for _, w := range r.watchers {
		close(w)
	}
	r.watchers = nil
}

func (r *HookRegistry) removeFromTypeIndex(ht HookType, id string) {
	ids := r.byType[ht]
	for i, v := range ids {
		if v == id {
			r.byType[ht] = append(ids[:i], ids[i+1:]...)
			break
		}
	}
}

func (r *HookRegistry) notifyWatchers() {
	for _, w := range r.watchers {
		select {
		case w <- struct{}{}:
		default:
		}
	}
}

func matchesHook(cfg *HookConfig, event HookEvent) bool {
	m := cfg.Match

	if m.ToolName != "" && m.ToolName != event.ToolName {
		return false
	}
	if len(m.ToolNames) > 0 {
		found := false
		for _, n := range m.ToolNames {
			if n == event.ToolName {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	if m.Glob != "" {
		matched, _ := filepath.Match(m.Glob, event.ToolName)
		if !matched {
			return false
		}
	}

	if m.Regex != "" {
		re, err := regexp.Compile(m.Regex)
		if err != nil {
			return false
		}
		if !re.MatchString(event.ToolName) {
			return false
		}
	}

	if m.Command != "" && m.Command != event.ToolName {
		return false
	}
	if len(m.Commands) > 0 {
		found := false
		for _, c := range m.Commands {
			if c == event.ToolName {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}
