// SPDX-License-Identifier: MIT
package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Dxrk777/Dxrk-Ai/internal/strconst"
	"github.com/spf13/cobra"
)

func RegisterDoctorCommand(reg *Registry) {
	reg.AddCommand(&cobra.Command{
		Use:   "doctor",
		Short: "Run system diagnostics",
		Long:  "Check Go, git, dependencies, permissions, and config for Dxrk-Ai.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDoctor()
		},
	})
}

func runDoctor() error {
	checks := []doctorCheck{
		{name: "go", fn: checkGo},
		{name: "git", fn: checkGit},
		{name: "home-dir", fn: checkHomeDir},
		{name: "dxrk-config", fn: checkDxrkConfig},
		{name: "env-api-key", fn: checkAPIKey},
		{name: "permissions", fn: checkPermissions},
	}

	passed := 0
	failed := 0
	for _, c := range checks {
		if err := c.fn(); err != nil {
			fmt.Fprintf(os.Stderr, "  FAIL  %s: %v\n", c.name, err)
			failed++
		} else {
			fmt.Fprintf(os.Stderr, "  OK    %s\n", c.name)
			passed++
		}
	}

	fmt.Fprintf(os.Stderr, "\n%d passed, %d failed\n", passed, failed)
	if failed > 0 {
		return fmt.Errorf("%d check(s) failed", failed)
	}
	return nil
}

type doctorCheck struct {
	name string
	fn   func() error
}

func checkGo() error {
	path, err := exec.LookPath("go")
	if err != nil {
		return fmt.Errorf("go not found in PATH")
	}
	out, err := exec.Command(path, strconst.StrVersion).Output()
	if err != nil {
		return fmt.Errorf("go version: %w", err)
	}
	_ = strings.TrimSpace(string(out))
	return nil
}

func checkGit() error {
	_, err := exec.LookPath("git")
	if err != nil {
		return fmt.Errorf("git not found in PATH")
	}
	return nil
}

func checkHomeDir() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot resolve home directory: %w", err)
	}
	if _, err := os.Stat(home); err != nil {
		return fmt.Errorf("home directory %q not accessible: %w", home, err)
	}
	return nil
}

func checkDxrkConfig() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	configDir := filepath.Join(home, ".dxrk")
	info, err := os.Stat(configDir)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("~/.dxrk not found — run 'dxrk init' first")
		}
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("~/.dxrk exists but is not a directory")
	}
	return nil
}

func checkAPIKey() error {
	if os.Getenv("ANTHROPIC_API_KEY") != "" {
		return nil
	}
	if os.Getenv("OPENAI_API_KEY") != "" {
		return nil
	}
	return fmt.Errorf("no API key found (set ANTHROPIC_API_KEY or OPENAI_API_KEY)")
}

func checkPermissions() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	configDir := filepath.Join(home, ".dxrk")
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		return nil
	}
	tmp := filepath.Join(configDir, ".doctor-test")
	f, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf("cannot write to ~/.dxrk: %w", err)
	}
	_ = f.Close()
	_ = os.Remove(tmp)
	return nil
}
