// SPDX-License-Identifier: MIT
// Package plugin provides a tool plugin system with lifecycle hooks,
// marketplace integration, and enterprise policy enforcement.
package plugin

import (
	"fmt"
	"sync"
	"time"
)

// HookPoint defines a named lifecycle hook point.
type HookPoint string

const (
	// Pre/post pairs for core operations.
	HookBeforeLoad       HookPoint = "before_load"
	HookAfterLoad        HookPoint = "after_load"
	HookBeforeUnload     HookPoint = "before_unload"
	HookAfterUnload      HookPoint = "after_unload"
	HookBeforeEnable     HookPoint = "before_enable"
	HookAfterEnable      HookPoint = "after_enable"
	HookBeforeDisable    HookPoint = "before_disable"
	HookAfterDisable     HookPoint = "after_disable"
	HookBeforeUpdate     HookPoint = "before_update"
	HookAfterUpdate      HookPoint = "after_update"
	HookBeforeRegister   HookPoint = "before_register"
	HookAfterRegister    HookPoint = "after_register"
	HookBeforeDeregister HookPoint = "before_deregister"
	HookAfterDeregister  HookPoint = "after_deregister"
	HookBeforeSettings   HookPoint = "before_settings"
	HookAfterSettings    HookPoint = "after_settings"

	// Pre/post pairs for tool operations.
	HookBeforeToolExec  HookPoint = "before_tool_exec"
	HookAfterToolExec   HookPoint = "after_tool_exec"
	HookBeforeToolReg   HookPoint = "before_tool_register"
	HookAfterToolReg    HookPoint = "after_tool_register"
	HookBeforeToolDereg HookPoint = "before_tool_deregister"
	HookAfterToolDereg  HookPoint = "after_tool_deregister"

	// Pre/post pairs for MCP operations.
	HookBeforeMCPStart  HookPoint = "before_mcp_start"
	HookAfterMCPStart   HookPoint = "after_mcp_start"
	HookBeforeMCPStop   HookPoint = "before_mcp_stop"
	HookAfterMCPStop    HookPoint = "after_mcp_stop"
	HookBeforeMCPConfig HookPoint = "before_mcp_config"
	HookAfterMCPConfig  HookPoint = "after_mcp_config"

	// Pre/post pairs for skill operations.
	HookBeforeSkillLoad HookPoint = "before_skill_load"
	HookAfterSkillLoad  HookPoint = "after_skill_load"
	HookBeforeSkillExec HookPoint = "before_skill_exec"
	HookAfterSkillExec  HookPoint = "after_skill_exec"

	// Pre/post pairs for hook operations.
	HookBeforeHookExec HookPoint = "before_hook_exec"
	HookAfterHookExec  HookPoint = "after_hook_exec"
)

// HookEvent contains metadata about a hook invocation.
type HookEvent struct {
	Point    HookPoint
	PluginID string
	Time     time.Time
	Data     map[string]any
}

// HookFunc is the signature for hook callbacks.
// Returning an error aborts the operation.
type HookFunc func(event HookEvent) error

// HookRegistration stores a registered hook callback.
type HookRegistration struct {
	ID       string
	Point    HookPoint
	Priority int // Lower = runs first
	Fn       HookFunc
}

// HookManager manages lifecycle hooks for plugins.
type HookManager struct {
	mu     sync.RWMutex
	hooks  map[HookPoint][]HookRegistration
	nextID int
}

// NewHookManager creates a new hook manager.
func NewHookManager() *HookManager {
	return &HookManager{
		hooks: make(map[HookPoint][]HookRegistration),
	}
}

// Register adds a hook callback for a given hook point.
func (hm *HookManager) Register(point HookPoint, priority int, fn HookFunc) string {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	hm.nextID++
	id := fmt.Sprintf("hook-%d", hm.nextID)

	reg := HookRegistration{
		ID:       id,
		Point:    point,
		Priority: priority,
		Fn:       fn,
	}

	hooks := hm.hooks[point]
	// Insert sorted by priority.
	inserted := false
	for i, h := range hooks {
		if priority < h.Priority {
			hooks = append(hooks, HookRegistration{})
			copy(hooks[i+1:], hooks[i:])
			hooks[i] = reg
			inserted = true
			break
		}
	}
	if !inserted {
		hooks = append(hooks, reg)
	}
	hm.hooks[point] = hooks
	return id
}

// Remove removes a hook by its ID.
func (hm *HookManager) Remove(id string) {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	for point, hooks := range hm.hooks {
		for i, h := range hooks {
			if h.ID == id {
				hm.hooks[point] = append(hooks[:i], hooks[i+1:]...)
				return
			}
		}
	}
}

// Execute runs all hooks for a given point, in priority order.
// Returns the first error encountered (aborts remaining hooks).
func (hm *HookManager) Execute(point HookPoint, pluginID string, data map[string]any) error {
	hm.mu.RLock()
	hooks := make([]HookRegistration, len(hm.hooks[point]))
	copy(hooks, hm.hooks[point])
	hm.mu.RUnlock()

	event := HookEvent{
		Point:    point,
		PluginID: pluginID,
		Time:     time.Now(),
		Data:     data,
	}

	for _, h := range hooks {
		if err := h.Fn(event); err != nil {
			return fmt.Errorf("hook %s (%s) failed: %w", point, h.ID, err)
		}
	}
	return nil
}

// Clear removes all hooks.
func (hm *HookManager) Clear() {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	hm.hooks = make(map[HookPoint][]HookRegistration)
}

// HookCount returns the number of hooks registered for a point.
func (hm *HookManager) HookCount(point HookPoint) int {
	hm.mu.RLock()
	defer hm.mu.RUnlock()
	return len(hm.hooks[point])
}

// TotalHooks returns the total number of registered hooks.
func (hm *HookManager) TotalHooks() int {
	hm.mu.RLock()
	defer hm.mu.RUnlock()
	total := 0
	for _, hooks := range hm.hooks {
		total += len(hooks)
	}
	return total
}
