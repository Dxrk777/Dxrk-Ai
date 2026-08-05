// SPDX-License-Identifier: MIT
package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

const defaultConfigVersion = "1"

// DxrkConfig represents the project-level Dxrk configuration.
type DxrkConfig struct {
	Version string            `json:"version"`
	Project string            `json:"project,omitempty"`
	Agents  []string          `json:"agents,omitempty"`
	Options map[string]string `json:"options,omitempty"`
}

func RegisterInitCommand(reg *Registry) {
	reg.AddCommand(&cobra.Command{
		Use:   "init",
		Short: "Initialize Dxrk-Ai in the current project",
		Long:  "Create .dxrk/ directory and default configuration for Dxrk-Ai.",
		RunE: func(cmd *cobra.Command, args []string) error {
			projectName := ""
			if len(args) > 0 {
				projectName = args[0]
			}
			return runInit(projectName)
		},
	})
}

func runInit(projectName string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve working directory: %w", err)
	}

	dxrkdir := filepath.Join(cwd, ".dxrk")
	if err := os.MkdirAll(dxrkdir, 0o750); err != nil {
		return fmt.Errorf("create .dxrk directory: %w", err)
	}

	configPath := filepath.Join(dxrkdir, "config.json")
	if _, err := os.Stat(configPath); err == nil {
		fmt.Fprintf(os.Stderr, "Dxrk-Ai already initialized in this directory.\n")
		fmt.Fprintf(os.Stderr, "Config: %s\n", configPath)
		return nil
	}

	if projectName == "" {
		projectName = filepath.Base(cwd)
	}

	config := DxrkConfig{
		Version: defaultConfigVersion,
		Project: projectName,
		Agents:  []string{},
		Options: map[string]string{},
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	memoryDir := filepath.Join(dxrkdir, "memory")
	if err := os.MkdirAll(memoryDir, 0o750); err != nil {
		return fmt.Errorf("create memory directory: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Initialized Dxrk-Ai in %s\n", dxrkdir)
	fmt.Fprintf(os.Stderr, "  Config:    %s\n", configPath)
	fmt.Fprintf(os.Stderr, "  Memory:    %s\n", memoryDir)
	fmt.Fprintf(os.Stderr, "  Project:   %s\n", projectName)
	return nil
}
