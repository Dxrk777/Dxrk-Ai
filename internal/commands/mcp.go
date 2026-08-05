// SPDX-License-Identifier: MIT
package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/Dxrk777/Dxrk-Ai/internal/log"
	"github.com/Dxrk777/Dxrk-Ai/internal/mcp"
	"github.com/Dxrk777/Dxrk-Ai/internal/strconst"
	"github.com/spf13/cobra"
)

// RegisterMCPCommand adds the mcp subcommand to the given root command.
func RegisterMCPCommand(root *cobra.Command) {
	mcpCmd := &cobra.Command{
		Use:   "mcp",
		Short: "Manage MCP servers",
		Long:  `List, add, remove, and test MCP server connections.`,
	}

	mcpCmd.AddCommand(
		newMCPListCmd(),
		newMCPAddCmd(),
		newMCPRemoveCmd(),
		newMCPStatusCmd(),
		newMCPTestCmd(),
	)

	root.AddCommand(mcpCmd)
}

func loadMCPConfig() (map[string]struct {
	Command string   `json:"command"`
	Args    []string `json:"args,omitempty"`
}, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve home: %w", err)
	}

	candidates := []string{
		filepath.Join(home, ".opencode", "mcp.json"),
		filepath.Join(home, ".config", "opencode", "mcp.json"),
		filepath.Join(home, ".claude", "mcp.json"),
	}

	for _, path := range candidates {
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
		return cfg.Servers, nil
	}
	return nil, fmt.Errorf("no MCP config found in ~/.opencode, ~/.config/opencode, or ~/.claude")
}

func newMCPListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured MCP servers",
		RunE: func(cmd *cobra.Command, args []string) error {
			servers, err := loadMCPConfig()
			if err != nil {
				fmt.Println("No MCP servers configured.")
				//nolint:nilerr // friendly UX: empty config is not a command failure.
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			_, _ = fmt.Fprintln(w, "NAME\tCOMMAND\tARGS")
			for name, s := range servers {
				_, _ = fmt.Fprintf(w, "%s\t%s\t%v\n", name, s.Command, s.Args)
			}
			return w.Flush()
		},
	}
}

func newMCPAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <name> <command> [args...]",
		Short: "Add an MCP server",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			command := args[1]
			var cmdArgs []string
			if len(args) > 2 {
				cmdArgs = args[2:]
			}

			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("resolve home: %w", err)
			}

			configDir := filepath.Join(home, ".config", "opencode")
			if err := os.MkdirAll(configDir, 0o750); err != nil {
				return fmt.Errorf("create config dir: %w", err)
			}

			configPath := filepath.Join(configDir, "mcp.json")
			cfg := struct {
				Servers map[string]struct {
					Command string   `json:"command"`
					Args    []string `json:"args,omitempty"`
				} `json:"mcpServers"`
			}{}

			data, err := os.ReadFile(configPath) //nolint:gosec
			if err == nil {
				_ = json.Unmarshal(data, &cfg)
			}

			if cfg.Servers == nil {
				cfg.Servers = make(map[string]struct {
					Command string   `json:"command"`
					Args    []string `json:"args,omitempty"`
				})
			}

			cfg.Servers[name] = struct {
				Command string   `json:"command"`
				Args    []string `json:"args,omitempty"`
			}{
				Command: command,
				Args:    cmdArgs,
			}

			out, err := json.MarshalIndent(cfg, "", "  ")
			if err != nil {
				return fmt.Errorf("marshal config: %w", err)
			}

			if err := os.WriteFile(configPath, append(out, '\n'), 0o600); err != nil {
				return fmt.Errorf("write config: %w", err)
			}

			fmt.Printf("MCP server %q added.\n", name)
			return nil
		},
	}
}

func newMCPRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove an MCP server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("resolve home: %w", err)
			}

			configPath := filepath.Join(home, ".config", "opencode", "mcp.json")
			data, err := os.ReadFile(configPath) //nolint:gosec
			if err != nil {
				return fmt.Errorf("MCP config not found: %w", err)
			}

			cfg := struct {
				Servers map[string]struct {
					Command string   `json:"command"`
					Args    []string `json:"args,omitempty"`
				} `json:"mcpServers"`
			}{}
			if err := json.Unmarshal(data, &cfg); err != nil {
				return fmt.Errorf("parse config: %w", err)
			}

			if _, exists := cfg.Servers[name]; !exists {
				return fmt.Errorf("MCP server %q not found", name)
			}

			delete(cfg.Servers, name)

			out, err := json.MarshalIndent(cfg, "", "  ")
			if err != nil {
				return fmt.Errorf("marshal config: %w", err)
			}

			if err := os.WriteFile(configPath, append(out, '\n'), 0o600); err != nil {
				return fmt.Errorf("write config: %w", err)
			}

			fmt.Printf("MCP server %q removed.\n", name)
			return nil
		},
	}
}

func newMCPStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   strconst.StrStatus,
		Short: "Show MCP server connection status",
		RunE: func(cmd *cobra.Command, args []string) error {
			servers, err := loadMCPConfig()
			if err != nil {
				fmt.Println("No MCP servers configured.")
				return nil
			}

			logger := log.NewSlog(slog.Default())
			gateway := mcp.NewGateway(nil, logger, nil)

			var configs []mcp.ServerConfig
			for name, s := range servers {
				configs = append(configs, mcp.ServerConfig{
					Name:    name,
					Command: s.Command,
					Args:    s.Args,
				})
			}

			gw := mcp.NewGateway(configs, logger, nil)
			tools, err := gw.Connect(context.Background())
			if err != nil {
				fmt.Printf("Gateway connection error: %v\n", err)
			}
			defer gw.Disconnect()

			_ = gateway

			fmt.Printf("Connected to %d MCP servers with %d tools discovered.\n",
				len(gw.ConnectedServers()), len(tools))

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			_, _ = fmt.Fprintln(w, "SERVER\tTOOLS\tSTATUS")
			for _, name := range gw.ConnectedServers() {
				count := 0
				for _, t := range tools {
					if t.ServerName == name {
						count++
					}
				}
				_, _ = fmt.Fprintf(w, "%s\t%d\tconnected\n", name, count)
			}
			return w.Flush()
		},
	}
}

func newMCPTestCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "test <server-name>",
		Short: "Test connection to an MCP server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			serverName := args[0]

			servers, err := loadMCPConfig()
			if err != nil {
				return fmt.Errorf("load MCP config: %w", err)
			}

			s, exists := servers[serverName]
			if !exists {
				return fmt.Errorf("MCP server %q not found", serverName)
			}

			logger := log.NewSlog(slog.Default())
			gateway := mcp.NewGateway([]mcp.ServerConfig{{
				Name:    serverName,
				Command: s.Command,
				Args:    s.Args,
			}}, logger, nil)

			tools, err := gateway.Connect(context.Background())
			gateway.Disconnect()

			if err != nil {
				fmt.Printf("Server %q: connection failed: %v\n", serverName, err)
				return nil
			}

			fmt.Printf("Server %q: connected successfully (%d tools)\n", serverName, len(tools))
			return nil
		},
	}
}
