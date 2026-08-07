// SPDX-License-Identifier: MIT
package mcp

import (
	"fmt"

	"github.com/Dxrk777/Dxrk/internal/agents"
	"github.com/Dxrk777/Dxrk/internal/components/filemerge"
	"github.com/Dxrk777/Dxrk/internal/model"
)

var defaultMemPalaceServerJSON = []byte("{\n  \"command\": \"mempalace-mcp\",\n  \"args\": []\n}\n")

var defaultMemPalaceOverlayJSON = []byte("{\n  \"mcpServers\": {\n    \"mempalace\": {\n      \"command\": \"mempalace-mcp\",\n      \"args\": []\n    }\n  }\n}\n")

var openCodeMemPalaceOverlayJSON = []byte("{\n  \"mcp\": {\n    \"mempalace\": {\n      \"type\": \"local\",\n      \"command\": [\"mempalace-mcp\"],\n      \"enabled\": true\n    }\n  }\n}\n")

func DefaultMemPalaceServerJSON() []byte {
	content := make([]byte, len(defaultMemPalaceServerJSON))
	copy(content, defaultMemPalaceServerJSON)
	return content
}

func DefaultMemPalaceOverlayJSON() []byte {
	content := make([]byte, len(defaultMemPalaceOverlayJSON))
	copy(content, defaultMemPalaceOverlayJSON)
	return content
}

func OpenCodeMemPalaceOverlayJSON() []byte {
	content := make([]byte, len(openCodeMemPalaceOverlayJSON))
	copy(content, openCodeMemPalaceOverlayJSON)
	return content
}

func InjectMemPalace(homeDir string, adapter agents.Adapter) (InjectionResult, error) {
	if !adapter.SupportsMCP() {
		return InjectionResult{}, nil
	}

	switch adapter.MCPStrategy() {
	case model.StrategySeparateMCPFiles:
		return injectMemPalaceSeparateFile(homeDir, adapter)
	case model.StrategyMergeIntoSettings:
		return injectMemPalaceMergeIntoSettings(homeDir, adapter)
	case model.StrategyMCPConfigFile:
		return injectMemPalaceMCPConfigFile(homeDir, adapter)
	case model.StrategyTOMLFile:
		return InjectionResult{}, nil
	default:
		return InjectionResult{}, fmt.Errorf("mempalace injector does not support MCP strategy %d for agent %q", adapter.MCPStrategy(), adapter.Agent())
	}
}

func injectMemPalaceSeparateFile(homeDir string, adapter agents.Adapter) (InjectionResult, error) {
	path := adapter.MCPConfigPath(homeDir, "mempalace")
	writeResult, err := filemerge.WriteFileAtomic(path, DefaultMemPalaceServerJSON(), 0o600)
	if err != nil {
		return InjectionResult{}, err
	}
	return InjectionResult{Changed: writeResult.Changed, Files: []string{path}}, nil
}

func injectMemPalaceMergeIntoSettings(homeDir string, adapter agents.Adapter) (InjectionResult, error) {
	settingsPath := adapter.SettingsPath(homeDir)
	if settingsPath == "" {
		return InjectionResult{}, nil
	}

	overlay := DefaultMemPalaceOverlayJSON()
	if adapter.Agent() == model.AgentOpenCode || adapter.Agent() == model.AgentKilocode {
		overlay = OpenCodeMemPalaceOverlayJSON()
	}

	settingsWrite, err := mergeJSONFile(settingsPath, overlay)
	if err != nil {
		return InjectionResult{}, err
	}

	return InjectionResult{Changed: settingsWrite.Changed, Files: []string{settingsPath}}, nil
}

func injectMemPalaceMCPConfigFile(homeDir string, adapter agents.Adapter) (InjectionResult, error) {
	path := adapter.MCPConfigPath(homeDir, "mempalace")
	if path == "" {
		return InjectionResult{}, nil
	}
	overlay := DefaultMemPalaceOverlayJSON()
	settingsWrite, err := mergeJSONFile(path, overlay)
	if err != nil {
		return InjectionResult{}, err
	}
	return InjectionResult{Changed: settingsWrite.Changed, Files: []string{path}}, nil
}
