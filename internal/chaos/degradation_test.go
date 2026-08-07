// SPDX-License-Identifier: MIT
package chaos

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"

	"github.com/Dxrk777/Dxrk/internal/log"

	"github.com/Dxrk777/Dxrk/internal/agents"
	"github.com/Dxrk777/Dxrk/internal/mcp"
	"github.com/Dxrk777/Dxrk/internal/query"
	"github.com/Dxrk777/Dxrk/internal/tools"
	"github.com/Dxrk777/Dxrk/internal/tools/dxrk"
)

func TestGracefulDegradation(t *testing.T) {
	t.Run("dxrk-memory_not_installed", func(t *testing.T) {
		origPath := os.Getenv("PATH")
		t.Cleanup(func() { _ = os.Setenv("PATH", origPath) })
		_ = os.Setenv("PATH", t.TempDir())

		backend, err := query.NewDxrkMemoryBackend(context.Background())
		if err != nil {
			t.Fatalf("expected nil error when dxrk-memory is missing, got: %v", err)
		}
		if backend != nil {
			t.Fatal("expected nil backend when dxrk-memory is missing, got non-nil")
		}
	})

	t.Run("rg_not_installed", func(t *testing.T) {
		origPath := os.Getenv("PATH")
		t.Cleanup(func() { _ = os.Setenv("PATH", origPath) })
		_ = os.Setenv("PATH", t.TempDir())

		reg := tools.New()
		agentReg, err := agents.NewDefaultRegistry()
		if err != nil {
			t.Fatalf("NewDefaultRegistry() error = %v", err)
		}
		if err := dxrk.RegisterAll(reg, agentReg); err != nil {
			t.Fatalf("RegisterAll() error = %v", err)
		}

		for _, name := range []string{"grep_search", "glob_search"} {
			tool, ok := reg.Get(name)
			if !ok {
				t.Fatalf("tool %q not found", name)
			}
			var input map[string]any
			if name == "grep_search" {
				input = map[string]any{"pattern": "test"}
			} else {
				input = map[string]any{"pattern": "*.go"}
			}
			result, err := tool.Execute(tools.Context{Context: context.Background()}, input)
			if err != nil {
				t.Fatalf("%s Execute() error = %v", name, err)
			}
			r, ok := result.(map[string]any)
			if !ok {
				t.Fatalf("%s result type = %T, want map[string]any", name, result)
			}
			errMsg, hasErr := r["error"]
			if !hasErr || errMsg == "" {
				t.Fatalf("%s expected graceful error when rg is missing", name)
			}
		}
	})

	t.Run("invalid_api_key", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":{"type":"authentication_error","message":"invalid x-api-key"}}`))
		}))
		t.Cleanup(server.Close)

		provider := query.NewAnthropicProvider(
			"sk-invalid-key",
			"claude-sonnet-4-20250514",
			query.WithAnthropicBaseURL(server.URL),
			query.WithAnthropicClient(server.Client()),
		)

		reg := tools.New()
		loop := query.New(provider, reg)
		_, err := loop.Run(context.Background(), []query.Message{
			{Role: query.RoleUser, Content: "hello"},
		})

		if err == nil {
			t.Fatal("expected error for invalid API key, got nil")
		}
	})

	t.Run("mcp_server_crash", func(t *testing.T) {
		crash := &crashTransport{
			handlers: map[string]func(json.RawMessage) (any, error){},
		}
		crash.handle("initialize", func(_ json.RawMessage) (any, error) {
			return mcp.InitializeResult{
				ProtocolVersion: "2024-11-05",
				ServerInfo:      mcp.Implementation{Name: "crash-test", Version: "1.0.0"},
			}, nil
		})
		crash.handle("tools/list", func(_ json.RawMessage) (any, error) {
			return mcp.ListToolsResult{
				Tools: []mcp.ToolDefinition{
					{Name: "ping", Description: "Ping", InputSchema: mcp.ToolSchema{Type: "object"}},
				},
			}, nil
		})

		g := mcp.NewGateway([]mcp.ServerConfig{
			{Name: "crash-srv", Transport: crash},
		}, log.NewSlog(slog.Default()), nil)

		_, err := g.Connect(context.Background())
		if err != nil {
			t.Fatalf("Connect() error = %v", err)
		}

		reg := tools.New()
		_, err = g.RegisterTools(reg)
		if err != nil {
			t.Fatalf("RegisterTools() error = %v", err)
		}

		tool, ok := reg.Get("mcp_crash-srv_ping")
		if !ok {
			t.Fatal("mcp_crash-srv_ping not registered")
		}

		crash.fail.Store(true)

		_, err = tool.Execute(tools.Context{Context: context.Background()}, nil)
		if err == nil {
			t.Fatal("expected error from crashed MCP server, got nil")
		}

		g.Disconnect()
	})

	t.Run("compressor_nil_safety", func(t *testing.T) {
		reg := tools.New()
		provider := &mockProvider{
			responses: []mockResponse{
				{text: "Hello!", usage: query.Usage{InputTokens: 5, OutputTokens: 5}},
			},
		}
		loop := query.New(provider, reg)

		result, err := loop.Run(context.Background(), []query.Message{
			{Role: query.RoleUser, Content: "say hi"},
		})
		if err != nil {
			t.Fatalf("Run() with nil compressor error = %v", err)
		}
		if result.StopReason != query.StopSuccess {
			t.Fatalf("StopReason = %q, want %q", result.StopReason, query.StopSuccess)
		}
	})

	t.Run("checker_nil_safety", func(t *testing.T) {
		tCtx := tools.Context{Context: context.Background()}
		if tCtx.PermissionChecker != nil {
			t.Fatal("expected nil PermissionChecker by default")
		}
		if tCtx.PermissionAudit != nil {
			t.Fatal("expected nil PermissionAudit by default")
		}

		reg := tools.New()
		provider := &mockProvider{
			responses: []mockResponse{
				{text: "Hello!", usage: query.Usage{InputTokens: 5, OutputTokens: 5}},
			},
		}
		loop := query.New(provider, reg)

		result, err := loop.Run(context.Background(), []query.Message{
			{Role: query.RoleUser, Content: "say hi"},
		})
		if err != nil {
			t.Fatalf("Run() with nil checker error = %v", err)
		}
		if result.StopReason != query.StopSuccess {
			t.Fatalf("StopReason = %q, want %q", result.StopReason, query.StopSuccess)
		}
	})

	t.Run("persistence_nil_safety", func(t *testing.T) {
		reg := tools.New()
		if err := reg.Register(newTestTool(t, "calc", false, "42")); err != nil {
			t.Fatalf("Register error = %v", err)
		}

		provider := &mockProvider{
			responses: []mockResponse{
				{
					text: "calc",
					toolUses: []query.ToolUseBlock{
						{ID: "tu_1", Name: "calc", Input: map[string]any{}, Index: 0},
					},
				},
				{text: "done"},
			},
		}

		var called bool
		loop := query.New(provider, reg,
			query.WithTurnCallback(func(turn, msgs, tools int) {
				called = true
			}),
		)

		result, err := loop.Run(context.Background(), []query.Message{
			{Role: query.RoleUser, Content: "calculate"},
		})
		if err != nil {
			t.Fatalf("Run() with nil persistence error = %v", err)
		}
		if !called {
			t.Fatal("expected turn callback to be called")
		}
		if result.StopReason != query.StopSuccess {
			t.Fatalf("StopReason = %q, want %q", result.StopReason, query.StopSuccess)
		}
	})
}

type mockProvider struct {
	responses []mockResponse
	cursor    atomic.Int32
}

type mockResponse struct {
	text     string
	toolUses []query.ToolUseBlock
	usage    query.Usage
}

func (m *mockProvider) Generate(_ context.Context, _ []query.Message, _ []query.ToolSchema) (query.Response, error) {
	i := int(m.cursor.Add(1)) - 1
	if i >= len(m.responses) {
		r := m.responses[len(m.responses)-1]
		return query.Response{Text: r.text, ToolUses: copyToolUses(r.toolUses), Usage: r.usage}, nil
	}
	r := m.responses[i]
	return query.Response{Text: r.text, ToolUses: copyToolUses(r.toolUses), Usage: r.usage}, nil
}

func copyToolUses(src []query.ToolUseBlock) []query.ToolUseBlock {
	out := make([]query.ToolUseBlock, len(src))
	copy(out, src)
	return out
}

func newTestTool(t *testing.T, name string, concurrentSafe bool, result string) tools.Tool {
	t.Helper()
	tool, err := tools.Build(tools.ToolDef{
		Name:             name,
		Description:      "test tool " + name,
		IsConcurrentSafe: &concurrentSafe,
		IsReadOnly:       &concurrentSafe,
		Execute: func(ctx tools.Context, input map[string]any) (any, error) {
			return result, nil
		},
	})
	if err != nil {
		t.Fatalf("Build(%q) error = %v", name, err)
	}
	return tool
}

type crashTransport struct {
	handlers map[string]func(json.RawMessage) (any, error)
	fail     atomic.Bool
}

func (t *crashTransport) handle(method string, fn func(json.RawMessage) (any, error)) {
	t.handlers[method] = fn
}

func (t *crashTransport) Send(_ context.Context, msg json.RawMessage) (json.RawMessage, error) {
	if t.fail.Load() {
		return nil, fmt.Errorf("mcp transport: connection closed")
	}
	var req struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params"`
	}
	if err := json.Unmarshal(msg, &req); err != nil {
		return nil, err
	}
	fn, ok := t.handlers[req.Method]
	if !ok {
		resp := mcpRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &mcpRPCError{Code: -32601, Message: "method not found: " + req.Method},
		}
		return json.Marshal(resp)
	}
	result, err := fn(req.Params)
	if err != nil {
		resp := mcpRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &mcpRPCError{Code: -32000, Message: err.Error()},
		}
		return json.Marshal(resp)
	}
	resultBytes, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	resp := mcpRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  resultBytes,
	}
	return json.Marshal(resp)
}

func (t *crashTransport) Close() error { return nil }

type mcpRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *mcpRPCError    `json:"error,omitempty"`
}

type mcpRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

var _ mcp.Transport = (*crashTransport)(nil)
