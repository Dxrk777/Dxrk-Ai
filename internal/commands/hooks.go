// SPDX-License-Identifier: MIT
package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/Dxrk777/Dxrk-Ai/internal/strconst"
	"github.com/spf13/cobra"
)

const hooksFileName = "hooks.json"

// Hook represents a lifecycle hook configuration.
type Hook struct {
	Name     string   `json:"name"`
	Event    string   `json:"event"`
	Command  string   `json:"command"`
	Args     []string `json:"args,omitempty"`
	Enabled  bool     `json:"enabled"`
	Patterns []string `json:"patterns,omitempty"`
}

// HookConfig is the top-level hooks configuration.
type HookConfig struct {
	Hooks []Hook `json:"hooks"`
}

func hooksConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home: %w", err)
	}
	return filepath.Join(home, ".config", "dxrk", hooksFileName), nil
}

func loadHooksConfig() (*HookConfig, error) {
	path, err := hooksConfigPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		if os.IsNotExist(err) {
			return &HookConfig{}, nil
		}
		return nil, fmt.Errorf("read hooks config: %w", err)
	}

	var cfg HookConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse hooks config: %w", err)
	}
	return &cfg, nil
}

func saveHooksConfig(cfg *HookConfig) error {
	path, err := hooksConfigPath()
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	return os.WriteFile(path, append(data, '\n'), 0o600) //nolint:gosec
}

// RegisterHooksCommand adds the hooks subcommand to the given root command.
func RegisterHooksCommand(root *cobra.Command) {
	hooksCmd := &cobra.Command{
		Use:   "hooks",
		Short: "Manage lifecycle hooks",
		Long:  `List, add, remove, test, and toggle lifecycle hooks.`,
	}

	hooksCmd.AddCommand(
		newHooksListCmd(),
		newHooksAddCmd(),
		newHooksRemoveCmd(),
		newHooksTestCmd(),
		newHooksEnableCmd(),
		newHooksDisableCmd(),
	)

	root.AddCommand(hooksCmd)
}

func newHooksListCmd() *cobra.Command {
	var output string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List configured hooks",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadHooksConfig()
			if err != nil {
				return err
			}

			if len(cfg.Hooks) == 0 {
				fmt.Println("No hooks configured.")
				return nil
			}

			switch output {
			case "json":
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(cfg.Hooks)
			default:
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				_, _ = fmt.Fprintln(w, "NAME\tEVENT\tCOMMAND\tENABLED")
				for _, h := range cfg.Hooks {
					enabled := "no"
					if h.Enabled {
						enabled = "yes"
					}
					_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", h.Name, h.Event, h.Command, enabled)
				}
				return w.Flush()
			}
		},
	}

	cmd.Flags().StringVarP(&output, "output", "o", "text", "Output format (text, json)")
	return cmd
}

func newHooksAddCmd() *cobra.Command {
	var (
		event   string
		enabled bool
	)

	cmd := &cobra.Command{
		Use:   "add <name> <command> [args...]",
		Short: "Add a lifecycle hook",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			command := args[1]
			var hookArgs []string
			if len(args) > 2 {
				hookArgs = args[2:]
			}

			if event == "" {
				event = "pre-commit"
			}

			cfg, err := loadHooksConfig()
			if err != nil {
				return err
			}

			// Check for duplicate names
			for _, h := range cfg.Hooks {
				if h.Name == name {
					return fmt.Errorf("hook %q already exists", name)
				}
			}

			cfg.Hooks = append(cfg.Hooks, Hook{
				Name:    name,
				Event:   event,
				Command: command,
				Args:    hookArgs,
				Enabled: enabled,
			})

			if err := saveHooksConfig(cfg); err != nil {
				return err
			}

			fmt.Printf("Hook %q added.\n", name)
			return nil
		},
	}

	cmd.Flags().StringVar(&event, "event", "", "Hook event (pre-commit, post-commit, pre-push, etc.)")
	cmd.Flags().BoolVar(&enabled, strconst.StrEnabled, true, "Enable the hook immediately")

	return cmd
}

func newHooksRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a hook",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			cfg, err := loadHooksConfig()
			if err != nil {
				return err
			}

			found := false
			var filtered []Hook
			for _, h := range cfg.Hooks {
				if h.Name == name {
					found = true
					continue
				}
				filtered = append(filtered, h)
			}

			if !found {
				return fmt.Errorf("hook %q not found", name)
			}

			cfg.Hooks = filtered
			if err := saveHooksConfig(cfg); err != nil {
				return err
			}

			fmt.Printf("Hook %q removed.\n", name)
			return nil
		},
	}
}

func newHooksTestCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "test <name>",
		Short: "Test a hook by running it",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			cfg, err := loadHooksConfig()
			if err != nil {
				return err
			}

			var hook *Hook
			for i := range cfg.Hooks {
				if cfg.Hooks[i].Name == name {
					hook = &cfg.Hooks[i]
					break
				}
			}

			if hook == nil {
				return fmt.Errorf("hook %q not found", name)
			}

			fmt.Printf("Testing hook %q (event: %s, command: %s)...\n",
				hook.Name, hook.Event, hook.Command)
			fmt.Println("(Hook execution is simulated in this version)")
			return nil
		},
	}
}

func newHooksEnableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "enable <name>",
		Short: "Enable a hook",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return toggleHook(args[0], true)
		},
	}
}

func newHooksDisableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "disable <name>",
		Short: "Disable a hook",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return toggleHook(args[0], false)
		},
	}
}

func toggleHook(name string, enabled bool) error {
	cfg, err := loadHooksConfig()
	if err != nil {
		return err
	}

	found := false
	for i := range cfg.Hooks {
		if cfg.Hooks[i].Name == name {
			cfg.Hooks[i].Enabled = enabled
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("hook %q not found", name)
	}

	if err := saveHooksConfig(cfg); err != nil {
		return err
	}

	action := strconst.StrEnabled
	if !enabled {
		action = "disabled"
	}
	fmt.Printf("Hook %q %s.\n", name, action)
	return nil
}
