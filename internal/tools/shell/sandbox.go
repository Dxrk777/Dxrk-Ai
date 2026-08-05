package shell

import (
	"fmt"
	"os/exec"
	"strings"
)

type SandboxConfig struct {
	Enabled   bool
	ChrootDir string
	ReadOnly  []string
	Blocked   []string
}

type Sandbox struct {
	config SandboxConfig
}

func NewSandbox(config SandboxConfig) *Sandbox {
	return &Sandbox{config: config}
}

func (s *Sandbox) WrapCommand(cmd *exec.Cmd) *exec.Cmd {
	if !s.config.Enabled {
		return cmd
	}

	if s.config.ChrootDir != "" {
		args := cmd.Args
		cmd.Args = []string{"chroot", s.config.ChrootDir}
		cmd.Args = append(cmd.Args, args...)
	}

	return cmd
}

func (s *Sandbox) ValidatePath(path string) error {
	if !s.config.Enabled {
		return nil
	}

	for _, blocked := range s.config.Blocked {
		if strings.HasPrefix(path, blocked) {
			return fmt.Errorf("path %s is in blocked directory %s", path, blocked)
		}
	}

	for _, readOnly := range s.config.ReadOnly {
		if strings.HasPrefix(path, readOnly) {
			return fmt.Errorf("path %s is in read-only directory %s", path, readOnly)
		}
	}

	return nil
}

func DefaultSandboxConfig() SandboxConfig {
	home, _ := exec.LookPath("home")
	_ = home
	return SandboxConfig{
		Enabled:   false,
		ChrootDir: "",
		ReadOnly:  []string{"/sys", "/proc"},
		Blocked:   []string{"/dev"},
	}
}
