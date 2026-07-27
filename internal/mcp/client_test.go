// SPDX-License-Identifier: MIT
package mcp

import (
	"context"
	"encoding/json"
	"testing"
)

// mockTransport simulates an MCP server for testing.
type mockTransport struct {
	t        *testing.T
	handlers map[string]func(params json.RawMessage) (any, error)
}

func newMockTransport(t *testing.T) *mockTransport {
	return &mockTransport{
		t:        t,
		handlers: make(map[string]func(params json.RawMessage) (any, error)),
	}
}

func (m *mockTransport) handle(method string, fn func(params json.RawMessage) (any, error)) {
	m.handlers[method] = fn
}

func (m *mockTransport) Send(ctx context.Context, msg json.RawMessage) (json.RawMessage, error) {
	var req jsonRPCRequest
	if err := json.Unmarshal(msg, &req); err != nil {
		return nil, err
	}

	fn, ok := m.handlers[req.Method]
	if !ok {
		resp := jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &jsonRPCError{
				Code:    -32601,
				Message: "method not found: " + req.Method,
			},
		}
		return json.Marshal(resp)
	}

	result, err := fn(req.Params)
	if err != nil {
		resp := jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &jsonRPCError{
				Code:    -32000,
				Message: err.Error(),
			},
		}
		return json.Marshal(resp)
	}

	resultBytes, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}

	resp := jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  resultBytes,
	}
	return json.Marshal(resp)
}

func (m *mockTransport) Close() error { return nil }

func TestClient_Initialize(t *testing.T) {
	mt := newMockTransport(t)
	mt.handle("initialize", func(params json.RawMessage) (any, error) {
		return InitializeResult{
			ProtocolVersion: "2024-11-05",
			Capabilities: &Capabilities{
				Tools: &ToolsCapabilities{ListChanged: true},
			},
			ServerInfo: Implementation{Name: "test-server", Version: "1.0.0"},
		}, nil
	})

	client := NewClient(mt)
	result, err := client.Initialize(context.Background())
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	if result.ProtocolVersion != "2024-11-05" {
		t.Fatalf("ProtocolVersion = %q, want %q", result.ProtocolVersion, "2024-11-05")
	}
	if result.ServerInfo.Name != "test-server" {
		t.Fatalf("ServerInfo.Name = %q, want %q", result.ServerInfo.Name, "test-server")
	}
	if result.Capabilities == nil || result.Capabilities.Tools == nil {
		t.Fatal("Capabilities.Tools is nil")
	}
}

func TestClient_ListTools(t *testing.T) {
	mt := newMockTransport(t)
	mt.handle("initialize", func(params json.RawMessage) (any, error) {
		return InitializeResult{ServerInfo: Implementation{Name: "test", Version: "1"}}, nil
	})
	mt.handle("tools/list", func(params json.RawMessage) (any, error) {
		return ListToolsResult{
			Tools: []ToolDefinition{
				{Name: "tool-1", Description: "first tool"},
				{Name: "tool-2", Description: "second tool"},
			},
		}, nil
	})

	client := NewClient(mt)
	if _, err := client.Initialize(context.Background()); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	tools, err := client.ListTools(context.Background())
	if err != nil {
		t.Fatalf("ListTools() error = %v", err)
	}
	if len(tools) != 2 {
		t.Fatalf("len(tools) = %d, want 2", len(tools))
	}
	if tools[0].Name != "tool-1" {
		t.Fatalf("tools[0].Name = %q, want %q", tools[0].Name, "tool-1")
	}
}

func TestClient_CallTool(t *testing.T) {
	mt := newMockTransport(t)
	mt.handle("initialize", func(params json.RawMessage) (any, error) {
		return InitializeResult{ServerInfo: Implementation{Name: "test", Version: "1"}}, nil
	})
	mt.handle("tools/call", func(params json.RawMessage) (any, error) {
		var p CallToolParams
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		return CallToolResult{
			Content: []ToolResultContent{
				{Type: "text", Text: "hello " + p.Arguments["name"].(string)},
			},
		}, nil
	})

	client := NewClient(mt)
	if _, err := client.Initialize(context.Background()); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	result, err := client.CallTool(context.Background(), "greet", map[string]any{"name": "dxrk"})
	if err != nil {
		t.Fatalf("CallTool() error = %v", err)
	}
	if len(result.Content) != 1 {
		t.Fatalf("len(Content) = %d, want 1", len(result.Content))
	}
	if result.Content[0].Text != "hello dxrk" {
		t.Fatalf("Text = %q, want %q", result.Content[0].Text, "hello dxrk")
	}
}

func TestClient_ErrorResponse(t *testing.T) {
	mt := newMockTransport(t)
	mt.handle("initialize", func(params json.RawMessage) (any, error) {
		return InitializeResult{ServerInfo: Implementation{Name: "test", Version: "1"}}, nil
	})

	client := NewClient(mt)
	if _, err := client.Initialize(context.Background()); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	_, err := client.CallTool(context.Background(), "nonexistent", nil)
	if err == nil {
		t.Fatal("expected error for unknown method")
	}
	if _, ok := err.(*RPCError); !ok {
		t.Fatalf("error type = %T, want *RPCError", err)
	}
}

func TestClient_ServerInfo(t *testing.T) {
	mt := newMockTransport(t)
	mt.handle("initialize", func(params json.RawMessage) (any, error) {
		return InitializeResult{
			ServerInfo: Implementation{Name: "info-server", Version: "1.2.3"},
		}, nil
	})

	client := NewClient(mt)

	if client.ServerInfo() != nil {
		t.Fatal("ServerInfo() should be nil before Initialize")
	}

	if _, err := client.Initialize(context.Background()); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	info := client.ServerInfo()
	if info.ServerInfo.Name != "info-server" {
		t.Fatalf("Name = %q, want %q", info.ServerInfo.Name, "info-server")
	}
}

func TestClient_ListResources(t *testing.T) {
	mt := newMockTransport(t)
	mt.handle("initialize", func(params json.RawMessage) (any, error) {
		return InitializeResult{ServerInfo: Implementation{Name: "test", Version: "1"}}, nil
	})
	mt.handle("resources/list", func(params json.RawMessage) (any, error) {
		return ListResourcesResult{
			Resources: []ResourceDefinition{
				{URI: "file:///test.md", Name: "test", MimeType: "text/markdown"},
			},
		}, nil
	})

	client := NewClient(mt)
	if _, err := client.Initialize(context.Background()); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	resources, err := client.ListResources(context.Background())
	if err != nil {
		t.Fatalf("ListResources() error = %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("len(resources) = %d, want 1", len(resources))
	}
	if resources[0].URI != "file:///test.md" {
		t.Fatalf("URI = %q, want %q", resources[0].URI, "file:///test.md")
	}
}

func TestClient_ReadResource(t *testing.T) {
	mt := newMockTransport(t)
	mt.handle("initialize", func(params json.RawMessage) (any, error) {
		return InitializeResult{ServerInfo: Implementation{Name: "test", Version: "1"}}, nil
	})
	mt.handle("resources/read", func(params json.RawMessage) (any, error) {
		return ReadResourceResult{
			Contents: []ResourceContent{
				{URI: "file:///test.md", Text: "# Hello"},
			},
		}, nil
	})

	client := NewClient(mt)
	if _, err := client.Initialize(context.Background()); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	result, err := client.ReadResource(context.Background(), "file:///test.md")
	if err != nil {
		t.Fatalf("ReadResource() error = %v", err)
	}
	if len(result.Contents) != 1 {
		t.Fatalf("len(Contents) = %d, want 1", len(result.Contents))
	}
	if result.Contents[0].Text != "# Hello" {
		t.Fatalf("Text = %q, want %q", result.Contents[0].Text, "# Hello")
	}
}
