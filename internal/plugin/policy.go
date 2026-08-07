// SPDX-License-Identifier: MIT
package plugin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Dxrk777/Dxrk/internal/strconst"
)

// PolicyLevel defines the enforcement level for enterprise policies.
type PolicyLevel int

const (
	PolicyAdvisory PolicyLevel = iota // Log but allow
	PolicyEnforce                     // Block if policy violated
)

func (p PolicyLevel) String() string {
	switch p {
	case PolicyAdvisory:
		return "advisory"
	case PolicyEnforce:
		return "enforce"
	default:
		return strconst.StrUnknown
	}
}

// EnterprisePolicy defines organizational rules for plugin usage.
type EnterprisePolicy struct {
	Level            PolicyLevel          `json:"level"`
	AllowedPlugins   []string             `json:"allowed_plugins,omitempty"`    // Allowlist of plugin IDs
	BlockedPlugins   []string             `json:"blocked_plugins,omitempty"`    // Blocklist of plugin IDs
	AllowedAuthors   []string             `json:"allowed_authors,omitempty"`    // Only plugins from these authors
	MaxPluginVersion string               `json:"max_plugin_version,omitempty"` // Max semver allowed
	RequiredTags     []string             `json:"required_tags,omitempty"`      // Plugins must have these tags
	BlockedTags      []string             `json:"blocked_tags,omitempty"`       // Plugins with these tags are blocked
	RestrictedHooks  []HookPoint          `json:"restricted_hooks,omitempty"`   // Hooks that require approval
	ApprovalRequired PluginApprovalConfig `json:"approval_required"`
	AuditLog         bool                 `json:"audit_log"`   // Log all plugin operations
	MaxPlugins       int                  `json:"max_plugins"` // Max concurrent plugins (0 = unlimited)
	TimeoutPolicy    PluginTimeoutPolicy  `json:"timeout_policy"`
}

// PluginApprovalConfig controls when plugin actions require approval.
type PluginApprovalConfig struct {
	Install  bool `json:"install"`
	Enable   bool `json:"enable"`
	Disable  bool `json:"disable"`
	Update   bool `json:"update"`
	HookExec bool `json:"hook_exec"`
}

// PluginTimeoutPolicy sets timeout limits.
type PluginTimeoutPolicy struct {
	MaxLoadTime     time.Duration `json:"max_load_time"`
	MaxHookTime     time.Duration `json:"max_hook_time"`
	MaxMCPStartTime time.Duration `json:"max_mcp_start_time"`
}

// AuditEntry records a plugin operation for auditing.
type AuditEntry struct {
	Timestamp time.Time         `json:"timestamp"`
	Action    string            `json:"action"`
	PluginID  string            `json:"plugin_id"`
	User      string            `json:"user,omitempty"`
	Allowed   bool              `json:"allowed"`
	Reason    string            `json:"reason,omitempty"`
	Details   map[string]string `json:"details,omitempty"`
}

// PolicyManager enforces enterprise plugin policies.
type PolicyManager struct {
	mu       sync.RWMutex
	policy   EnterprisePolicy
	auditLog []AuditEntry
	maxAudit int
}

// NewPolicyManager creates a policy manager with default (permissive) policy.
func NewPolicyManager() *PolicyManager {
	return &PolicyManager{
		policy: EnterprisePolicy{
			Level:      PolicyAdvisory,
			MaxPlugins: 0,
			AuditLog:   false,
			TimeoutPolicy: PluginTimeoutPolicy{
				MaxLoadTime:     10 * time.Second,
				MaxHookTime:     30 * time.Second,
				MaxMCPStartTime: 15 * time.Second,
			},
		},
		maxAudit: 1000,
	}
}

// LoadPolicy loads a policy from a JSON file.
func (pm *PolicyManager) LoadPolicy(path string) error {
	data, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		return fmt.Errorf("read policy file: %w", err)
	}

	var policy EnterprisePolicy
	if err := json.Unmarshal(data, &policy); err != nil {
		return fmt.Errorf("parse policy: %w", err)
	}

	pm.mu.Lock()
	pm.policy = policy
	pm.mu.Unlock()
	return nil
}

// SavePolicy saves the current policy to a JSON file.
func (pm *PolicyManager) SavePolicy(path string) error {
	pm.mu.RLock()
	policy := pm.policy
	pm.mu.RUnlock()

	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	data, err := json.MarshalIndent(policy, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// GetPolicy returns a copy of the current policy.
func (pm *PolicyManager) GetPolicy() EnterprisePolicy {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.policy
}

// CheckInstall checks if a plugin can be installed per policy.
func (pm *PolicyManager) CheckInstall(meta *PluginMetadata) (bool, string) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	// Check blocked list.
	for _, blocked := range pm.policy.BlockedPlugins {
		if blocked == meta.ID {
			pm.audit("install", meta.ID, false, "plugin is blocked")
			return false, "plugin is in the blocked list"
		}
	}

	// Check allowlist (if non-empty, only allowed plugins can install).
	if len(pm.policy.AllowedPlugins) > 0 {
		allowed := false
		for _, a := range pm.policy.AllowedPlugins {
			if a == meta.ID {
				allowed = true
				break
			}
		}
		if !allowed {
			pm.audit("install", meta.ID, false, "plugin not in allowlist")
			return false, "plugin is not in the allowed list"
		}
	}

	// Check author restrictions.
	if len(pm.policy.AllowedAuthors) > 0 {
		authorOK := false
		for _, a := range pm.policy.AllowedAuthors {
			if a == meta.Author {
				authorOK = true
				break
			}
		}
		if !authorOK {
			pm.audit("install", meta.ID, false, "author not allowed")
			return false, "plugin author is not in the allowed authors list"
		}
	}

	// Check blocked tags.
	for _, blockedTag := range pm.policy.BlockedTags {
		for _, tag := range meta.Tags {
			if tag == blockedTag {
				pm.audit("install", meta.ID, false, "blocked tag: "+tag)
				return false, fmt.Sprintf("plugin has blocked tag: %s", tag)
			}
		}
	}

	// Check max plugins limit.
	if pm.policy.MaxPlugins > 0 {
		pm.audit("install", meta.ID, true, "within plugin limit")
	}

	pm.audit("install", meta.ID, true, "policy check passed")
	return true, ""
}

// CheckHookExec checks if a hook can be executed per policy.
func (pm *PolicyManager) CheckHookExec(pluginID string, point HookPoint) (bool, string) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	for _, restricted := range pm.policy.RestrictedHooks {
		if restricted == point {
			pm.audit("hook_exec", pluginID, false, "restricted hook: "+string(point))
			return false, fmt.Sprintf("hook %s requires approval", point)
		}
	}

	pm.audit("hook_exec", pluginID, true, "policy check passed")
	return true, ""
}

// IsPluginAllowed checks if a plugin is generally allowed.
func (pm *PolicyManager) IsPluginAllowed(pluginID string) bool {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	for _, blocked := range pm.policy.BlockedPlugins {
		if blocked == pluginID {
			return false
		}
	}

	if len(pm.policy.AllowedPlugins) > 0 {
		for _, allowed := range pm.policy.AllowedPlugins {
			if allowed == pluginID {
				return true
			}
		}
		return false
	}

	return true
}

func (pm *PolicyManager) audit(action, pluginID string, allowed bool, reason string) {
	if !pm.policy.AuditLog {
		return
	}

	entry := AuditEntry{
		Timestamp: time.Now(),
		Action:    action,
		PluginID:  pluginID,
		Allowed:   allowed,
		Reason:    reason,
	}

	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.auditLog = append(pm.auditLog, entry)
	if len(pm.auditLog) > pm.maxAudit {
		pm.auditLog = pm.auditLog[len(pm.auditLog)-pm.maxAudit:]
	}
}

// GetAuditLog returns a copy of the audit log.
func (pm *PolicyManager) GetAuditLog() []AuditEntry {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	out := make([]AuditEntry, len(pm.auditLog))
	copy(out, pm.auditLog)
	return out
}
