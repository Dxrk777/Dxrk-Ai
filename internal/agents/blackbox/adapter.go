// SPDX-License-Identifier: MIT
// Package blackbox provides Blackbox AI agent integration.
package blackbox

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/Dxrk777/Dxrk-Ai/internal/model"
	"github.com/Dxrk777/Dxrk-Ai/internal/system"
)

var LookPathOverride = exec.LookPath

type statResult struct {
	isDir bool
	err   error
}

type Adapter struct {
	lookPath func(string) (string, error)
	statPath func(string) statResult
}

func NewAdapter() *Adapter {
	return &Adapter{
		lookPath: LookPathOverride,
		statPath: defaultStat,
	}
}

func (a *Adapter) Agent() model.AgentID    { return model.AgentBlackbox }
func (a *Adapter) Tier() model.SupportTier { return model.TierFull }

func (a *Adapter) Detect(_ context.Context, homeDir string) (bool, string, string, bool, error) {
	configPath := ConfigPath(homeDir)

	binaryPath, err := a.lookPath("blackbox")
	installed := err == nil

	stat := a.statPath(configPath)
	if stat.err != nil {
		if os.IsNotExist(stat.err) {
			return installed, binaryPath, configPath, false, nil
		}
		return false, "", "", false, stat.err
	}

	return installed, binaryPath, configPath, stat.isDir, nil
}

func (a *Adapter) SupportsAutoInstall() bool { return true }

func (a *Adapter) InstallCommand(_ system.PlatformProfile) ([][]string, error) {
	return [][]string{
		{"npm", "install", "-g", "blackboxai"},
	}, nil
}

func (a *Adapter) GlobalConfigDir(homeDir string) string  { return ConfigPath(homeDir) }
func (a *Adapter) SystemPromptDir(homeDir string) string  { return homeDir }
func (a *Adapter) SystemPromptFile(homeDir string) string { return filepath.Join(homeDir, "AGENTS.md") }
func (a *Adapter) SkillsDir(homeDir string) string {
	return filepath.Join(ConfigPath(homeDir), "skills")
}
func (a *Adapter) SettingsPath(homeDir string) string {
	return filepath.Join(ConfigPath(homeDir), "settings.json")
}

func (a *Adapter) SystemPromptStrategy() model.SystemPromptStrategy {
	return model.StrategyAppendToFile
}

func (a *Adapter) MCPStrategy() model.MCPStrategy {
	return model.StrategyMCPConfigFile
}

func (a *Adapter) MCPConfigPath(homeDir string, _ string) string {
	return filepath.Join(ConfigPath(homeDir), "mcp.json")
}

func (a *Adapter) SupportsOutputStyles() bool     { return false }
func (a *Adapter) OutputStyleDir(_ string) string { return "" }
func (a *Adapter) SupportsSlashCommands() bool    { return false }
func (a *Adapter) CommandsDir(_ string) string    { return "" }
func (a *Adapter) SupportsSubAgents() bool        { return true }
func (a *Adapter) SubAgentsDir(homeDir string) string {
	return filepath.Join(ConfigPath(homeDir), "agents")
}
func (a *Adapter) EmbeddedSubAgentsDir() string { return "blackbox/agents" }
func (a *Adapter) SupportsSkills() bool         { return true }
func (a *Adapter) SupportsSystemPrompt() bool   { return true }
func (a *Adapter) SupportsMCP() bool            { return true }

func ConfigPath(homeDir string) string {
	return filepath.Join(homeDir, ".blackbox")
}

func defaultStat(path string) statResult {
	info, err := os.Stat(path)
	if err != nil {
		return statResult{err: err}
	}
	return statResult{isDir: info.IsDir()}
}

type AgentNotInstallableError struct{ Agent model.AgentID }

func (e AgentNotInstallableError) Error() string {
	return fmt.Sprintf("agent %q must be installed manually before Dxrk AI can configure it", e.Agent)
}
