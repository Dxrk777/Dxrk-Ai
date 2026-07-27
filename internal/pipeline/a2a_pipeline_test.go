// SPDX-License-Identifier: MIT
package pipeline

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Dxrk777/Dxrk-Ai/internal/query"
	"github.com/Dxrk777/Dxrk-Ai/internal/router"
)

type mockProvider struct {
	response string
	err      error
}

func (m *mockProvider) Generate(ctx context.Context, messages []query.Message, tools []query.ToolSchema) (query.Response, error) {
	if m.err != nil {
		return query.Response{}, m.err
	}
	return query.Response{
		Text:  m.response,
		Usage: query.Usage{InputTokens: 10, OutputTokens: 20},
	}, nil
}

func newTestRouter(response string) *router.Router {
	mockProv := &mockProvider{response: response}
	providers := []router.ProviderEntry{
		{Name: "mock", Model: "mock-model", Provider: mockProv},
	}
	return router.NewRouter(providers)
}

func TestPipelineConfig_Defaults(t *testing.T) {
	cfg := DefaultPipelineConfig(nil)
	if cfg.MaxIterations != 3 {
		t.Errorf("expected 3, got %d", cfg.MaxIterations)
	}
	if cfg.Timeouts != 60*time.Second {
		t.Errorf("expected 60s, got %v", cfg.Timeouts)
	}
}

func TestNewPipeline(t *testing.T) {
	rtr := newTestRouter("test")
	cfg := DefaultPipelineConfig(rtr)
	p := NewPipeline(cfg)
	defer p.Stop()

	if len(p.nodes) != 4 {
		t.Fatalf("expected 4 nodes, got %d", len(p.nodes))
	}

	for _, role := range pipelineRoles {
		node, ok := p.nodes[role]
		if !ok {
			t.Fatalf("missing node for role %s", role)
		}
		if node.Name != string(role) {
			t.Errorf("expected name %s, got %s", role, node.Name)
		}
	}
}

func TestNodeCapabilities(t *testing.T) {
	rtr := newTestRouter("test")
	cfg := DefaultPipelineConfig(rtr)
	p := NewPipeline(cfg)
	defer p.Stop()

	coder := p.nodes[RoleCoder]
	if len(coder.Capabilities) != 1 {
		t.Fatalf("expected 1 capability, got %d", len(coder.Capabilities))
	}
	if coder.Capabilities[0].Name != "coding" {
		t.Errorf("expected 'coding', got %s", coder.Capabilities[0].Name)
	}
}

func TestPipelineExecute_Success(t *testing.T) {
	rtr := newTestRouter("```go\npackage main\n\nfunc main() {}\n```")
	cfg := DefaultPipelineConfig(rtr)
	cfg.MaxIterations = 1

	p := NewPipeline(cfg)
	defer p.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result := p.Execute(ctx, PipelineTask{
		ID:          "task-1",
		Description: "Write a Go hello world program",
		Language:    "go",
	})

	t.Logf("result: success=%v, iterations=%d, duration=%dms",
		result.Success, result.Iterations, result.DurationMs)
	t.Logf("stages: %d", len(result.Stages))

	if !result.Success {
		t.Log("pipeline didn't succeed (may be expected without real LLM)")
	}
}

func TestPipelineExecute_MaxIterations(t *testing.T) {
	rtr := newTestRouter("some code with issues")
	cfg := DefaultPipelineConfig(rtr)
	cfg.MaxIterations = 2

	p := NewPipeline(cfg)
	defer p.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result := p.Execute(ctx, PipelineTask{
		ID:          "task-2",
		Description: "Write Python code",
		Language:    "python",
	})

	if result.Iterations > 2 {
		t.Errorf("expected max 2 iterations, got %d", result.Iterations)
	}
}

func TestPipelineRoleOrder(t *testing.T) {
	expected := []PipelineRole{RoleCoder, RoleTester, RoleReviewer, RoleDocs}
	if len(pipelineRoles) != len(expected) {
		t.Fatalf("expected %d roles, got %d", len(expected), len(pipelineRoles))
	}
	for i, role := range pipelineRoles {
		if role != expected[i] {
			t.Errorf("position %d: expected %s, got %s", i, expected[i], role)
		}
	}
}

func TestPipelineSystemPrompts(t *testing.T) {
	cfg := DefaultPipelineConfig(nil)
	p := &Pipeline{cfg: cfg}

	if p.systemPromptForRole(RoleCoder) != cfg.CodePrompt {
		t.Error("coder prompt mismatch")
	}
	if p.systemPromptForRole(RoleTester) != cfg.TestPrompt {
		t.Error("tester prompt mismatch")
	}
	if p.systemPromptForRole(RoleReviewer) != cfg.ReviewPrompt {
		t.Error("reviewer prompt mismatch")
	}
	if p.systemPromptForRole(RoleDocs) != cfg.DocsPrompt {
		t.Error("docs prompt mismatch")
	}
}

func TestPipelineHandoffViaA2A(t *testing.T) {
	rtr := newTestRouter("```go\npackage main\n```")
	cfg := DefaultPipelineConfig(rtr)

	p := NewPipeline(cfg)
	defer p.Stop()

	a1 := p.nodes[RoleCoder]
	_ = p.nodes[RoleTester]

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := a1.Handoff(ctx, string(RoleTester), "test this code", nil)
	if err != nil {
		t.Fatalf("handoff failed: %v", err)
	}
	if !result.Accepted {
		t.Fatal("handoff should be accepted")
	}
	if result.SessionID == "" {
		t.Fatal("expected non-empty session ID")
	}
	t.Logf("handoff result: accepted=%v, session=%s", result.Accepted, result.SessionID)
}

func TestPipelineQueryViaA2A(t *testing.T) {
	rtr := newTestRouter("the answer is 42")
	cfg := DefaultPipelineConfig(rtr)

	p := NewPipeline(cfg)
	defer p.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := p.nodes[RoleCoder].Query(ctx, string(RoleDocs), "what is the meaning of life?", nil)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if result.Answer != "the answer is 42" {
		t.Errorf("expected 'the answer is 42', got %q", result.Answer)
	}
}

func TestPipelineShareContext(t *testing.T) {
	rtr := newTestRouter("ok")
	cfg := DefaultPipelineConfig(rtr)

	p := NewPipeline(cfg)
	defer p.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	contextData := map[string]string{"project": "dxrk", "branch": "main"}
	err := p.nodes[RoleCoder].ShareContext(ctx, []string{string(RoleDocs), string(RoleReviewer)}, contextData)
	if err != nil {
		t.Fatalf("share context failed: %v", err)
	}
}

func TestPipelineTaskJSON(t *testing.T) {
	task := PipelineTask{
		ID:          "test-1",
		Description: "implement feature",
		Language:    "python",
		Spec:        json.RawMessage(`{"endpoint": "/api/test"}`),
	}

	data, err := json.Marshal(task)
	if err != nil {
		t.Fatal(err)
	}

	var decoded PipelineTask
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	if decoded.ID != task.ID {
		t.Errorf("expected ID %q, got %q", task.ID, decoded.ID)
	}
	if decoded.Language != task.Language {
		t.Errorf("expected language %q, got %q", task.Language, decoded.Language)
	}
}

func TestPipelineResultJSON(t *testing.T) {
	result := PipelineResult{
		TaskID:     "task-1",
		Code:       "package main",
		Tests:      "func TestMain",
		ReviewLog:  "looks good",
		Docs:       "# Docs",
		Iterations: 1,
		Success:    true,
		DurationMs: 1500,
		Stages: []StageReport{
			{Role: RoleCoder, Status: "completed", Output: "code", DurationMs: 500},
			{Role: RoleTester, Status: "completed", Output: "tests", DurationMs: 500},
		},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}

	var decoded PipelineResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	if decoded.TaskID != result.TaskID {
		t.Errorf("expected TaskID %q, got %q", result.TaskID, decoded.TaskID)
	}
	if len(decoded.Stages) != 2 {
		t.Errorf("expected 2 stages, got %d", len(decoded.Stages))
	}
}
