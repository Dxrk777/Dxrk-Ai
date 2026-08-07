// SPDX-License-Identifier: MIT
// Package config provides a comprehensive hierarchical configuration system for Dxrk.
//
// Configuration is loaded from multiple sources with the following priority
// (highest to lowest):
//
//  1. CLI flags (runtime overrides)
//  2. Environment variables (DXRK_* prefix)
//  3. Project config (.dxrk/config.yaml)
//  4. User config (~/.dxrk/config.yaml)
//  5. Global config (/etc/dxrk/config.yaml)
//  6. Built-in defaults
//
// The package provides:
//
//   - ConfigManager: hierarchical config with dot-notation access, validation, and watch
//   - SettingsStore: pluggable key-value persistence (file, project, memory)
//   - SettingsManager: multi-store settings with export/import
//   - SettingsSyncer: bidirectional sync across devices
//   - FeatureFlagManager: feature flags with rollout percentages
//   - Validators: composable config validation pipeline
//
// File formats: YAML (primary), JSON (fallback).
// All types are exported for cross-package usage.
package config
