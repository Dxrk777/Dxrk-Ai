// SPDX-License-Identifier: MIT
package plugin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/Dxrk777/Dxrk-Ai/internal/strconst"
)

// PluginStatus represents the current state of a plugin.
type PluginStatus int

const (
	StatusInstalled PluginStatus = iota
	StatusEnabled
	StatusDisabled
	StatusUpdating
	StatusError
)

func (s PluginStatus) String() string {
	switch s {
	case StatusInstalled:
		return "installed"
	case StatusEnabled:
		return strconst.StrEnabled
	case StatusDisabled:
		return "disabled"
	case StatusUpdating:
		return "updating"
	case StatusError:
		return strconst.StrError
	default:
		return strconst.StrUnknown
	}
}

// PluginMetadata holds metadata about a plugin in the marketplace.
type PluginMetadata struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Version     string           `json:"version"`
	Description string           `json:"description"`
	Author      string           `json:"author"`
	License     string           `json:"license"`
	Tags        []string         `json:"tags,omitempty"`
	Homepage    string           `json:"homepage,omitempty"`
	Repository  string           `json:"repository,omitempty"`
	Components  PluginComponents `json:"components"`
	Settings    PluginSettings   `json:"settings,omitempty"`
	Status      PluginStatus     `json:"status"`
	InstalledAt *time.Time       `json:"installed_at,omitempty"`
	UpdatedAt   *time.Time       `json:"updated_at,omitempty"`
	Enabled     bool             `json:"enabled"`
}

// PluginComponents describes what a plugin provides.
type PluginComponents struct {
	Skills []PluginSkill `json:"skills,omitempty"`
	Hooks  []PluginHook  `json:"hooks,omitempty"`
	MCPs   []PluginMCP   `json:"mcps,omitempty"`
	Tools  []PluginTool  `json:"tools,omitempty"`
}

type PluginSkill struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Description string `json:"description"`
}

type PluginHook struct {
	Name     string    `json:"name"`
	Point    HookPoint `json:"point"`
	Priority int       `json:"priority"`
}

type PluginMCP struct {
	Name    string            `json:"name"`
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

type PluginTool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// PluginSettings controls plugin-specific configuration.
type PluginSettings struct {
	RequiresApproval bool              `json:"requires_approval,omitempty"`
	MaxInstances     int               `json:"max_instances,omitempty"`
	Timeout          time.Duration     `json:"timeout,omitempty"`
	Env              map[string]string `json:"env,omitempty"`
}

// Marketplace manages plugin discovery, installation, and updates.
type Marketplace struct {
	mu         sync.RWMutex
	registry   map[string]*PluginMetadata
	pluginsDir string
	onChange   func(event MarketplaceEvent)
}

// MarketplaceEvent describes a marketplace change.
type MarketplaceEvent struct {
	Type     string // "installed", "updated", "removed", "enabled", "disabled"
	PluginID string
	Time     time.Time
}

// NewMarketplace creates a new marketplace manager.
func NewMarketplace(pluginsDir string) *Marketplace {
	m := &Marketplace{
		registry:   make(map[string]*PluginMetadata),
		pluginsDir: pluginsDir,
	}
	return m
}

// SetOnChange sets a callback for marketplace events.
func (mp *Marketplace) SetOnChange(fn func(MarketplaceEvent)) {
	mp.mu.Lock()
	defer mp.mu.Unlock()
	mp.onChange = fn
}

func (mp *Marketplace) emitEvent(typ, pluginID string) {
	if mp.onChange != nil {
		mp.onChange(MarketplaceEvent{
			Type:     typ,
			PluginID: pluginID,
			Time:     time.Now(),
		})
	}
}

// Scan scans the plugins directory for installed plugins.
func (mp *Marketplace) Scan() error {
	mp.mu.Lock()
	defer mp.mu.Unlock()

	entries, err := os.ReadDir(mp.pluginsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read plugins dir: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		manifestPath := filepath.Join(mp.pluginsDir, entry.Name(), "plugin.json")
		data, err := os.ReadFile(manifestPath) //nolint:gosec
		if err != nil {
			continue
		}
		var meta PluginMetadata
		if err := json.Unmarshal(data, &meta); err != nil {
			continue
		}
		meta.Status = StatusInstalled
		mp.registry[meta.ID] = &meta
	}
	return nil
}

// Install installs a plugin from a manifest file.
func (mp *Marketplace) Install(meta PluginMetadata) error {
	mp.mu.Lock()
	defer mp.mu.Unlock()

	if _, exists := mp.registry[meta.ID]; exists {
		return fmt.Errorf("plugin %s already installed", meta.ID)
	}

	meta.Status = StatusEnabled
	meta.Enabled = true
	now := time.Now()
	meta.InstalledAt = &now
	mp.registry[meta.ID] = &meta
	mp.emitEvent("installed", meta.ID)
	return nil
}

// Uninstall removes a plugin from the marketplace.
func (mp *Marketplace) Uninstall(pluginID string) error {
	mp.mu.Lock()
	defer mp.mu.Unlock()

	if _, exists := mp.registry[pluginID]; !exists {
		return fmt.Errorf("plugin %s not found", pluginID)
	}

	delete(mp.registry, pluginID)
	mp.emitEvent("removed", pluginID)
	return nil
}

// Enable enables a plugin.
func (mp *Marketplace) Enable(pluginID string) error {
	mp.mu.Lock()
	defer mp.mu.Unlock()

	meta, exists := mp.registry[pluginID]
	if !exists {
		return fmt.Errorf("plugin %s not found", pluginID)
	}

	meta.Enabled = true
	meta.Status = StatusEnabled
	mp.emitEvent(strconst.StrEnabled, pluginID)
	return nil
}

// Disable disables a plugin.
func (mp *Marketplace) Disable(pluginID string) error {
	mp.mu.Lock()
	defer mp.mu.Unlock()

	meta, exists := mp.registry[pluginID]
	if !exists {
		return fmt.Errorf("plugin %s not found", pluginID)
	}

	meta.Enabled = false
	meta.Status = StatusDisabled
	mp.emitEvent("disabled", pluginID)
	return nil
}

// Get returns metadata for a plugin.
func (mp *Marketplace) Get(pluginID string) (*PluginMetadata, bool) {
	mp.mu.RLock()
	defer mp.mu.RUnlock()
	meta, ok := mp.registry[pluginID]
	return meta, ok
}

// List returns all registered plugins, sorted by name.
func (mp *Marketplace) List() []*PluginMetadata {
	mp.mu.RLock()
	defer mp.mu.RUnlock()

	out := make([]*PluginMetadata, 0, len(mp.registry))
	for _, meta := range mp.registry {
		out = append(out, meta)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}

// EnabledPlugins returns only enabled plugins.
func (mp *Marketplace) EnabledPlugins() []*PluginMetadata {
	mp.mu.RLock()
	defer mp.mu.RUnlock()

	var out []*PluginMetadata
	for _, meta := range mp.registry {
		if meta.Enabled {
			out = append(out, meta)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}

// UpdateAvailable checks if an update is available for a plugin.
func (mp *Marketplace) UpdateAvailable(pluginID, latestVersion string) bool {
	mp.mu.RLock()
	defer mp.mu.RUnlock()

	meta, ok := mp.registry[pluginID]
	if !ok {
		return false
	}
	return meta.Version != latestVersion
}
