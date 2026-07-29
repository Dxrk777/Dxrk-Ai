// SPDX-License-Identifier: MIT
package agents

import (
	"errors"
	"reflect"
	"testing"

	"github.com/Dxrk777/Dxrk-Ai/internal/model"
)

func TestFactoryResolvesPiAdapter(t *testing.T) {
	adapter, err := NewAdapter(model.AgentPi)
	if err != nil {
		t.Fatalf("NewAdapter(%q) returned error: %v", model.AgentPi, err)
	}

	if got := adapter.Agent(); got != model.AgentPi {
		t.Fatalf("adapter.Agent() = %q, want %q", got, model.AgentPi)
	}
}

func TestDefaultRegistryIncludesPi(t *testing.T) {
	registry, err := NewDefaultRegistry()
	if err != nil {
		t.Fatalf("NewDefaultRegistry() returned error: %v", err)
	}

	adapter, ok := registry.Get(model.AgentPi)
	if !ok {
		t.Fatalf("registry missing %s adapter", model.AgentPi)
	}

	if got := adapter.Agent(); got != model.AgentPi {
		t.Fatalf("registry adapter.Agent() = %q, want %q", got, model.AgentPi)
	}
}

func TestDefaultRegistrySupportedAgentsMatchesFactoryAgents(t *testing.T) {
	registry, err := NewDefaultRegistry()
	if err != nil {
		t.Fatalf("NewDefaultRegistry() returned error: %v", err)
	}

	want := []model.AgentID{
		model.AgentAider,
		model.AgentAmazonQ,
		model.AgentAmp,
		model.AgentAntigravity,
		model.AgentBlackbox,
		model.AgentBolt,
		model.AgentClaudeCode,
		model.AgentCline,
		model.AgentCodex,
		model.AgentCody,
		model.AgentConductor,
		model.AgentContinue,
		model.AgentCursor,
		model.AgentDevin,
		model.AgentGeminiCLI,
		model.AgentCopilot,
		model.AgentHermes,
		model.AgentJetBrains,
		model.AgentJunie,
		model.AgentKilocode,
		model.AgentKimi,
		model.AgentKiroIDE,
		model.AgentLoopoperators,
		model.AgentLovable,
		model.AgentOpenClaw,
		model.AgentOpenCode,
		model.AgentOpenHands,
		model.AgentPearAI,
		model.AgentPi,
		model.AgentQodo,
		model.AgentQwenCode,
		model.AgentReplit,
		model.AgentRooCode,
		model.AgentRunCell,
		model.AgentTabnine,
		model.AgentTrae,
		model.AgentV0,
		model.AgentVoid,
		model.AgentVSCodeCopilot,
		model.AgentWindsurf,
		model.AgentZCode,
		model.AgentZedAI,
	}

	if got := registry.SupportedAgents(); !reflect.DeepEqual(got, want) {
		t.Fatalf("SupportedAgents() = %v, want %v", got, want)
	}
}

func TestFactoryRejectsUnsupportedOpenClawLookalike(t *testing.T) {
	_, err := NewAdapter(model.AgentID("openclaw-beta"))
	if err == nil {
		t.Fatalf("NewAdapter() expected unsupported agent error")
	}

	if !errors.Is(err, ErrAgentNotSupported) {
		t.Fatalf("NewAdapter() error = %v, want ErrAgentNotSupported", err)
	}
}
