// SPDX-License-Identifier: MIT
package ml

import (
	"fmt"
	"os/exec"
)

// ValidateDataStep checks that the dataset exists on the Hub and is accessible.
type ValidateDataStep struct {
	Config *TrainConfig
}

func (s *ValidateDataStep) ID() string { return "ml:validate-data" }

func (s *ValidateDataStep) Run() error {
	dataset := s.Config.Dataset.Name
	if dataset == "" {
		return fmt.Errorf("dataset name is required")
	}

	// Use huggingface-cli to check dataset access
	cmd := exec.Command("huggingface-cli", "repo", "info", "dataset", dataset)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("dataset %q not accessible: %s", dataset, string(out))
	}

	fmt.Printf("  ✓ dataset %q verified\n", dataset)
	return nil
}
