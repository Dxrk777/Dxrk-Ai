// SPDX-License-Identifier: MIT
package agents

import (
	"fmt"

	"github.com/Dxrk777/Dxrk-Ai/internal/agents/aider"
	"github.com/Dxrk777/Dxrk-Ai/internal/agents/amazonq"
	"github.com/Dxrk777/Dxrk-Ai/internal/agents/antigravity"
	"github.com/Dxrk777/Dxrk-Ai/internal/agents/claude"
	"github.com/Dxrk777/Dxrk-Ai/internal/agents/cline"
	"github.com/Dxrk777/Dxrk-Ai/internal/agents/codex"
	"github.com/Dxrk777/Dxrk-Ai/internal/agents/cody"
	continueadapter "github.com/Dxrk777/Dxrk-Ai/internal/agents/continue_adapter"
	"github.com/Dxrk777/Dxrk-Ai/internal/agents/copilot"
	cursoradapter "github.com/Dxrk777/Dxrk-Ai/internal/agents/cursor"
	"github.com/Dxrk777/Dxrk-Ai/internal/agents/devin"
	"github.com/Dxrk777/Dxrk-Ai/internal/agents/gemini"
	"github.com/Dxrk777/Dxrk-Ai/internal/agents/junie"
	"github.com/Dxrk777/Dxrk-Ai/internal/agents/kilocode"
	"github.com/Dxrk777/Dxrk-Ai/internal/agents/kimi"
	"github.com/Dxrk777/Dxrk-Ai/internal/agents/kiro"
	"github.com/Dxrk777/Dxrk-Ai/internal/agents/openclaw"
	"github.com/Dxrk777/Dxrk-Ai/internal/agents/opencode"
	"github.com/Dxrk777/Dxrk-Ai/internal/agents/openhands"
	"github.com/Dxrk777/Dxrk-Ai/internal/agents/pi"
	"github.com/Dxrk777/Dxrk-Ai/internal/agents/qwen"
	"github.com/Dxrk777/Dxrk-Ai/internal/agents/replit"
	"github.com/Dxrk777/Dxrk-Ai/internal/agents/roocode"
	"github.com/Dxrk777/Dxrk-Ai/internal/agents/tabnine"
	voidadapter "github.com/Dxrk777/Dxrk-Ai/internal/agents/void_adapter"
	"github.com/Dxrk777/Dxrk-Ai/internal/agents/vscode"
	"github.com/Dxrk777/Dxrk-Ai/internal/agents/windsurf"
	"github.com/Dxrk777/Dxrk-Ai/internal/agents/zedai"
	"github.com/Dxrk777/Dxrk-Ai/internal/model"
)

var defaultAgentIDs = []model.AgentID{
	model.AgentClaudeCode,
	model.AgentOpenCode,
	model.AgentKilocode,
	model.AgentGeminiCLI,
	model.AgentCursor,
	model.AgentVSCodeCopilot,
	model.AgentCodex,
	model.AgentAntigravity,
	model.AgentWindsurf,
	model.AgentKimi,
	model.AgentQwenCode,
	model.AgentKiroIDE,
	model.AgentOpenClaw,
	model.AgentPi,
	model.AgentAider,
	model.AgentCline,
	model.AgentRooCode,
	model.AgentContinue,
	model.AgentJunie,
	model.AgentAmazonQ,
	model.AgentOpenHands,
	model.AgentZedAI,
	model.AgentCopilot,
	model.AgentDevin,
	model.AgentCody,
	model.AgentTabnine,
	model.AgentReplit,
	model.AgentVoid,
}

func NewAdapter(agent model.AgentID) (Adapter, error) {
	switch agent {
	case model.AgentClaudeCode:
		return claude.NewAdapter(), nil
	case model.AgentOpenCode:
		return opencode.NewAdapter(), nil
	case model.AgentKilocode:
		return kilocode.NewAdapter(), nil
	case model.AgentGeminiCLI:
		return gemini.NewAdapter(), nil
	case model.AgentCursor:
		return cursoradapter.NewAdapter(), nil
	case model.AgentVSCodeCopilot:
		return vscode.NewAdapter(), nil
	case model.AgentCodex:
		return codex.NewAdapter(), nil
	case model.AgentAntigravity:
		return antigravity.NewAdapter(), nil
	case model.AgentWindsurf:
		return windsurf.NewAdapter(), nil
	case model.AgentKimi:
		return kimi.NewAdapter(), nil
	case model.AgentQwenCode:
		return qwen.NewAdapter(), nil
	case model.AgentKiroIDE:
		return kiro.NewAdapter(), nil
	case model.AgentOpenClaw:
		return openclaw.NewAdapter(), nil
	case model.AgentPi:
		return pi.NewAdapter(), nil
	case model.AgentAider:
		return aider.NewAdapter(), nil
	case model.AgentCline:
		return cline.NewAdapter(), nil
	case model.AgentRooCode:
		return roocode.NewAdapter(), nil
	case model.AgentContinue:
		return continueadapter.NewAdapter(), nil
	case model.AgentJunie:
		return junie.NewAdapter(), nil
	case model.AgentAmazonQ:
		return amazonq.NewAdapter(), nil
	case model.AgentOpenHands:
		return openhands.NewAdapter(), nil
	case model.AgentZedAI:
		return zedai.NewAdapter(), nil
	case model.AgentCopilot:
		return copilot.NewAdapter(), nil
	case model.AgentDevin:
		return devin.NewAdapter(), nil
	case model.AgentCody:
		return cody.NewAdapter(), nil
	case model.AgentTabnine:
		return tabnine.NewAdapter(), nil
	case model.AgentReplit:
		return replit.NewAdapter(), nil
	case model.AgentVoid:
		return voidadapter.NewAdapter(), nil
	default:
		return nil, AgentNotSupportedError{Agent: agent}
	}
}

func NewDefaultRegistry() (*Registry, error) {
	adapters := make([]Adapter, 0, len(defaultAgentIDs))

	for _, agent := range defaultAgentIDs {
		adapter, err := NewAdapter(agent)
		if err != nil {
			return nil, fmt.Errorf("create %s adapter: %w", agent, err)
		}
		adapters = append(adapters, adapter)
	}

	registry, err := NewRegistry(adapters...)
	if err != nil {
		return nil, fmt.Errorf("create registry: %w", err)
	}

	return registry, nil
}

// NewMVPRegistry creates a registry with only the MVP agents (Claude Code, OpenCode).
// Kept for backward compatibility.
func NewMVPRegistry() (*Registry, error) {
	claudeAdapter, err := NewAdapter(model.AgentClaudeCode)
	if err != nil {
		return nil, fmt.Errorf("create claude adapter: %w", err)
	}

	opencodeAdapter, err := NewAdapter(model.AgentOpenCode)
	if err != nil {
		return nil, fmt.Errorf("create opencode adapter: %w", err)
	}

	registry, err := NewRegistry(claudeAdapter, opencodeAdapter)
	if err != nil {
		return nil, fmt.Errorf("create registry: %w", err)
	}

	return registry, nil
}
