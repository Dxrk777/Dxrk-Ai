// SPDX-License-Identifier: MIT
package checker

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Dxrk777/Dxrk-Ai/internal/components/filemerge"
)

type InjectionResult struct {
	Changed bool
	Files   []string
}

// defaultRulesJSON defines the default Checker permission rules written during install/sync.
var defaultRulesJSON = []byte(`{
  "deny_by_default": true,
  "rules": [
    {"action": "read", "target": "*.env", "allow": false},
    {"action": "read", "target": "*.env.*", "allow": false},
    {"action": "read", "target": "**/secrets/**", "allow": false},
    {"action": "read", "target": "**/credentials.json", "allow": false},
    {"action": "read", "target": "**/.ssh/**", "allow": false},
    {"action": "read", "target": "**/.config/opencode/keys.json", "allow": false},
    {"action": "read", "target": "**/.config/opencode/auth.json", "allow": false},
    {"action": "exec", "target": "rm -rf /*", "allow": false},
    {"action": "exec", "target": "sudo rm -rf /*", "allow": false},
    {"action": "edit", "target": "*.env", "allow": false},
    {"action": "edit", "target": "*.env.*", "allow": false}
  ]
}`)

type checkerConfig struct {
	DenyByDefault bool        `json:"deny_by_default"`
	Rules         []ruleEntry `json:"rules"`
}

type ruleEntry struct {
	Action string `json:"action"`
	Target string `json:"target"`
	Allow  bool   `json:"allow"`
}

func Inject(homeDir string) (InjectionResult, error) {
	dir := filepath.Join(homeDir, ".config", "dxrk")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return InjectionResult{}, fmt.Errorf("create checker dir: %w", err)
	}

	path := filepath.Join(dir, "checker.json")

	// Validate JSON before writing
	if !json.Valid(defaultRulesJSON) {
		return InjectionResult{}, fmt.Errorf("default checker rules are not valid JSON")
	}

	wr, err := filemerge.WriteFileAtomic(path, defaultRulesJSON, 0o600)
	if err != nil {
		return InjectionResult{}, fmt.Errorf("write checker config: %w", err)
	}

	return InjectionResult{Changed: wr.Changed, Files: []string{path}}, nil
}

// DefaultCheckerConfig returns the path to the checker config file.
func DefaultCheckerConfig(homeDir string) string {
	return filepath.Join(homeDir, ".config", "dxrk", "checker.json")
}

// LoadRules reads rules from the checker config file.
func LoadRules(homeDir string) ([]Rule, bool, error) {
	path := DefaultCheckerConfig(homeDir)
	data, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		if os.IsNotExist(err) {
			return nil, true, nil
		}
		return nil, true, fmt.Errorf("read checker config: %w", err)
	}

	var cfg checkerConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, true, fmt.Errorf("parse checker config: %w", err)
	}

	rules := make([]Rule, len(cfg.Rules))
	for i, r := range cfg.Rules {
		rules[i] = Rule(r)
	}
	return rules, cfg.DenyByDefault, nil
}

type Rule struct {
	Action string
	Target string
	Allow  bool
}
