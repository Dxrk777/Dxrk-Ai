// SPDX-License-Identifier: MIT
package devex

import (
	"context"
	"sort"
	"strings"
)

type Suggestion struct {
	Command     string
	Description string
	Priority    int
}

type Suggester struct {
	suggestions []Suggestion
}

func NewSuggester() *Suggester {
	return &Suggester{
		suggestions: defaultSuggestions(),
	}
}

func (s *Suggester) Suggest(_ context.Context, input string) []Suggestion {
	input = strings.TrimSpace(strings.ToLower(input))
	if input == "" {
		return topSuggestions(s.suggestions, 5)
	}

	var matched []Suggestion
	for _, sug := range s.suggestions {
		if strings.Contains(strings.ToLower(sug.Command), input) || strings.Contains(strings.ToLower(sug.Description), input) {
			matched = append(matched, sug)
		}
	}

	sort.Slice(matched, func(i, j int) bool {
		return matched[i].Priority > matched[j].Priority
	})

	if len(matched) > 10 {
		matched = matched[:10]
	}
	return matched
}

func topSuggestions(s []Suggestion, n int) []Suggestion {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func defaultSuggestions() []Suggestion {
	return []Suggestion{
		{Command: "dxrk help", Description: "Show help and available commands", Priority: 100},
		{Command: "dxrk sdd init", Description: "Initialize Spec-Driven Development in the project", Priority: 95},
		{Command: "dxrk sdd new", Description: "Create a new SDD change proposal", Priority: 90},
		{Command: "dxrk install", Description: "Install agents into the current project", Priority: 90},
		{Command: "dxrk sync", Description: "Sync agent configs across all IDEs", Priority: 90},
		{Command: "dxrk install --all", Description: "Install agents into all supported IDEs", Priority: 85},
		{Command: "dxrk upgrade", Description: "Upgrade dxrk to the latest version", Priority: 85},
		{Command: "dxrk sdd apply", Description: "Apply the current SDD change tasks", Priority: 85},
		{Command: "dxrk chat", Description: "Start an interactive chat session", Priority: 85},
		{Command: "dxrk install <agent>", Description: "Install a specific agent by name", Priority: 80},
		{Command: "dxrk query", Description: "Query all installed agents in parallel", Priority: 80},
		{Command: "dxrk agent-builder create", Description: "Scaffold a new custom agent", Priority: 80},
		{Command: "dxrk sdd verify", Description: "Verify implementation matches SDD specs", Priority: 80},
		{Command: "dxrk backup create", Description: "Create a backup of current agent configs", Priority: 75},
		{Command: "dxrk restore", Description: "Restore agent configs from a backup", Priority: 75},
		{Command: "dxrk dxrk-memory sync", Description: "Refresh Dxrk Memory persistence", Priority: 75},
		{Command: "dxrk validate", Description: "Validate project compatibility", Priority: 75},
		{Command: "dxrk backup list", Description: "List available backups", Priority: 70},
		{Command: "dxrk model list", Description: "List configured LLM models and providers", Priority: 70},
		{Command: "dxrk agent-builder list", Description: "List locally built agents", Priority: 70},
		{Command: "dxrk dxrk-memory download", Description: "Download the Dxrk Memory prompt", Priority: 70},
		{Command: "dxrk provider list", Description: "Show all configured LLM providers", Priority: 70},
		{Command: "dxrk dryrun", Description: "Preview changes without applying them", Priority: 70},
		{Command: "dxrk uninstall", Description: "Remove dxrk agents from the project", Priority: 70},
		{Command: "dxrk restore list", Description: "Show available restore points", Priority: 65},
		{Command: "dxrk model switch", Description: "Switch the active LLM model", Priority: 65},
		{Command: "dxrk catalog agents", Description: "Browse the agent catalog", Priority: 65},
		{Command: "dxrk catalog skills", Description: "Browse available skills", Priority: 65},
		{Command: "dxrk provider switch", Description: "Switch the default LLM provider", Priority: 65},
		{Command: "dxrk query --provider <name>", Description: "Query a specific provider only", Priority: 60},
		{Command: "dxrk telemetry enable", Description: "Enable local usage telemetry", Priority: 60},
		{Command: "dxrk telemetry disable", Description: "Disable local usage telemetry", Priority: 60},
		{Command: "dxrk version", Description: "Show installed dxrk version", Priority: 60},
		{Command: "dxrk completion bash", Description: "Generate bash completion script", Priority: 55},
		{Command: "dxrk completion zsh", Description: "Generate zsh completion script", Priority: 55},
	}
}
