// SPDX-License-Identifier: MIT
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/Dxrk777/Dxrk/internal/tools"
)

func rawJSON(s string) json.RawMessage {
	return json.RawMessage(s)
}

// pipeServer creates a server with pipe-based IO for testing.
// Returns the server, a writer for requests, and a reader for responses.
func pipeServer(t *testing.T, reg *tools.Registry) (*Server, *io.PipeWriter, *io.PipeReader) {
	t.Helper()
	reqR, reqW := io.Pipe()
	respR, respW := io.Pipe()
	server := NewServer(reg, reqR, respW)
	return server, reqW, respR
}

func sendRequest(t *testing.T, w *io.PipeWriter, method string, params, id json.RawMessage) {
	t.Helper()
	req := jsonRPCRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}
	if len(id) > 0 {
		req.ID = id
	}
	data, _ := json.Marshal(req)
	data = append(data, '\n')
	_, _ = w.Write(data)
}

func readResponse(t *testing.T, r *io.PipeReader, v any) {
	t.Helper()
	b := make([]byte, 16384)
	n, err := r.Read(b)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	if err := json.Unmarshal(b[:n], v); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
}

func TestServer_Initialize(t *testing.T) {
	reg := tools.New()
	server, reqW, respR := pipeServer(t, reg)

	ctx, cancel := context.WithCancel(testCtx(t))
	go func() {
		_ = server.Serve(ctx)
	}()

	sendRequest(t, reqW, "initialize", rawJSON(`{}`), rawJSON(`1`))

	var resp jsonRPCResponse
	readResponse(t, respR, &resp)
	if resp.Error != nil {
		t.Fatalf("initialize error: %+v", resp.Error)
	}
	cancel()
}

func TestServer_CallTool_NotFound(t *testing.T) {
	reg := tools.New()
	server, reqW, respR := pipeServer(t, reg)

	ctx, cancel := context.WithCancel(testCtx(t))
	go func() {
		_ = server.Serve(ctx)
	}()

	sendRequest(t, reqW, "tools/call", rawJSON(`{"name":"nonexistent"}`), rawJSON(`1`))

	var resp jsonRPCResponse
	readResponse(t, respR, &resp)
	if resp.Error == nil {
		t.Fatal("expected error for nonexistent tool")
	}
	cancel()
}

func TestServer_CallTool_Success(t *testing.T) {
	reg := tools.New()
	tool, err := tools.Build(tools.ToolDef{
		Name:        "echo",
		Description: "echoes input",
		Execute: func(ctx tools.Context, input map[string]any) (any, error) {
			return input["key"], nil
		},
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if err := reg.Register(tool); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	params, _ := json.Marshal(map[string]any{
		"name": "echo",
		"arguments": map[string]any{
			"key": "hello",
		},
	})
	s := NewServer(reg, nil, io.Discard)
	result, err := s.handleCallTool(params)
	if err != nil {
		t.Fatalf("handleCallTool() error = %v", err)
	}
	ctr, ok := result.(CallToolResult)
	if !ok {
		t.Fatalf("expected CallToolResult, got %T", result)
	}
	if ctr.IsError {
		t.Fatalf("IsError = true, want false: %s", ctr.Content[0].Text)
	}
}

func TestServer_ListTools_Empty(t *testing.T) {
	reg := tools.New()
	s := NewServer(reg, nil, io.Discard)
	result := s.handleListTools()
	ltr, ok := result.(listToolsResult)
	if !ok {
		t.Fatalf("expected listToolsResult, got %T", result)
	}
	if len(ltr.Tools) != 0 {
		t.Fatalf("expected 0 tools, got %d", len(ltr.Tools))
	}
}

func TestServer_ListTools_WithTools(t *testing.T) {
	reg := tools.New()
	tool, err := tools.Build(tools.ToolDef{
		Name:        "greet",
		Description: "greets someone",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{"type": "string", "description": "person to greet"},
			},
		},
		Execute: func(ctx tools.Context, input map[string]any) (any, error) {
			return "hello", nil
		},
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if err := reg.Register(tool); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	s := NewServer(reg, nil, io.Discard)
	result := s.handleListTools()
	ltr, ok := result.(listToolsResult)
	if !ok {
		t.Fatalf("expected listToolsResult, got %T", result)
	}
	if len(ltr.Tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(ltr.Tools))
	}
	if ltr.Tools[0].Name != "greet" {
		t.Fatalf("tool name = %q, want %q", ltr.Tools[0].Name, "greet")
	}
}

func TestRateLimiter_Allow(t *testing.T) {
	rl := NewRateLimiter(3, 100) // 3 tokens, fast refill
	for i := 0; i < 3; i++ {
		if !rl.Allow() {
			t.Fatalf("Allow() returned false on attempt %d, want true", i+1)
		}
	}
	// Fourth should be denied (bucket empty, refill hasn't happened yet)
	if rl.Allow() {
		t.Fatal("Allow() returned true after consuming all tokens, want false")
	}
}

func TestRateLimiter_Refill(t *testing.T) {
	rl := NewRateLimiter(2, 1000) // 2 tokens, 1000/sec refill
	rl.Allow()                    // 1 used
	rl.Allow()                    // 2 used
	if rl.Allow() {
		t.Fatal("Allow() returned true on empty bucket")
	}
	// Wait for refill
	time.Sleep(5 * time.Millisecond)
	if !rl.Allow() {
		t.Fatal("Allow() returned false after refill, want true")
	}
}

func TestServer_UnknownMethod(t *testing.T) {
	reg := tools.New()
	server, reqW, respR := pipeServer(t, reg)

	ctx, cancel := context.WithCancel(testCtx(t))
	go func() {
		_ = server.Serve(ctx)
	}()

	sendRequest(t, reqW, "bogus_method", nil, rawJSON(`1`))

	var resp jsonRPCResponse
	readResponse(t, respR, &resp)
	if resp.Error == nil {
		t.Fatal("expected error for unknown method")
	}
	cancel()
}

func TestServer_E2E_FullConversation(t *testing.T) {
	// Full end-to-end: initialize → tools/list → tools/call → response
	reg := tools.New()
	tool, err := tools.Build(tools.ToolDef{
		Name:        "greet",
		Description: "greets a user by name",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{"type": "string"},
			},
			"required": []string{"name"},
		},
		Execute: func(ctx tools.Context, input map[string]any) (any, error) {
			name, _ := input["name"].(string)
			return fmt.Sprintf("Hello, %s!", name), nil
		},
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if err := reg.Register(tool); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	server, reqW, respR := pipeServer(t, reg)
	ctx, cancel := context.WithCancel(testCtx(t))

	// Handler started
	done := make(chan error, 1)
	go func() {
		done <- server.Serve(ctx)
	}()

	// 1. Initialize
	sendRequest(t, reqW, "initialize", rawJSON(`{}`), rawJSON(`1`))
	var initResp jsonRPCResponse
	readResponse(t, respR, &initResp)
	if initResp.Error != nil {
		t.Fatalf("initialize error: %+v", initResp.Error)
	}

	// 2. tools/list
	sendRequest(t, reqW, "tools/list", nil, rawJSON(`2`))
	var listResp struct {
		Result struct {
			Tools []struct {
				Name        string `json:"name"`
				Description string `json:"description"`
			} `json:"tools"`
		} `json:"result"`
		JSONRPC string `json:"jsonrpc"`
		ID      int    `json:"id"`
	}
	readResponse(t, respR, &listResp)
	if len(listResp.Result.Tools) != 1 {
		t.Fatalf("tools = %d, want 1", len(listResp.Result.Tools))
	}
	if listResp.Result.Tools[0].Name != "greet" {
		t.Fatalf("tool name = %q, want %q", listResp.Result.Tools[0].Name, "greet")
	}

	// 3. tools/call with arguments
	sendRequest(t, reqW, "tools/call", rawJSON(`{"name":"greet","arguments":{"name":"World"}}`), rawJSON(`3`))
	var callResp struct {
		Result CallToolResult `json:"result"`
		ID     int            `json:"id"`
	}
	readResponse(t, respR, &callResp)
	if callResp.Result.IsError {
		t.Fatalf("IsError = true, want false: %s", callResp.Result.Content[0].Text)
	}
	if len(callResp.Result.Content) == 0 {
		t.Fatal("empty content")
	}
	if callResp.Result.Content[0].Text != `"Hello, World!"` {
		t.Fatalf("result = %q, want %q", callResp.Result.Content[0].Text, `"Hello, World!"`)
	}

	_ = reqW.Close()
	cancel()
	<-done
}

func testCtx(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return ctx
}
