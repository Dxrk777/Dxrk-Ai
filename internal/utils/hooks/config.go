package hooks

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

var (
	ErrConfigNotFound = errors.New("hooks: config not found")
	ErrConfigParse    = errors.New("hooks: config parse error")
)

// HookConfigFile represents the on-disk hook configuration format.
type HookConfigFile struct {
	Version string       `json:"version"`
	Hooks   []HookConfig `json:"hooks"`
}

// DefaultConfig returns a default hook configuration.
func DefaultConfig() *HookConfigFile {
	return &HookConfigFile{
		Version: "1.0",
		Hooks:   []HookConfig{},
	}
}

// LoadConfig loads hook configuration from a file.
func LoadConfig(path string) (*HookConfigFile, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return nil, ErrConfigNotFound
	}

	var cfg HookConfigFile
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, ErrConfigParse
	}

	if cfg.Version == "" {
		cfg.Version = "1.0"
	}

	for i := range cfg.Hooks {
		if cfg.Hooks[i].Timeout == 0 {
			cfg.Hooks[i].Timeout = 30 * time.Second
		}
		if cfg.Hooks[i].MaxRetries < 0 {
			cfg.Hooks[i].MaxRetries = 0
		}
		if cfg.Hooks[i].RetryDelay == 0 {
			cfg.Hooks[i].RetryDelay = 1 * time.Second
		}
	}

	return &cfg, nil
}

// SaveConfig saves hook configuration to a file.
func SaveConfig(path string, cfg *HookConfigFile) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return ErrConfigParse
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o644)
}

// ValidateConfig validates a hook configuration.
func ValidateConfig(cfg *HookConfigFile) error {
	if cfg == nil {
		return ErrConfigParse
	}

	ids := make(map[string]bool)
	for _, hook := range cfg.Hooks {
		if hook.ID == "" {
			return ErrConfigParse
		}
		if ids[hook.ID] {
			return ErrInvalidConfig
		}
		ids[hook.ID] = true

		if hook.Command == "" {
			return ErrConfigParse
		}

		if _, ok := ParseHookType(hook.Type.String()); !ok {
			return ErrConfigParse
		}

		if hook.Timeout < 0 {
			return ErrConfigParse
		}
		if hook.MaxRetries < 0 {
			return ErrConfigParse
		}
		if hook.RetryDelay < 0 {
			return ErrConfigParse
		}
	}

	return nil
}

// MergeConfigs merges multiple hook configurations.
func MergeConfigs(configs ...*HookConfigFile) *HookConfigFile {
	merged := DefaultConfig()
	ids := make(map[string]bool)

	for _, cfg := range configs {
		if cfg == nil {
			continue
		}
		for _, hook := range cfg.Hooks {
			if !ids[hook.ID] {
				merged.Hooks = append(merged.Hooks, hook)
				ids[hook.ID] = true
			}
		}
	}

	return merged
}

// FilterByType returns hooks of a specific type.
func FilterByType(cfg *HookConfigFile, ht HookType) []HookConfig {
	var result []HookConfig
	for _, hook := range cfg.Hooks {
		if hook.Type == ht {
			result = append(result, hook)
		}
	}
	return result
}

// FilterEnabled returns only enabled hooks.
func FilterEnabled(cfg *HookConfigFile) []HookConfig {
	var result []HookConfig
	for _, hook := range cfg.Hooks {
		if hook.Enabled {
			result = append(result, hook)
		}
	}
	return result
}

// HookDefaults holds default values for hook configuration.
type HookDefaults struct {
	Timeout    time.Duration
	MaxRetries int
	RetryDelay time.Duration
}

// ApplyDefaults applies default values to a hook config.
func (d HookDefaults) ApplyDefaults(cfg *HookConfig) {
	if cfg.Timeout == 0 {
		cfg.Timeout = d.Timeout
	}
	if cfg.MaxRetries < 0 {
		cfg.MaxRetries = d.MaxRetries
	}
	if cfg.RetryDelay == 0 {
		cfg.RetryDelay = d.RetryDelay
	}
}

// DefaultHookDefaults returns the default hook defaults.
func DefaultHookDefaults() HookDefaults {
	return HookDefaults{
		Timeout:    30 * time.Second,
		MaxRetries: 3,
		RetryDelay: 1 * time.Second,
	}
}
