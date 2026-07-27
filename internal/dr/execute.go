// SPDX-License-Identifier: MIT
package dr

import (
	"bytes"
	"context"
	"os/exec"
	"time"
)

type executeConfig struct {
	timeout time.Duration
	dryRun  bool
}

type ExecuteOption func(*executeConfig)

func WithTimeout(d time.Duration) ExecuteOption {
	return func(c *executeConfig) {
		c.timeout = d
	}
}

func WithDryRun() ExecuteOption {
	return func(c *executeConfig) {
		c.dryRun = true
	}
}

func defaultConfig() executeConfig {
	return executeConfig{
		timeout: 30 * time.Second,
	}
}

func executeStep(ctx context.Context, step RecoveryStep, cfg executeConfig) RecoveryResult {
	start := time.Now()
	result := RecoveryResult{Step: step}

	if cfg.dryRun {
		result.Success = true
		result.Output = "[dry-run] would execute: " + step.Command
		result.Duration = time.Since(start)
		return result
	}

	stepCtx := ctx
	if cfg.timeout > 0 {
		var cancel context.CancelFunc
		stepCtx, cancel = context.WithTimeout(ctx, cfg.timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(stepCtx, "sh", "-c", step.Command)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result.Duration = time.Since(start)
	result.Output = stdout.String()
	if stderr.Len() > 0 {
		if result.Output != "" {
			result.Output += "\n"
		}
		result.Output += stderr.String()
	}

	if err != nil {
		result.Success = false
		result.Error = err.Error()
	} else {
		result.Success = true
	}

	return result
}
