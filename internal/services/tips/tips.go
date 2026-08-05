package tips

import (
	"math/rand"
	"sort"
	"sync"
	"time"
)

// Tip represents a single tip with metadata.
type Tip struct {
	ID         string
	Category   string
	Title      string
	Content    string
	Trigger    Trigger
	Frequency  Frequency
	Priority   int
	Enabled    bool
	ShownCount int
	LastShown  time.Time
	Tags       []string
}

type Frequency int

const (
	FrequencyOnce   Frequency = iota // Show only once per session
	FrequencyDaily                   // Show once per day
	FrequencyWeekly                  // Show once per week
	FrequencyAlways                  // Show every time triggered
)

type Trigger int

const (
	TriggerOnStart    Trigger = iota // Show when session starts
	TriggerOnIdle                    // Show when user is idle
	TriggerOnTask                    // Show after task completion
	TriggerOnError                   // Show after an error
	TriggerOnToolUse                 // Show after specific tool use
	TriggerOnFirstUse                // Show on first use of a feature
)

// TipsStats holds statistics about tip usage.
type TipsStats struct {
	Total             int
	Enabled           int
	Disabled          int
	ShownThisSession  int
	TotalShownAllTime int
	ByCategory        map[string]int
}

// TipsEngine manages tip delivery.
type TipsEngine struct {
	mu           sync.RWMutex
	tips         map[string]*Tip
	shown        map[string]time.Time // tipID -> last shown time
	sessionShown map[string]bool      // tips shown this session
	config       TipsConfig
	sessionStart time.Time
}

// TipsConfig configures the tips engine behavior.
type TipsConfig struct {
	Enabled       bool
	MaxPerSession int
	MinInterval   time.Duration
	ShuffleTips   bool
}

// NewEngine creates a new TipsEngine with the given configuration.
// Zero-value fields are applied as-is; use the defaults below as reference.
//
//	MaxPerSession: 5
//	MinInterval:   0 (no throttle)
//	ShuffleTips:   true
//	Enabled:       true
func NewEngine(cfg TipsConfig) *TipsEngine {
	c := TipsConfig{
		Enabled:       true,
		MaxPerSession: 5,
		MinInterval:   0,
		ShuffleTips:   true,
	}
	// Apply user config on top of defaults.
	if cfg.MaxPerSession != 0 {
		c.MaxPerSession = cfg.MaxPerSession
	}
	c.MinInterval = cfg.MinInterval
	c.Enabled = cfg.Enabled
	c.ShuffleTips = cfg.ShuffleTips
	return &TipsEngine{
		tips:         make(map[string]*Tip),
		shown:        make(map[string]time.Time),
		sessionShown: make(map[string]bool),
		config:       c,
		sessionStart: time.Now(),
	}
}

// Register adds a tip to the engine.
func (e *TipsEngine) Register(tip Tip) {
	e.mu.Lock()
	defer e.mu.Unlock()
	stored := tip
	e.tips[tip.ID] = &stored
}

// RegisterBatch adds multiple tips.
func (e *TipsEngine) RegisterBatch(tips []Tip) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, tip := range tips {
		stored := tip
		e.tips[tip.ID] = &stored
	}
}

// GetNext returns the next tip to show, or nil if none.
// It respects frequency rules, max-per-session, and min-interval.
func (e *TipsEngine) GetNext() *Tip {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.config.Enabled {
		return nil
	}

	if len(e.sessionShown) >= e.config.MaxPerSession {
		return nil
	}

	now := time.Now()
	candidates := make([]*Tip, 0)

	for _, tip := range e.tips {
		if !tip.Enabled {
			continue
		}
		if !e.canShowTip(tip, now) {
			continue
		}
		candidates = append(candidates, tip)
	}

	if len(candidates) == 0 {
		return nil
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Priority > candidates[j].Priority
	})

	if e.config.ShuffleTips && len(candidates) > 1 {
		// Shuffle top candidates with same priority
		topPriority := candidates[0].Priority
		samePriority := 0
		for _, c := range candidates {
			if c.Priority == topPriority {
				samePriority++
			} else {
				break
			}
		}
		if samePriority > 1 {
			rand.Shuffle(samePriority, func(i, j int) {
				candidates[i], candidates[j] = candidates[j], candidates[i]
			})
		}
	}

	return candidates[0]
}

func (e *TipsEngine) canShowTip(tip *Tip, now time.Time) bool {
	// Check min interval since last shown
	if lastShown, ok := e.shown[tip.ID]; ok {
		if now.Sub(lastShown) < e.config.MinInterval {
			return false
		}
	}

	switch tip.Frequency {
	case FrequencyOnce:
		return !e.sessionShown[tip.ID]
	case FrequencyDaily:
		if lastShown, ok := e.shown[tip.ID]; ok {
			return now.Sub(lastShown) >= 24*time.Hour
		}
		return true
	case FrequencyWeekly:
		if lastShown, ok := e.shown[tip.ID]; ok {
			return now.Sub(lastShown) >= 7*24*time.Hour
		}
		return true
	case FrequencyAlways:
		return true
	default:
		return true
	}
}

// GetByTrigger returns tips matching a specific trigger.
func (e *TipsEngine) GetByTrigger(trigger Trigger) []*Tip {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*Tip, 0)
	now := time.Now()
	for _, tip := range e.tips {
		if !tip.Enabled {
			continue
		}
		if tip.Trigger == trigger && e.canShowTip(tip, now) {
			result = append(result, tip)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Priority > result[j].Priority
	})
	return result
}

// GetByCategory returns tips in a category.
func (e *TipsEngine) GetByCategory(category string) []*Tip {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*Tip, 0)
	for _, tip := range e.tips {
		if tip.Category == category {
			result = append(result, tip)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Priority > result[j].Priority
	})
	return result
}

// MarkShown marks a tip as shown.
func (e *TipsEngine) MarkShown(tipID string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	tip, ok := e.tips[tipID]
	if !ok {
		return
	}
	now := time.Now()
	tip.ShownCount++
	tip.LastShown = now
	e.shown[tipID] = now
	e.sessionShown[tipID] = true
}

// ResetSession clears session-shown state.
func (e *TipsEngine) ResetSession() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.sessionShown = make(map[string]bool)
	e.sessionStart = time.Now()
}

// Enable enables a tip by ID.
func (e *TipsEngine) Enable(tipID string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	tip, ok := e.tips[tipID]
	if !ok {
		return false
	}
	tip.Enabled = true
	return true
}

// Disable disables a tip by ID.
func (e *TipsEngine) Disable(tipID string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	tip, ok := e.tips[tipID]
	if !ok {
		return false
	}
	tip.Enabled = false
	return true
}

// EnableCategory enables all tips in a category.
func (e *TipsEngine) EnableCategory(category string) int {
	e.mu.Lock()
	defer e.mu.Unlock()
	count := 0
	for _, tip := range e.tips {
		if tip.Category == category && !tip.Enabled {
			tip.Enabled = true
			count++
		}
	}
	return count
}

// DisableCategory disables all tips in a category.
func (e *TipsEngine) DisableCategory(category string) int {
	e.mu.Lock()
	defer e.mu.Unlock()
	count := 0
	for _, tip := range e.tips {
		if tip.Category == category && tip.Enabled {
			tip.Enabled = false
			count++
		}
	}
	return count
}

// Stats returns tip statistics.
func (e *TipsEngine) Stats() TipsStats {
	e.mu.RLock()
	defer e.mu.RUnlock()

	stats := TipsStats{
		Total:            len(e.tips),
		ShownThisSession: len(e.sessionShown),
		ByCategory:       make(map[string]int),
	}

	for _, tip := range e.tips {
		if tip.Enabled {
			stats.Enabled++
		} else {
			stats.Disabled++
		}
		stats.TotalShownAllTime += tip.ShownCount
		stats.ByCategory[tip.Category]++
	}

	return stats
}
