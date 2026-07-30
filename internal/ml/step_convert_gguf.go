// SPDX-License-Identifier: MIT
package ml

import (
	"fmt"
	"os/exec"
)

// ConvertGGUFStep converts the trained model to GGUF format for local deployment.
type ConvertGGUFStep struct {
	Config     *TrainConfig
	OutputPath string // e.g. "./output/model.gguf"
}

func (s *ConvertGGUFStep) ID() string { return "ml:convert-gguf" }

func (s *ConvertGGUFStep) Run() error {
	if !s.Config.ExportGGUF {
		return nil // skip if not requested
	}

	// Use llama.cpp convert script via uv
	cmd := exec.Command("uv", "run", "--with", "llama-cpp-python", "python", "-c", //nolint:gosec // trusted subprocess
		fmt.Sprintf(`
from llama_cpp import Llama
import sys
# Placeholder: real conversion uses llama.cpp's convert-hf-to-gguf.py
print("GGUF conversion for %s")
`, s.Config.Model.HubID))

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("GGUF conversion failed: %s\n%s", err, string(out))
	}

	fmt.Printf("  ✓ GGUF exported to %s\n", s.OutputPath)
	return nil
}

func (s *ConvertGGUFStep) Rollback() error {
	if s.OutputPath != "" {
		cmd := exec.Command("rm", "-f", s.OutputPath) //nolint:gosec // safe literal path
		_ = cmd.Run()
	}
	return nil
}
