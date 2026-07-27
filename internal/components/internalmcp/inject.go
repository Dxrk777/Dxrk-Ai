// SPDX-License-Identifier: MIT
package internalmcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Dxrk777/Dxrk-Ai/internal/agents"
	"github.com/Dxrk777/Dxrk-Ai/internal/components/filemerge"
	"github.com/Dxrk777/Dxrk-Ai/internal/model"
)

type InjectionResult struct {
	Changed bool
	Files   []string
}

var osExecutable = os.Executable

func Inject(homeDir string, adapter agents.Adapter) (InjectionResult, error) {
	if !adapter.SupportsMCP() {
		return InjectionResult{}, nil
	}

	dxrkBin, err := osExecutable()
	if err != nil {
		return InjectionResult{}, fmt.Errorf("resolve dxrk binary: %w", err)
	}

	config := map[string]any{
		"mcpServers": map[string]any{
			"dxrk": map[string]any{
				"command": dxrkBin,
				"args":    []string{"mcp", "serve"},
			},
		},
	}
	overlay, err := json.Marshal(config)
	if err != nil {
		return InjectionResult{}, fmt.Errorf("marshal config: %w", err)
	}

	switch adapter.MCPStrategy() {
	case model.StrategySeparateMCPFiles:
		mcpPath := adapter.MCPConfigPath(homeDir, "dxrk")
		if mcpPath == "" {
			return InjectionResult{}, nil
		}
		if err := os.MkdirAll(filepath.Dir(mcpPath), 0o750); err != nil {
			return InjectionResult{}, fmt.Errorf("create mcp dir: %w", err)
		}
		wr, err := filemerge.WriteFileAtomic(mcpPath, overlay, 0o600)
		if err != nil {
			return InjectionResult{}, fmt.Errorf("write %q: %w", mcpPath, err)
		}
		return InjectionResult{Changed: wr.Changed, Files: []string{mcpPath}}, nil

	case model.StrategyMergeIntoSettings, model.StrategyMCPConfigFile:
		cfgPath := adapter.MCPConfigPath(homeDir, "dxrk")
		if cfgPath == "" {
			cfgPath = adapter.SettingsPath(homeDir)
		}
		if cfgPath == "" {
			return InjectionResult{}, nil
		}
		base, err := readFile(cfgPath)
		if err != nil {
			return InjectionResult{}, err
		}
		merged, err := filemerge.MergeJSONObjects(base, overlay)
		if err != nil {
			return InjectionResult{}, fmt.Errorf("merge into %q: %w", cfgPath, err)
		}
		wr, err := filemerge.WriteFileAtomic(cfgPath, merged, 0o600)
		if err != nil {
			return InjectionResult{}, err
		}
		return InjectionResult{Changed: wr.Changed, Files: []string{cfgPath}}, nil
	}

	return InjectionResult{}, nil
}

func readFile(path string) ([]byte, error) {
	content, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read %q: %w", path, err)
	}
	return content, nil
}
