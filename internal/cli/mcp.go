// SPDX-License-Identifier: MIT
package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/Dxrk777/Dxrk/internal/agents"
	"github.com/Dxrk777/Dxrk/internal/components/checker"
	"github.com/Dxrk777/Dxrk/internal/components/permissions"
	"github.com/Dxrk777/Dxrk/internal/log"
	"github.com/Dxrk777/Dxrk/internal/mcp"
	"github.com/Dxrk777/Dxrk/internal/task"
	"github.com/Dxrk777/Dxrk/internal/tools"
	dxrktools "github.com/Dxrk777/Dxrk/internal/tools/dxrk"
	"github.com/Dxrk777/Dxrk/internal/tools/filetools"
	"github.com/Dxrk777/Dxrk/internal/trace"
)

const (
	mcpConfigFile = "mcp.json"
	subMCPServe   = "serve"
	goosWindows   = "windows"
)

// checkerAdapter adapts *permissions.Checker to tools.PermissionChecker.
type checkerAdapter struct{ c *permissions.Checker }

func (a checkerAdapter) Check(action, target string) (bool, string) {
	r := a.c.Check(permissions.Action(action), target)
	return r.Allowed, r.Reason
}

// auditAdapter adapts *permissions.Audit to tools.PermissionAudit.
type auditAdapter struct{ a *permissions.Audit }

func (ad auditAdapter) Record(_ string, target string, allowed bool, reason string) {
	ad.a.Record("", "", target, permissions.Result{Allowed: allowed, Reason: reason})
}

// RunMCPServe starts the MCP server over stdio.
func RunMCPServe(_ []string) error {
	logger := log.NewSlog(slog.Default())

	var tp trace.Exporter
	if os.Getenv("DXRK_TRACE") != "" {
		var err error
		tp, err = trace.NewTracerProvider("dxrk-mcp")
		if err != nil {
			logger.Warn("trace init failed", "error", err)
		} else {
			defer func() { _ = tp.Shutdown(context.Background()) }()
		}
	}

	toolReg := tools.New()
	agentReg, err := agents.NewDefaultRegistry()
	if err != nil {
		return fmt.Errorf("create agent registry: %w", err)
	}
	if err := dxrktools.RegisterAll(toolReg, agentReg); err != nil {
		return fmt.Errorf("register tools: %w", err)
	}
	if err := filetools.RegisterAll(toolReg); err != nil {
		return fmt.Errorf("register filetools: %w", err)
	}

	// Connect MCP gateway: discover and register tools from external MCP servers
	gateway := newGatewayFromConfig(logger, tp)
	if gateway != nil {
		gwTools, gwErr := gateway.Connect(context.Background())
		if gwErr != nil {
			logger.Warn("MCP gateway: partial connection", "error", gwErr)
		}
		for _, gt := range gwTools {
			if regErr := registerGatewayTool(toolReg, gt); regErr != nil {
				logger.Warn("MCP gateway: register tool", "server", gt.ServerName, "tool", gt.ToolName, "error", regErr)
			}
		}
		if len(gwTools) > 0 {
			logger.Info("MCP gateway: registered tools", "count", len(gwTools), "servers", gateway.ConnectedServers())
		}
	}

	// Start worker pool for async task execution
	queue := task.NewQueue()
	pool := task.NewWorkerPool(queue, func(_ context.Context, t *task.Task) (any, error) {
		return t.Payload.Data, nil
	}, task.WithLogger(logger), task.WithWorkerCount(2))
	pool.Start()

	// Load and wire runtime permission checker
	homeDir, _ := os.UserHomeDir()
	checkRules, denyDefault, _ := checker.LoadRules(homeDir)
	permRules := make([]permissions.Rule, len(checkRules))
	for i, r := range checkRules {
		permRules[i] = permissions.Rule{
			Action: permissions.Action(r.Action),
			Target: r.Target,
			Allow:  r.Allow,
		}
	}
	permChecker := permissions.NewChecker(
		permissions.WithDenyByDefault(denyDefault),
		permissions.WithRules(permRules...),
	)
	permAudit := permissions.NewAudit(1000)

	metrics := mcp.NewPrometheusMetrics()

	serverOpts := []mcp.ServerOption{
		mcp.ServerWithLogger(logger),
		mcp.ServerWithPermissionChecker(
			checkerAdapter{c: permChecker},
			auditAdapter{a: permAudit},
		),
		mcp.ServerWithRateLimiter(10, 5),
		mcp.ServerWithWorkerPool(pool),
		mcp.ServerWithWatchdog(10 * time.Second),
		mcp.ServerWithMetrics(metrics),
	}
	if tp != nil {
		serverOpts = append(serverOpts, mcp.ServerWithTracer(tp))
	}
	server := mcp.NewServer(toolReg, os.Stdin, os.Stdout, serverOpts...)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)

	// Run with watchdog retry; on stall or crash, restart once
	for attempts := 0; attempts < 2; attempts++ {
		serveErr := server.Serve(ctx)

		// Check if watchdog detected stall
		select {
		case <-server.WatchdogCh():
			logger.Warn("MCP watchdog: server stalled, restarting")
			continue
		default:
		}

		if serveErr != nil {
			logger.Warn("MCP server exited", "error", serveErr, "attempt", attempts)
		}
		break
	}

	pool.Stop()
	if gateway != nil {
		gateway.Disconnect()
	}
	stop()
	return nil
}

// newGatewayFromConfig discovers MCP servers from config directories.
func newGatewayFromConfig(logger log.Logger, tp trace.Exporter) *mcp.Gateway {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	var servers []mcp.ServerConfig

	candidates := []struct {
		dir  string
		file string
	}{
		{filepath.Join(home, ".opencode"), mcpConfigFile},
		{filepath.Join(home, ".config", "opencode"), mcpConfigFile},
		{filepath.Join(home, ".claude"), mcpConfigFile},
	}

	for _, c := range candidates {
		path := filepath.Join(c.dir, c.file)
		f, err := os.Open(path) //nolint:gosec
		if err != nil {
			continue
		}
		var cfg struct {
			Servers map[string]struct {
				Command string   `json:"command"`
				Args    []string `json:"args,omitempty"`
			} `json:"mcpServers"`
		}
		err = json.NewDecoder(f).Decode(&cfg)
		_ = f.Close()
		if err != nil {
			continue
		}
		for name, s := range cfg.Servers {
			if s.Command == "" {
				continue
			}
			servers = append(servers, mcp.ServerConfig{
				Name:    name,
				Command: s.Command,
				Args:    s.Args,
			})
		}
		break // first config found wins
	}

	if len(servers) == 0 {
		return nil
	}
	return mcp.NewGateway(servers, logger, tp)
}

// registerGatewayTool registers a single external MCP tool as a Dxrk tool.
func registerGatewayTool(reg *tools.Registry, gt mcp.GatewayTool) error {
	fullName := fmt.Sprintf("mcp_%s_%s", gt.ServerName, gt.ToolName)

	props := make(map[string]any)
	for k, v := range gt.InputSchema.Properties {
		props[k] = map[string]any{
			"type":        v.Type,
			"description": v.Description,
		}
	}
	inputSchema := map[string]any{
		"type":       "object",
		"properties": props,
	}

	t, err := tools.Build(tools.ToolDef{
		Name:        fullName,
		Description: gt.Description,
		InputSchema: inputSchema,
		Execute: func(ctx tools.Context, input map[string]any) (any, error) {
			resp, err := gt.Client.CallTool(ctx, gt.ToolName, input)
			if err != nil {
				return nil, fmt.Errorf("mcp %s/%s: %w", gt.ServerName, gt.ToolName, err)
			}
			return resp.Content, nil
		},
		IsReadOnly: tools.DefaultEnabled(),
	})
	if err != nil {
		return err
	}
	return reg.Register(t)
}

// MCPServerConfig represents a single MCP server entry for opencode.json.
type MCPServerConfig struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

// OpenCodeMCPConfig represents the full opencode.json MCP servers section.
type OpenCodeMCPConfig struct {
	MCPServers map[string]MCPServerConfig `json:"mcpServers"`
}

// RunMCPGenerateConfig writes .opencode/mcp.json with Go backend servers.
func RunMCPGenerateConfig(args []string) error {
	fs := flag.NewFlagSet("mcp generate-config", flag.ContinueOnError)
	fs.SetOutput(ioDiscard{})
	projectDir := fs.String("project-dir", ".", "Project directory")
	out := fs.String("out", "", "Output path (default: .opencode/mcp.json)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	dxrkBin, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve dxrk binary: %w", err)
	}

	config := OpenCodeMCPConfig{
		MCPServers: map[string]MCPServerConfig{
			"dxrk-mcp": {
				Command: dxrkBin,
				Args:    []string{"mcp", subMCPServe},
			},
			"dxrk-memory": {
				Command: "dxrk-memory",
				Args:    []string{"mcp", "--tools=agent"},
			},
		},
	}

	outPath := *out
	if outPath == "" {
		mcpDir := filepath.Join(*projectDir, ".opencode")
		if err := os.MkdirAll(mcpDir, 0o750); err != nil {
			return fmt.Errorf("create %q: %w", mcpDir, err)
		}
		outPath = filepath.Join(mcpDir, "mcp.json")
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	data = append(data, '\n')

	if err := os.WriteFile(outPath, data, 0o600); err != nil {
		return fmt.Errorf("write %q: %w", outPath, err)
	}

	fmt.Printf("Wrote MCP config to %s (Go backend)\n", outPath)
	return nil
}

// RunMCPDiscover scans config directories for external MCP servers
// and prints their discovered tools.
func RunMCPDiscover(_ []string) error {
	logger := log.NewSlog(slog.Default())
	gateway := newGatewayFromConfig(logger, nil)
	if gateway == nil {
		fmt.Println("No MCP server config found in ~/.opencode, ~/.config/opencode, or ~/.claude")
		return nil
	}

	mcpTools, err := gateway.Connect(context.Background())
	if err != nil {
		fmt.Printf("MCP gateway: %v\n", err)
	}
	defer gateway.Disconnect()

	if len(mcpTools) == 0 {
		fmt.Println("No tools discovered from configured MCP servers.")
		return nil
	}

	fmt.Printf("Discovered %d tools from %d servers:\n\n", len(mcpTools), len(gateway.ConnectedServers()))
	byServer := make(map[string][]mcp.GatewayTool)
	for _, t := range mcpTools {
		byServer[t.ServerName] = append(byServer[t.ServerName], t)
	}
	for _, name := range gateway.ConnectedServers() {
		fmt.Printf("  %s (%d tools):\n", name, len(byServer[name]))
		for _, t := range byServer[name] {
			desc := t.Description
			if len(desc) > 60 {
				desc = desc[:60] + "..."
			}
			fmt.Printf("    - mcp_%s_%s: %s\n", name, t.ToolName, desc)
		}
	}
	return nil
}

// ParseMCPFlags parses flags for the mcp subcommand.
func ParseMCPFlags(args []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("expected mcp subcommand: serve, generate-config, discover")
	}
	sub := args[0]
	rest := args[1:]
	switch sub {
	case subMCPServe:
		fs := flag.NewFlagSet("mcp serve", flag.ContinueOnError)
		fs.SetOutput(ioDiscard{})
		if err := fs.Parse(rest); err != nil {
			return "", err
		}
		return subMCPServe, nil
	case "generate-config":
		if err := RunMCPGenerateConfig(rest); err != nil {
			return "", err
		}
		return "generate-config", nil
	case "discover":
		if err := RunMCPDiscover(rest); err != nil {
			return "", err
		}
		return "discover", nil
	default:
		return "", fmt.Errorf("unknown mcp subcommand %q (expected: serve, generate-config, discover)", sub)
	}
}

// RunCheckPermission checks file permissions for an agent.
func RunCheckPermission(args []string) error {
	fs := flag.NewFlagSet("check-permission", flag.ContinueOnError)
	fs.SetOutput(ioDiscard{})
	agent := fs.String("agent", "opencode", "Agent")
	path := fs.String("path", "", "Path to check")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *path == "" {
		return fmt.Errorf("--path is required")
	}
	fmt.Printf("Permission check for %s on %s:\n", *agent, *path)
	fmt.Println("  (runtime permission checking coming in next release)")
	return nil
}
