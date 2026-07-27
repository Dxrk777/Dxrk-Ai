// SPDX-License-Identifier: MIT
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	dxrklog "github.com/Dxrk777/Dxrk-Ai/internal/log"
	"github.com/Dxrk777/Dxrk-Ai/internal/tools"
	"github.com/Dxrk777/Dxrk-Ai/internal/trace"

	"go.opentelemetry.io/otel/attribute"
)

// ServerConfig describes an external MCP server connection.
type ServerConfig struct {
	Name      string // unique name (e.g. "dxrk-memory", "context7")
	Command   string // executable path
	Args      []string
	Transport Transport
}

// Gateway connects to external MCP servers and registers their tools
// in a tools.Registry as first-class Dxrk tools.
type Gateway struct {
	mu      sync.RWMutex
	clients map[string]*Client
	servers map[string]ServerConfig
	logger  dxrklog.Logger
	tp      trace.Exporter
}

// NewGateway creates a Gateway.
func NewGateway(servers []ServerConfig, logger dxrklog.Logger, tp trace.Exporter) *Gateway {
	g := &Gateway{
		clients: make(map[string]*Client),
		servers: make(map[string]ServerConfig),
		logger:  logger,
		tp:      tp,
	}
	for _, s := range servers {
		g.servers[s.Name] = s
	}
	return g
}

// Connect initializes all configured servers and returns a list of all
// discovered tools. Callers should then use RegisterTool to add them.
func (g *Gateway) Connect(ctx context.Context) ([]GatewayTool, error) {
	var mu sync.Mutex
	var all []GatewayTool
	var errs []error

	for name, cfg := range g.servers {
		client, gwt, err := g.connectOne(ctx, name, cfg)
		mu.Lock()
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", name, err))
		} else {
			g.clients[name] = client
			all = append(all, gwt...)
		}
		mu.Unlock()
	}

	if len(errs) > 0 && len(all) == 0 {
		return nil, fmt.Errorf("all MCP servers failed: %v", errs)
	}
	return all, nil
}

func (g *Gateway) connectOne(ctx context.Context, name string, cfg ServerConfig) (*Client, []GatewayTool, error) {
	log := g.logger.With("server", name)

	var transport Transport
	switch {
	case cfg.Transport != nil:
		transport = cfg.Transport
	case cfg.Command != "":
		var err error
		transport, err = NewStdioTransport(ctx, cfg.Command, cfg.Args...)
		if err != nil {
			return nil, nil, fmt.Errorf("create transport: %w", err)
		}
	default:
		return nil, nil, fmt.Errorf("no transport or command specified")
	}

	client := NewClient(transport)
	info, err := client.Initialize(ctx)
	if err != nil {
		_ = transport.Close()
		return nil, nil, fmt.Errorf("initialize: %w", err)
	}
	log.Debug("MCP server initialized", "name", info.ServerInfo.Name, "version", info.ServerInfo.Version)

	defs, err := client.ListTools(ctx)
	if err != nil {
		_ = transport.Close()
		return nil, nil, fmt.Errorf("list tools: %w", err)
	}

	gwt := make([]GatewayTool, 0, len(defs))
	for _, d := range defs {
		inner := gatewayTool{
			serverName: name,
			toolName:   d.Name,
			client:     client,
			toolDef:    d,
			tp:         g.tp,
		}
		gwt = append(gwt, inner.toGatewayTool())
	}
	return client, gwt, nil
}

// RegisterTools registers all discovered MCP tools into the given registry.
// Tool names are prefixed with "mcp_{serverName}_" to avoid collisions.
// Returns the count of registered tools.
func (g *Gateway) RegisterTools(registry *tools.Registry) (int, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	registered := 0
	for name, client := range g.clients {
		defs, err := client.ListTools(context.Background())
		if err != nil {
			g.logger.Warn("MCP gateway: list tools failed", "server", name, "error", err)
			continue
		}
		for _, d := range defs {
			gt := gatewayTool{
				serverName: name,
				toolName:   d.Name,
				client:     client,
				toolDef:    d,
				tp:         g.tp,
			}
			if err := gt.register(registry); err != nil {
				g.logger.Warn("MCP gateway: register tool failed", "server", name, "tool", d.Name, "error", err)
				continue
			}
			registered++
		}
	}
	return registered, nil
}

// Disconnect closes all client connections.
func (g *Gateway) Disconnect() {
	g.mu.Lock()
	defer g.mu.Unlock()
	for name, client := range g.clients {
		if err := client.Close(); err != nil {
			g.logger.Warn("MCP gateway: close failed", "server", name, "error", err)
		}
	}
	g.clients = make(map[string]*Client)
}

// ConnectedServers returns names of successfully connected servers.
func (g *Gateway) ConnectedServers() []string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	names := make([]string, 0, len(g.clients))
	for name := range g.clients {
		names = append(names, name)
	}
	return names
}

// KnownConfigPaths returns list of known MCP config file locations to scan.
func KnownConfigPaths(homeDir string) []string {
	return []string{
		filepath.Join(homeDir, ".opencode", "mcp.json"),
		filepath.Join(homeDir, ".config", "opencode", "mcp.json"),
		filepath.Join(homeDir, ".claude", "mcp.json"),
		filepath.Join(homeDir, ".cursor", "mcp.json"),
	}
}

// parseMCPConfig reads a JSON file and returns server configs.
// Supports two formats:
//   - {"mcpServers": {"name": {"command": "..."}}}
//   - {"name": {"command": "..."}}
func parseMCPConfig(path string) ([]ServerConfig, error) {
	data, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	var cfgs []ServerConfig

	// Try mcpServers wrapper first
	if serversRaw, ok := raw["mcpServers"]; ok {
		var servers map[string]struct {
			Command string   `json:"command"`
			Args    []string `json:"args"`
		}
		if err := json.Unmarshal(serversRaw, &servers); err == nil {
			for name, s := range servers {
				if s.Command != "" {
					cfgs = append(cfgs, ServerConfig{
						Name:    name,
						Command: s.Command,
						Args:    s.Args,
					})
				}
			}
		}
		return cfgs, nil
	}

	// Flat format: {"name": {"command": "..."}}
	var flat map[string]struct {
		Command string   `json:"command"`
		Args    []string `json:"args"`
	}
	if err := json.Unmarshal(data, &flat); err != nil {
		return nil, err
	}
	for name, s := range flat {
		if s.Command != "" {
			cfgs = append(cfgs, ServerConfig{
				Name:    name,
				Command: s.Command,
				Args:    s.Args,
			})
		}
	}
	return cfgs, nil
}

// DiscoverServers scans known MCP config locations and returns server configs.
// Duplicate names (last wins) are resolved by file order.
func DiscoverServers(homeDir string) []ServerConfig {
	seen := make(map[string]bool)
	var all []ServerConfig

	for _, path := range KnownConfigPaths(homeDir) {
		cfgs, err := parseMCPConfig(path)
		if err != nil || len(cfgs) == 0 {
			continue
		}
		for _, c := range cfgs {
			if seen[c.Name] {
				continue
			}
			seen[c.Name] = true
			all = append(all, c)
		}
	}

	return all
}

// GatewayTool exposes a discovered external MCP tool for registration.
type GatewayTool struct {
	ServerName  string
	ToolName    string
	Client      *Client
	Description string
	InputSchema ToolSchema
}

// gatewayTool wraps an external MCP tool as a Dxrk Tool.
type gatewayTool struct {
	serverName string
	toolName   string
	client     *Client
	toolDef    ToolDefinition
	tp         trace.Exporter
}

func (gt *gatewayTool) toGatewayTool() GatewayTool {
	return GatewayTool{
		ServerName:  gt.serverName,
		ToolName:    gt.toolName,
		Client:      gt.client,
		Description: gt.toolDef.Description,
		InputSchema: gt.toolDef.InputSchema,
	}
}

func (gt *gatewayTool) register(registry *tools.Registry) error {
	fullName := fmt.Sprintf("mcp_%s_%s", gt.serverName, gt.toolName)

	schema := gt.toolDef.InputSchema
	props := make(map[string]any)
	for k, v := range schema.Properties {
		props[k] = map[string]any{
			"type":        v.Type,
			"description": v.Description,
		}
	}
	inputSchema := map[string]any{
		"type":       schema.Type,
		"properties": props,
	}

	t, err := tools.Build(tools.ToolDef{
		Name:        fullName,
		Description: gt.toolDef.Description,
		InputSchema: inputSchema,
		Execute: func(ctx tools.Context, input map[string]any) (any, error) {
			if gt.tp != nil {
				ctx.Context, _ = trace.StartSpan(ctx.Context, "mcp."+gt.serverName+"."+gt.toolName,
					trace.WithAttributes(
						attribute.String("mcp.server", gt.serverName),
						attribute.String("mcp.tool", gt.toolName),
					),
				)
			}
			resp, err := gt.client.CallTool(ctx, gt.toolName, input)
			if err != nil {
				return nil, fmt.Errorf("mcp %s/%s: %w", gt.serverName, gt.toolName, err)
			}
			return resp.Content, nil
		},
	})
	if err != nil {
		return err
	}

	return registry.Register(t)
}

// MarshalJSON for gatewayTool's tool definition (for display).
func (gt *gatewayTool) String() string {
	b, _ := json.Marshal(map[string]string{
		"server":      gt.serverName,
		"tool":        gt.toolName,
		"description": gt.toolDef.Description,
	})
	return string(b)
}
