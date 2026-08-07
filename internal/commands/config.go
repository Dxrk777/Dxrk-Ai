// SPDX-License-Identifier: MIT
package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Dxrk777/Dxrk/internal/config"
	"github.com/Dxrk777/Dxrk/internal/strconst"
)

// RegisterConfigCommand registers the `dxrk config` command with subcommands`dxrk config` command with subcommands
// for viewing and editing configuration.
func RegisterConfigCommand(reg *Registry) {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "View and manage configuration",
		Long:  "View, edit, and manage Dxrk configuration across all scopes.",
	}

	cmd.AddCommand(
		configGetCmd(),
		configSetCmd(),
		configListCmd(),
		configPathCmd(),
	)

	reg.AddCommand(cmd)
}

func defaultConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".dxrk", "config.yaml")
}

func configGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <key>",
		Short: "Get a configuration value",
		Long: `Get a configuration value by dot-separated key path.

Examples:
  dxrk config get project.name
  dxrk config get providers.0.model
  dxrk config get git.auto_commit`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]
			cfg, err := config.Load(defaultConfigPath())
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			val := getConfigValue(cfg, key)
			if val == "" {
				return fmt.Errorf("key %q not found or empty", key)
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), val)
			return nil
		},
	}
}

func configSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a configuration value",
		Long: `Set a configuration value by dot-separated key path.

Examples:
  dxrk config set project.name my-app
  dxrk config set git.auto_commit true`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			key, value := args[0], args[1]
			cfg, err := config.Load(defaultConfigPath())
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			if err := setConfigValue(cfg, key, value); err != nil {
				return err
			}

			if err := config.Save(defaultConfigPath(), cfg); err != nil {
				return fmt.Errorf("save config: %w", err)
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Set %s = %s\n", key, value)
			return nil
		},
	}
}

func configListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Short:   "List all configuration values",
		Aliases: []string{"ls"},
		Long:    "List all configuration values from the current config file.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(defaultConfigPath())
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			out := cmd.OutOrStdout()
			_, _ = fmt.Fprintf(out, "Project Name:        %s\n", cfg.Project.Name)
			_, _ = fmt.Fprintf(out, "Default Provider:    %s\n", cfg.Project.DefaultProvider)
			_, _ = fmt.Fprintln(out)

			_, _ = fmt.Fprintln(out, "Providers:")
			for _, p := range cfg.Providers {
				_, _ = fmt.Fprintf(out, "  %s: model=%s base_url=%s\n", p.Name, p.Model, p.BaseURL)
			}

			if cfg.Git != nil {
				_, _ = fmt.Fprintln(out)
				_, _ = fmt.Fprintf(out, "Git AutoCommit:      %t\n", cfg.Git.AutoCommit)
				_, _ = fmt.Fprintf(out, "Git AutoPush:        %t\n", cfg.Git.AutoPush)
				_, _ = fmt.Fprintf(out, "Git RequirePR:       %t\n", cfg.Git.RequirePR)
			}

			if cfg.Sandbox != nil {
				_, _ = fmt.Fprintln(out)
				_, _ = fmt.Fprintf(out, "Sandbox Image:       %s\n", cfg.Sandbox.DefaultImage)
				_, _ = fmt.Fprintf(out, "Sandbox Memory:      %s\n", cfg.Sandbox.MemoryLimit)
			}

			if cfg.WebUI != nil {
				_, _ = fmt.Fprintln(out)
				_, _ = fmt.Fprintf(out, "WebUI Enabled:       %t\n", cfg.WebUI.Enabled)
				_, _ = fmt.Fprintf(out, "WebUI Port:          %d\n", cfg.WebUI.Port)
				_, _ = fmt.Fprintf(out, "WebUI Theme:         %s\n", cfg.WebUI.Theme)
			}

			if cfg.Autonomy != nil {
				_, _ = fmt.Fprintln(out)
				_, _ = fmt.Fprintf(out, "Autonomy Enabled:    %t\n", cfg.Autonomy.Enabled)
				_, _ = fmt.Fprintf(out, "Autonomy Interval:   %ds\n", cfg.Autonomy.IntervalSec)
			}

			return nil
		},
	}
}

func configPathCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "Show configuration file paths",
		Long:  "Display the paths to all configuration files.",
		RunE: func(cmd *cobra.Command, args []string) error {
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("resolve home directory: %w", err)
			}

			out := cmd.OutOrStdout()
			_, _ = fmt.Fprintf(out, "User config:    %s\n", filepath.Join(home, ".dxrk", "config.yaml"))
			_, _ = fmt.Fprintf(out, "User settings:  %s\n", filepath.Join(home, ".dxrk", "settings.json"))

			wd, _ := os.Getwd()
			_, _ = fmt.Fprintf(out, "Project config: %s\n", filepath.Join(wd, ".dxrk", "config.yaml"))
			return nil
		},
	}
}

func getConfigValue(cfg *config.Config, key string) string {
	parts := strings.Split(key, ".")
	if len(parts) == 0 {
		return ""
	}

	switch parts[0] {
	case strconst.StrProject:
		if len(parts) < 2 {
			return ""
		}
		switch parts[1] {
		case "name":
			return cfg.Project.Name
		case "root":
			return cfg.Project.Root
		case "default_provider":
			return cfg.Project.DefaultProvider
		}
	case "providers":
		if len(parts) < 3 {
			return ""
		}
		var idx int
		if _, err := fmt.Sscanf(parts[1], "%d", &idx); err != nil {
			return ""
		}
		if idx < 0 || idx >= len(cfg.Providers) {
			return ""
		}
		p := cfg.Providers[idx]
		switch parts[2] {
		case "name":
			return p.Name
		case "model":
			return p.Model
		case "base_url":
			return p.BaseURL
		case "api_key_env":
			return p.APIKeyEnv
		}
	case "git":
		if cfg.Git == nil || len(parts) < 2 {
			return ""
		}
		switch parts[1] {
		case "auto_commit":
			return fmt.Sprintf("%t", cfg.Git.AutoCommit)
		case "auto_push":
			return fmt.Sprintf("%t", cfg.Git.AutoPush)
		case "require_pr":
			return fmt.Sprintf("%t", cfg.Git.RequirePR)
		}
	case "sandbox":
		if cfg.Sandbox == nil || len(parts) < 2 {
			return ""
		}
		switch parts[1] {
		case "default_image":
			return cfg.Sandbox.DefaultImage
		case "memory_limit":
			return cfg.Sandbox.MemoryLimit
		case "cpu_limit":
			return cfg.Sandbox.CPULimit
		case "timeout_sec":
			return fmt.Sprintf("%d", cfg.Sandbox.TimeoutSec)
		}
	case "webui":
		if cfg.WebUI == nil || len(parts) < 2 {
			return ""
		}
		switch parts[1] {
		case strconst.StrEnabled:
			return fmt.Sprintf("%t", cfg.WebUI.Enabled)
		case "port":
			return fmt.Sprintf("%d", cfg.WebUI.Port)
		case "theme":
			return cfg.WebUI.Theme
		case "host":
			return cfg.WebUI.Host
		}
	case "autonomy":
		if cfg.Autonomy == nil || len(parts) < 2 {
			return ""
		}
		switch parts[1] {
		case strconst.StrEnabled:
			return fmt.Sprintf("%t", cfg.Autonomy.Enabled)
		case "interval_sec":
			return fmt.Sprintf("%d", cfg.Autonomy.IntervalSec)
		}
	}
	return ""
}

func setConfigValue(cfg *config.Config, key, value string) error {
	parts := strings.Split(key, ".")
	if len(parts) < 2 {
		return fmt.Errorf("key must be in format <group>.<field> (e.g. project.name)")
	}

	switch parts[0] {
	case strconst.StrProject:
		switch parts[1] {
		case "name":
			cfg.Project.Name = value
		case "root":
			cfg.Project.Root = value
		case "default_provider":
			cfg.Project.DefaultProvider = value
		default:
			return fmt.Errorf("unknown project field: %s", parts[1])
		}
	case "git":
		if cfg.Git == nil {
			cfg.Git = &config.GitConfig{}
		}
		switch parts[1] {
		case "auto_commit":
			cfg.Git.AutoCommit = value == "true"
		case "auto_push":
			cfg.Git.AutoPush = value == "true"
		case "require_pr":
			cfg.Git.RequirePR = value == "true"
		default:
			return fmt.Errorf("unknown git field: %s", parts[1])
		}
	case "sandbox":
		if cfg.Sandbox == nil {
			cfg.Sandbox = &config.SandboxConfig{}
		}
		switch parts[1] {
		case "default_image":
			cfg.Sandbox.DefaultImage = value
		case "memory_limit":
			cfg.Sandbox.MemoryLimit = value
		case "cpu_limit":
			cfg.Sandbox.CPULimit = value
		case "timeout_sec":
			var v int
			if _, err := fmt.Sscanf(value, "%d", &v); err != nil {
				return fmt.Errorf("invalid integer: %w", err)
			}
			cfg.Sandbox.TimeoutSec = v
		default:
			return fmt.Errorf("unknown sandbox field: %s", parts[1])
		}
	case "webui":
		if cfg.WebUI == nil {
			cfg.WebUI = &config.WebUIConfig{}
		}
		switch parts[1] {
		case strconst.StrEnabled:
			cfg.WebUI.Enabled = value == "true"
		case "port":
			var v int
			if _, err := fmt.Sscanf(value, "%d", &v); err != nil {
				return fmt.Errorf("invalid integer: %w", err)
			}
			cfg.WebUI.Port = v
		case "theme":
			cfg.WebUI.Theme = value
		case "host":
			cfg.WebUI.Host = value
		default:
			return fmt.Errorf("unknown webui field: %s", parts[1])
		}
	default:
		return fmt.Errorf("unknown config group: %s", parts[0])
	}
	return nil
}
