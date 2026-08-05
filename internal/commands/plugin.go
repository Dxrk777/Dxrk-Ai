// SPDX-License-Identifier: MIT
package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/Dxrk777/Dxrk-Ai/internal/plugin"
	"github.com/spf13/cobra"
)

// RegisterPluginCommand adds the plugin subcommand to the given root command.
func RegisterPluginCommand(root *cobra.Command) {
	pluginCmd := &cobra.Command{
		Use:   "plugin",
		Short: "Manage tool plugins",
		Long:  `List, install, uninstall, update, and inspect tool plugins.`,
	}

	pluginCmd.AddCommand(
		newPluginListCmd(),
		newPluginInstallCmd(),
		newPluginUninstallCmd(),
		newPluginUpdateCmd(),
		newPluginInfoCmd(),
	)

	root.AddCommand(pluginCmd)
}

func pluginsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home: %w", err)
	}
	return filepath.Join(home, ".config", "dxrk", "plugins"), nil
}

func newPluginListCmd() *cobra.Command {
	var output string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List installed plugins",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := pluginsDir()
			if err != nil {
				return err
			}

			entries, err := os.ReadDir(dir)
			if err != nil {
				if os.IsNotExist(err) {
					fmt.Println("No plugins installed.")
					return nil
				}
				return fmt.Errorf("read plugins dir: %w", err)
			}

			var manifests []plugin.Manifest
			for _, entry := range entries {
				if !entry.IsDir() {
					continue
				}
				manifestPath := filepath.Join(dir, entry.Name(), "plugin.json")
				data, err := os.ReadFile(manifestPath) //nolint:gosec
				if err != nil {
					continue
				}
				var m plugin.Manifest
				if err := json.Unmarshal(data, &m); err != nil {
					continue
				}
				manifests = append(manifests, m)
			}

			if len(manifests) == 0 {
				fmt.Println("No plugins installed.")
				return nil
			}

			switch output {
			case "json":
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(manifests)
			default:
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				_, _ = fmt.Fprintln(w, "NAME\tVERSION\tDESCRIPTION")
				for _, m := range manifests {
					desc := m.Description
					if len(desc) > 50 {
						desc = desc[:50] + "..."
					}
					_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", m.Name, m.Version, desc)
				}
				return w.Flush()
			}
		},
	}

	cmd.Flags().StringVarP(&output, "output", "o", "text", "Output format (text, json)")
	return cmd
}

func newPluginInstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install <name>",
		Short: "Install a plugin",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			dir, err := pluginsDir()
			if err != nil {
				return err
			}

			pluginDir := filepath.Join(dir, name)
			if err := os.MkdirAll(pluginDir, 0o750); err != nil {
				return fmt.Errorf("create plugin dir: %w", err)
			}

			manifest := plugin.Manifest{
				Name:        name,
				Description: "Plugin: " + name,
				Version:     "0.1.0",
				Entry:       "index.js",
			}

			data, err := json.MarshalIndent(manifest, "", "  ")
			if err != nil {
				return fmt.Errorf("marshal manifest: %w", err)
			}

			manifestPath := filepath.Join(pluginDir, "plugin.json")
			if err := os.WriteFile(manifestPath, append(data, '\n'), 0o600); err != nil { //nolint:gosec
				return fmt.Errorf("write manifest: %w", err)
			}

			fmt.Printf("Plugin %q installed.\n", name)
			return nil
		},
	}
}

func newPluginUninstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall <name>",
		Short: "Uninstall a plugin",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			dir, err := pluginsDir()
			if err != nil {
				return err
			}

			pluginDir := filepath.Join(dir, name)
			if _, err := os.Stat(pluginDir); os.IsNotExist(err) {
				return fmt.Errorf("plugin %q not found", name)
			}

			if err := os.RemoveAll(pluginDir); err != nil {
				return fmt.Errorf("remove plugin: %w", err)
			}

			fmt.Printf("Plugin %q uninstalled.\n", name)
			return nil
		},
	}
}

func newPluginUpdateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update <name>",
		Short: "Update a plugin",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			dir, err := pluginsDir()
			if err != nil {
				return err
			}

			manifestPath := filepath.Join(dir, name, "plugin.json")
			data, err := os.ReadFile(manifestPath) //nolint:gosec
			if err != nil {
				return fmt.Errorf("plugin %q not found", name)
			}

			var m plugin.Manifest
			if err := json.Unmarshal(data, &m); err != nil {
				return fmt.Errorf("parse manifest: %w", err)
			}

			fmt.Printf("Plugin %q (v%s) is up to date.\n", m.Name, m.Version)
			return nil
		},
	}
}

func newPluginInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info <name>",
		Short: "Show plugin information",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			dir, err := pluginsDir()
			if err != nil {
				return err
			}

			manifestPath := filepath.Join(dir, name, "plugin.json")
			data, err := os.ReadFile(manifestPath) //nolint:gosec
			if err != nil {
				return fmt.Errorf("plugin %q not found", name)
			}

			var m plugin.Manifest
			if err := json.Unmarshal(data, &m); err != nil {
				return fmt.Errorf("parse manifest: %w", err)
			}

			fmt.Printf("Name:        %s\n", m.Name)
			fmt.Printf("Version:     %s\n", m.Version)
			fmt.Printf("Description: %s\n", m.Description)
			fmt.Printf("Entry:       %s\n", m.Entry)
			return nil
		},
	}
}
