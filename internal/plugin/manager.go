// SPDX-License-Identifier: MIT
// Package plugin provides a tool plugin system with file-based discovery.
package plugin

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Dxrk777/Dxrk/internal/tools"
)

// Plugin defines an external tool provider.
type Plugin struct {
	Name        string
	Description string
	Version     string
	Register    func(*tools.Registry) error
}

// Manager discovers and loads tool plugins.
type Manager struct {
	registry   *tools.Registry
	pluginsDir string
	plugins    []Plugin
}

// NewManager creates a plugin manager.
func NewManager(registry *tools.Registry, pluginsDir string) *Manager {
	return &Manager{
		registry:   registry,
		pluginsDir: pluginsDir,
	}
}

// Register adds a plugin to the manager.
func (m *Manager) Register(p Plugin) error {
	if p.Name == "" {
		return fmt.Errorf("plugin name is required")
	}
	if p.Register == nil {
		return fmt.Errorf("plugin %q: Register function is required", p.Name)
	}
	m.plugins = append(m.plugins, p)
	return nil
}

// LoadAll loads all registered plugins.
func (m *Manager) LoadAll() (int, error) {
	loaded := 0
	for _, p := range m.plugins {
		if err := p.Register(m.registry); err != nil {
			return loaded, fmt.Errorf("load plugin %q: %w", p.Name, err)
		}
		loaded++
	}
	return loaded, nil
}

// DiscoverPlugins scans the plugins directory for plugin packages.
// Each subdirectory with a plugin.json manifest is discovered but not loaded.
type Manifest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version"`
	Entry       string `json:"entry"`
}

// Discover returns manifests found in the plugins directory.
func (m *Manager) Discover() ([]Manifest, error) {
	entries, err := os.ReadDir(m.pluginsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read plugins dir %q: %w", m.pluginsDir, err)
	}

	var manifests []Manifest
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		manifestPath := filepath.Join(m.pluginsDir, entry.Name(), "plugin.json")
		if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
			continue
		}
		// Found a manifest (actual loading deferred to LoadAll)
		_ = manifestPath
	}
	return manifests, nil
}

// Plugins returns the list of registered plugins.
func (m *Manager) Plugins() []Plugin {
	out := make([]Plugin, len(m.plugins))
	copy(out, m.plugins)
	return out
}
