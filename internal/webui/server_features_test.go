// SPDX-License-Identifier: MIT
package webui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Dxrk777/Dxrk/internal/config"
	"github.com/Dxrk777/Dxrk/internal/pipeline"
	"github.com/Dxrk777/Dxrk/internal/tools"
)

type fakePipelineResult struct {
	result pipeline.PipelineResult
}

func (f *fakePipelineResult) LastResult() pipeline.PipelineResult {
	return f.result
}

func TestStatusEndpoint_WithPipeline(t *testing.T) {
	srv := NewServer(&config.WebUIConfig{Host: "", Port: 8080}, nil, nil, nil, nil, nil,
		&fakePipelineResult{result: pipeline.PipelineResult{TaskID: "run-1", Iterations: 3, Success: true}}, nil)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	srv.HandleStatus(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp struct {
		Pipeline *struct {
			Running    bool   `json:"running"`
			LastRun    string `json:"last_run"`
			Iterations int    `json:"iterations"`
			Success    bool   `json:"success"`
		} `json:"pipeline"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Pipeline == nil {
		t.Fatal("expected pipeline to be populated")
	}
	if resp.Pipeline.Iterations != 3 {
		t.Errorf("expected 3 iterations, got %d", resp.Pipeline.Iterations)
	}
	if !resp.Pipeline.Success {
		t.Error("expected success=true")
	}
	if resp.Pipeline.LastRun != "run-1" {
		t.Errorf("expected last_run=run-1, got %q", resp.Pipeline.LastRun)
	}
}

func TestStatusEndpoint_PipelineStaysNilWithoutInstance(t *testing.T) {
	srv := NewServer(&config.WebUIConfig{Host: "", Port: 8080}, nil, nil, nil, nil, nil, nil, nil)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	srv.HandleStatus(w, r)

	var resp struct {
		Pipeline *json.RawMessage `json:"pipeline"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Pipeline != nil {
		t.Error("expected pipeline to be omitted when no instance is attached")
	}
}

func TestHandleTools_EmptyWithoutRegistry(t *testing.T) {
	srv := NewServer(&config.WebUIConfig{Host: "", Port: 8080}, nil, nil, nil, nil, nil, nil, nil)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/tools", nil)
	srv.HandleTools(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var list []toolInfo
	if err := json.NewDecoder(w.Body).Decode(&list); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("expected empty list, got %d entries", len(list))
	}
}

func TestHandleTools_ListsEnabledFromRegistry(t *testing.T) {
	reg := tools.New()
	tool, err := tools.Build(tools.ToolDef{
		Name:        "echo",
		Description: "repeats input",
		InputSchema: map[string]any{"type": "object"},
		Execute: func(ctx tools.Context, input map[string]any) (any, error) {
			return input, nil
		},
	})
	if err != nil {
		t.Fatalf("build tool: %v", err)
	}
	if err := reg.Register(tool); err != nil {
		t.Fatalf("register: %v", err)
	}

	srv := NewServer(&config.WebUIConfig{Host: "", Port: 8080}, nil, nil, nil, nil, nil, nil, nil).WithTools(reg)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/tools", nil)
	srv.HandleTools(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var list []toolInfo
	if err := json.NewDecoder(w.Body).Decode(&list); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(list))
	}
	if list[0].Name != "echo" || list[0].Description != "repeats input" {
		t.Errorf("unexpected tool info: %+v", list[0])
	}
	if !list[0].Enabled || list[0].ReadOnly {
		t.Errorf("expected enabled=true, read_only=false, got %+v", list[0])
	}
	if list[0].InputSchema == nil {
		t.Error("expected input_schema to be present")
	}
}

type fakeChecker struct {
	action  string
	target  string
	allowed bool
	reason  string
	checked bool
}

func (c *fakeChecker) Check(action, target string) (bool, string) {
	c.checked = true
	c.action = action
	c.target = target
	return c.allowed, c.reason
}

type fakeAudit struct {
	recorded bool
	action   string
	target   string
	allowed  bool
	reason   string
}

func (a *fakeAudit) Record(action, target string, allowed bool, reason string) {
	a.recorded = true
	a.action = action
	a.target = target
	a.allowed = allowed
	a.reason = reason
}

func TestHandleConfig_AllowsWithoutChecker(t *testing.T) {
	srv := NewServer(&config.WebUIConfig{Host: "", Port: 8080}, nil, nil, nil, nil, nil, nil, nil)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	srv.HandleConfig(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 without checker, got %d", w.Code)
	}
}

func TestHandleConfig_DeniedByChecker(t *testing.T) {
	checker := &fakeChecker{allowed: false, reason: "denied"}
	audit := &fakeAudit{}
	srv := NewServer(&config.WebUIConfig{Host: "", Port: 8080}, nil, nil, nil, nil, nil, nil, nil).
		WithPermissions(checker, audit)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	srv.HandleConfig(w, r)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
	if !checker.checked {
		t.Error("expected checker to be consulted")
	}
	if checker.action != "config.read" {
		t.Errorf("expected action config.read, got %q", checker.action)
	}
	if !audit.recorded || audit.allowed {
		t.Error("expected audit to record denied decision (allowed=false)")
	}
}

func TestHandleConfig_AllowedByChecker(t *testing.T) {
	checker := &fakeChecker{allowed: true}
	audit := &fakeAudit{}
	srv := NewServer(&config.WebUIConfig{Host: "", Port: 8080}, nil, nil, nil, nil, nil, nil, nil).
		WithPermissions(checker, audit)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	srv.HandleConfig(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if checker.action != "config.read" {
		t.Errorf("expected action config.read, got %q", checker.action)
	}
	if !audit.recorded || !audit.allowed {
		t.Error("expected audit to record granted decision")
	}
}

func TestHandleSettings_PostDeniedByChecker(t *testing.T) {
	checker := &fakeChecker{allowed: false, reason: "no write"}
	audit := &fakeAudit{}
	srv := NewServer(&config.WebUIConfig{Host: "", Port: 8080}, nil, nil, nil, nil, nil, nil, nil).
		WithPermissions(checker, audit)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/settings", nil)
	srv.HandleSettings(w, r)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
	if checker.action != "config.write" {
		t.Errorf("expected action config.write, got %q", checker.action)
	}
}
