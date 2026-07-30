// SPDX-License-Identifier: MIT
package ml

import (
	"fmt"
	"os/exec"
)

// PushHubStep pushes the trained model to the Hugging Face Hub.
type PushHubStep struct {
	Config *TrainConfig
}

func (s *PushHubStep) ID() string { return "ml:push-hub" }

func (s *PushHubStep) Run() error {
	if !s.Config.PushToHub {
		return nil
	}

	cmd := exec.Command("huggingface-cli", "upload",
		s.Config.Model.HubID,
		"--repo-type", "model",
	)
	cmd.Env = append(cmd.Environ(), "HF_TOKEN="+s.Config.HFToken)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("hub push failed: %s\n%s", err, string(out))
	}

	fmt.Printf("  ✓ pushed to https://huggingface.co/%s\n", s.Config.Model.HubID)
	return nil
}
