package coordinator

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Dxrk777/Dxrk-Ai/internal/strconst"
)

// CoordinatorMode represents the operating mode for multi-agent orchestration.
type CoordinatorMode int

const (
	// ModeSingleAgent runs a single worker (default).
	ModeSingleAgent CoordinatorMode = iota
	// ModeCoordinator enables multi-agent orchestration with teams.
	ModeCoordinator
	// ModeWorker runs as a worker agent under a coordinator.
	ModeWorker
)

func (m CoordinatorMode) String() string {
	switch m {
	case ModeSingleAgent:
		return "single_agent"
	case ModeCoordinator:
		return "coordinator"
	case ModeWorker:
		return "worker"
	default:
		return strconst.StrUnknown
	}
}

// ScratchpadEntry represents a shared context item between agents.
type ScratchpadEntry struct {
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	SetBy     string    `json:"set_by"` // agent name that set this
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CoordinatorConfig holds configuration for the multi-agent system.
type CoordinatorConfig struct {
	Mode            CoordinatorMode
	MaxWorkers      int
	ScratchpadLimit int
	AgentTimeout    time.Duration
	ContextWindow   int // max tokens for injected context
}

// DefaultCoordinatorConfig returns sensible defaults.
func DefaultCoordinatorConfig() CoordinatorConfig {
	return CoordinatorConfig{
		Mode:            ModeSingleAgent,
		MaxWorkers:      8,
		ScratchpadLimit: 100,
		AgentTimeout:    30 * time.Minute,
		ContextWindow:   128000,
	}
}

// AgentStatus represents the current state of a worker agent.
type AgentStatus int

const (
	AgentIdle AgentStatus = iota
	AgentBusy
	AgentFailed
	AgentDone
)

func (s AgentStatus) String() string {
	switch s {
	case AgentIdle:
		return "idle"
	case AgentBusy:
		return "busy"
	case AgentFailed:
		return strconst.StrFailed
	case AgentDone:
		return "done"
	default:
		return strconst.StrUnknown
	}
}

// AgentResult is the output from a worker agent.
type AgentResult struct {
	AgentID  string `json:"agent_id"`
	Output   string `json:"output"`
	Error    string `json:"error,omitempty"`
	Duration time.Duration
}

// Coordinator manages multi-agent orchestration.
type Coordinator struct {
	mu          sync.RWMutex
	config      CoordinatorConfig
	teams       map[string]*Team
	scratchpad  map[string]*ScratchpadEntry
	workers     map[string]*Worker
	agentCtx    context.Context
	agentCancel context.CancelFunc
}

// NewCoordinator creates a new coordinator with the given config.
func NewCoordinator(config CoordinatorConfig) *Coordinator {
	ctx, cancel := context.WithCancel(context.Background())
	return &Coordinator{
		config:      config,
		teams:       make(map[string]*Team),
		scratchpad:  make(map[string]*ScratchpadEntry),
		workers:     make(map[string]*Worker),
		agentCtx:    ctx,
		agentCancel: cancel,
	}
}

// CreateTeam creates a new agent team with the given name and members.
func (c *Coordinator) CreateTeam(name string, members []string) (*Team, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.teams[name]; exists {
		return nil, fmt.Errorf("team %q already exists", name)
	}

	if len(members) == 0 {
		return nil, fmt.Errorf("team must have at least one member")
	}

	team := NewTeam(name, members)
	c.teams[name] = team
	return team, nil
}

// DeleteTeam removes a team by name.
func (c *Coordinator) DeleteTeam(name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	team, exists := c.teams[name]
	if !exists {
		return fmt.Errorf("team %q not found", name)
	}

	// Mark all members as done
	team.mu.Lock()
	for _, member := range team.Members {
		member.Status = AgentDone
	}
	team.mu.Unlock()

	delete(c.teams, name)
	return nil
}

// SendMessage sends a message from one agent to another within a team.
func (c *Coordinator) SendMessage(teamName, fromAgent, toAgent, message string) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	team, exists := c.teams[teamName]
	if !exists {
		return fmt.Errorf("team %q not found", teamName)
	}

	return team.RouteMessage(fromAgent, toAgent, message)
}

// BroadcastMessage sends a message from one agent to all other team members.
func (c *Coordinator) BroadcastMessage(teamName, fromAgent, message string) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	team, exists := c.teams[teamName]
	if !exists {
		return fmt.Errorf("team %q not found", teamName)
	}

	return team.Broadcast(fromAgent, message)
}

// SetScratchpad sets a value in the shared scratchpad.
func (c *Coordinator) SetScratchpad(key, value, setBy string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.scratchpad) >= c.config.ScratchpadLimit {
		// Evict oldest entry
		var oldestKey string
		var oldestTime time.Time
		for k, v := range c.scratchpad {
			if oldestKey == "" || v.CreatedAt.Before(oldestTime) {
				oldestKey = k
				oldestTime = v.CreatedAt
			}
		}
		if oldestKey != "" {
			delete(c.scratchpad, oldestKey)
		}
	}

	now := time.Now()
	if entry, exists := c.scratchpad[key]; exists {
		entry.Value = value
		entry.SetBy = setBy
		entry.UpdatedAt = now
	} else {
		c.scratchpad[key] = &ScratchpadEntry{
			Key:       key,
			Value:     value,
			SetBy:     setBy,
			CreatedAt: now,
			UpdatedAt: now,
		}
	}
	return nil
}

// GetScratchpad retrieves a scratchpad entry by key.
func (c *Coordinator) GetScratchpad(key string) (*ScratchpadEntry, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.scratchpad[key]
	if !exists {
		return nil, fmt.Errorf("scratchpad key %q not found", key)
	}
	return entry, nil
}

// GetAllScratchpad returns all scratchpad entries.
func (c *Coordinator) GetAllScratchpad() map[string]*ScratchpadEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[string]*ScratchpadEntry, len(c.scratchpad))
	for k, v := range c.scratchpad {
		result[k] = v
	}
	return result
}

// DeleGateWork assigns a task to an available worker in a team.
func (c *Coordinator) DeleGateWork(teamName, task string) (*Worker, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	team, exists := c.teams[teamName]
	if !exists {
		return nil, fmt.Errorf("team %q not found", teamName)
	}

	// Find an idle worker
	team.mu.RLock()
	var candidate *Worker
	for _, member := range team.Members {
		if member.Status == AgentIdle {
			candidate = member
			break
		}
	}
	team.mu.RUnlock()

	if candidate == nil {
		return nil, fmt.Errorf("no idle workers available in team %q", teamName)
	}

	candidate.AssignTask(task)
	return candidate, nil
}

// InjectContext builds a context string for a worker agent, including
// scratchpad entries and team-relevant messages.
func (c *Coordinator) InjectContext(workerID string, teamName string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var ctx string

	// Add scratchpad context
	if len(c.scratchpad) > 0 {
		ctx += "## Shared Scratchpad\n\n"
		for key, entry := range c.scratchpad {
			ctx += fmt.Sprintf("- **%s**: %s (set by %s)\n", key, entry.Value, entry.SetBy)
		}
		ctx += "\n"
	}

	// Add team context
	if team, exists := c.teams[teamName]; exists {
		memberNames := make([]string, 0, len(team.Members))
		for _, m := range team.Members {
			memberNames = append(memberNames, fmt.Sprintf("%s (%s)", m.ID, m.Status))
		}
		ctx += fmt.Sprintf("## Team: %s\n\nMembers: %s\n\n", teamName, joinStrings(memberNames, ", "))

		// Add recent messages for this worker
		team.mu.RLock()
		for _, msg := range team.Messages {
			if msg.To == workerID || msg.From == workerID || msg.To == "all" {
				ctx += fmt.Sprintf("[%s → %s]: %s\n", msg.From, msg.To, msg.Content)
			}
		}
		team.mu.RUnlock()
	}

	return ctx
}

// GetTeam returns a team by name.
func (c *Coordinator) GetTeam(name string) (*Team, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	team, exists := c.teams[name]
	if !exists {
		return nil, fmt.Errorf("team %q not found", name)
	}
	return team, nil
}

// ListTeams returns all team names.
func (c *Coordinator) ListTeams() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	names := make([]string, 0, len(c.teams))
	for name := range c.teams {
		names = append(names, name)
	}
	return names
}

// Shutdown stops all workers and cleans up.
func (c *Coordinator) Shutdown() {
	c.agentCancel()

	c.mu.Lock()
	defer c.mu.Unlock()

	for _, team := range c.teams {
		team.mu.Lock()
		for _, member := range team.Members {
			member.Stop()
		}
		team.mu.Unlock()
	}

	for _, worker := range c.workers {
		worker.Stop()
	}
}

func joinStrings(ss []string, sep string) string {
	result := ""
	for i, s := range ss {
		if i > 0 {
			result += sep
		}
		result += s
	}
	return result
}
