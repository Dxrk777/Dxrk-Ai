// SPDX-License-Identifier: MIT
package mcp

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/Dxrk777/Dxrk-Ai/internal/log"
	"github.com/Dxrk777/Dxrk-Ai/internal/tools"
)

func newMockTransport_GT(t *testing.T, defs []ToolDefinition) *mockTransport {
	t.Helper()
	mt := newMockTransport(t)
	mt.handle("initialize", func(params json.RawMessage) (any, error) {
		return InitializeResult{
			ProtocolVersion: "2024-11-05",
			Capabilities: &Capabilities{
				Tools: &ToolsCapabilities{ListChanged: true},
			},
			ServerInfo: Implementation{Name: "mock-server", Version: "1.0.0"},
		}, nil
	})
	mt.handle("tools/list", func(params json.RawMessage) (any, error) {
		return ListToolsResult{Tools: defs}, nil
	})
	mt.handle("tools/call", func(params json.RawMessage) (any, error) {
		return CallToolResult{
			Content: []ToolResultContent{
				{Type: "text", Text: "mock result"},
			},
		}, nil
	})
	return mt
}

func TestGateway_Connect_SingleServer(t *testing.T) {
	defs := []ToolDefinition{
		{Name: "hello", Description: "Says hello", InputSchema: ToolSchema{Type: "object"}},
	}
	mt := newMockTransport_GT(t, defs)
	cfg := ServerConfig{Name: "test", Transport: mt}
	g := NewGateway([]ServerConfig{cfg}, log.NewSlog(slog.Default()), nil)

	toolList, err := g.Connect(context.Background())
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	if len(toolList) != 1 {
		t.Fatalf("Connect() returned %d tools, want 1", len(toolList))
	}
	if toolList[0].ServerName != "test" {
		t.Fatalf("ServerName = %q, want %q", toolList[0].ServerName, "test")
	}
	if toolList[0].ToolName != "hello" {
		t.Fatalf("ToolName = %q, want %q", toolList[0].ToolName, "hello")
	}
	connected := g.ConnectedServers()
	if len(connected) != 1 || connected[0] != "test" {
		t.Fatalf("ConnectedServers() = %v, want [test]", connected)
	}
}

func TestGateway_Connect_MultipleServers(t *testing.T) {
	mt1 := newMockTransport_GT(t, []ToolDefinition{
		{Name: "tool_a", Description: "Tool A", InputSchema: ToolSchema{Type: "object"}},
	})
	mt2 := newMockTransport_GT(t, []ToolDefinition{
		{Name: "tool_b", Description: "Tool B", InputSchema: ToolSchema{Type: "object"}},
	})

	g := NewGateway([]ServerConfig{
		{Name: "server_a", Transport: mt1},
		{Name: "server_b", Transport: mt2},
	}, log.NewSlog(slog.Default()), nil)

	toolList, err := g.Connect(context.Background())
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	if len(toolList) != 2 {
		t.Fatalf("Connect() returned %d tools, want 2", len(toolList))
	}
}

func TestGateway_Connect_NoServers(t *testing.T) {
	g := NewGateway(nil, log.NewSlog(slog.Default()), nil)
	_, err := g.Connect(context.Background())
	if err != nil {
		t.Fatalf("Connect() with no servers should return nil, got error: %v", err)
	}
}

func TestGateway_Connect_PartialFailure(t *testing.T) {
	mt := newMockTransport_GT(t, []ToolDefinition{
		{Name: "working", Description: "Works", InputSchema: ToolSchema{Type: "object"}},
	})
	g := NewGateway([]ServerConfig{
		{Name: "good", Transport: mt},
		{Name: "bad", Transport: nil},
	}, log.NewSlog(slog.Default()), nil)

	toolList, err := g.Connect(context.Background())
	if err != nil {
		t.Fatalf("Connect() should not fail with partial success, got: %v", err)
	}
	if len(toolList) != 1 {
		t.Fatalf("Connect() returned %d tools, want 1", len(toolList))
	}
}

func TestGateway_RegisterTools(t *testing.T) {
	defs := []ToolDefinition{
		{
			Name: "greet", Description: "Greets someone",
			InputSchema: ToolSchema{
				Type: "object",
				Properties: map[string]ToolProperty{
					"name": {Type: "string", Description: "Name to greet"},
				},
			},
		},
	}
	mt := newMockTransport_GT(t, defs)
	cfg := ServerConfig{Name: "hello", Transport: mt}
	g := NewGateway([]ServerConfig{cfg}, log.NewSlog(slog.Default()), nil)

	_, err := g.Connect(context.Background())
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	reg := tools.New()
	count, err := g.RegisterTools(reg)
	if err != nil {
		t.Fatalf("RegisterTools() error = %v", err)
	}
	if count != 1 {
		t.Fatalf("Registered %d tools, want 1", count)
	}
	tool, ok := reg.Get("mcp_hello_greet")
	if !ok {
		t.Fatal("Registered tool mcp_hello_greet not found")
	}
	if tool.Description() != "Greets someone" {
		t.Fatalf("Description = %q, want %q", tool.Description(), "Greets someone")
	}
}

func TestGateway_RegisterTools_Call(t *testing.T) {
	defs := []ToolDefinition{
		{Name: "ping", Description: "Pings the server", InputSchema: ToolSchema{Type: "object"}},
	}
	mt := newMockTransport_GT(t, defs)
	cfg := ServerConfig{Name: "test", Transport: mt}
	g := NewGateway([]ServerConfig{cfg}, log.NewSlog(slog.Default()), nil)

	_, err := g.Connect(context.Background())
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	reg := tools.New()
	_, err = g.RegisterTools(reg)
	if err != nil {
		t.Fatalf("RegisterTools() error = %v", err)
	}

	tool, ok := reg.Get("mcp_test_ping")
	if !ok {
		t.Fatal("mcp_test_ping not found")
	}
	result, err := tool.Execute(tools.Context{Context: context.Background()}, nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result == nil {
		t.Fatal("Execute() returned nil result")
	}
}

func TestGateway_Disconnect(t *testing.T) {
	defs := []ToolDefinition{
		{Name: "x", Description: "X", InputSchema: ToolSchema{Type: "object"}},
	}
	mt := newMockTransport_GT(t, defs)
	cfg := ServerConfig{Name: "x", Transport: mt}
	g := NewGateway([]ServerConfig{cfg}, log.NewSlog(slog.Default()), nil)

	_, err := g.Connect(context.Background())
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	g.Disconnect()
	if len(g.ConnectedServers()) != 0 {
		t.Fatal("Disconnect() did not clear connected servers")
	}
}

func TestDiscoverServers_EmptyHome(t *testing.T) {
	home := t.TempDir()
	servers := DiscoverServers(home)
	if len(servers) != 0 {
		t.Fatalf("DiscoverServers() = %d, want 0", len(servers))
	}
}

func TestDiscoverServers_WithConfig(t *testing.T) {
	home := t.TempDir()
	mcpDir := filepath.Join(home, ".opencode")
	_ = os.MkdirAll(mcpDir, 0o750)
	mcpPath := filepath.Join(mcpDir, "mcp.json")
	_ = os.WriteFile(mcpPath, []byte(`{"mcpServers":{"test-srv":{"command":"python3","args":["-m","server"]}}}`), 0o600)

	servers := DiscoverServers(home)
	if len(servers) != 1 {
		t.Fatalf("DiscoverServers() = %d, want 1", len(servers))
	}
	if servers[0].Name != "test-srv" {
		t.Fatalf("Name = %q, want %q", servers[0].Name, "test-srv")
	}
}

func TestDiscoverServers_DeduplicatesByName(t *testing.T) {
	home := t.TempDir()
	_ = os.MkdirAll(filepath.Join(home, ".opencode"), 0o750)
	_ = os.WriteFile(filepath.Join(home, ".opencode", "mcp.json"),
		[]byte(`{"mcpServers":{"dup":{"command":"python3","args":["a"]}}}`), 0o600)
	_ = os.MkdirAll(filepath.Join(home, ".config", "opencode"), 0o750)
	_ = os.WriteFile(filepath.Join(home, ".config", "opencode", "mcp.json"),
		[]byte(`{"mcpServers":{"dup":{"command":"python3","args":["b"]}}}`), 0o600)

	servers := DiscoverServers(home)
	for _, s := range servers {
		if s.Name == "dup" {
			// Only one "dup" should exist
			if len(s.Args) == 0 || s.Args[0] != "a" {
				t.Fatalf("expected first occurrence of dup with arg 'a', got %v", s.Args)
			}
		}
	}
	dupCount := 0
	for _, s := range servers {
		if s.Name == "dup" {
			dupCount++
		}
	}
	if dupCount > 1 {
		t.Fatalf("duplicate server name not deduplicated: %d occurrences", dupCount)
	}
}

func TestParseMCPConfig_MissingFile(t *testing.T) {
	cfgs, err := parseMCPConfig("/nonexistent/path.json")
	if err != nil {
		t.Fatalf("parseMCPConfig() error = %v", err)
	}
	if len(cfgs) != 0 {
		t.Fatalf("parseMCPConfig() = %d, want 0", len(cfgs))
	}
}

func TestParseMCPConfig_FlatFormat(t *testing.T) {
	f := filepath.Join(t.TempDir(), "mcp.json")
	_ = os.WriteFile(f, []byte(`{"srv1":{"command":"node","args":["server.js"]}}`), 0o600)

	cfgs, err := parseMCPConfig(f)
	if err != nil {
		t.Fatalf("parseMCPConfig() error = %v", err)
	}
	if len(cfgs) != 1 || cfgs[0].Name != "srv1" {
		t.Fatalf("expected 1 server named srv1, got %d", len(cfgs))
	}
}

func TestGateway_DuplicateToolName(t *testing.T) {
	defs := []ToolDefinition{
		{Name: "dup", Description: "Dup 1", InputSchema: ToolSchema{Type: "object"}},
		{Name: "dup", Description: "Dup 2", InputSchema: ToolSchema{Type: "object"}},
	}
	mt := newMockTransport_GT(t, defs)
	cfg := ServerConfig{Name: "srv", Transport: mt}
	g := NewGateway([]ServerConfig{cfg}, log.NewSlog(slog.Default()), nil)

	_, err := g.Connect(context.Background())
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	reg := tools.New()
	count, err := g.RegisterTools(reg)
	if err != nil {
		t.Fatalf("RegisterTools() error = %v", err)
	}
	if count != 1 {
		t.Fatalf("Registered %d tools, want 1 (second should fail as duplicate)", count)
	}
}
