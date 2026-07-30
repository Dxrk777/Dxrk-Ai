// SPDX-License-Identifier: MIT
package ml

import (
	"fmt"

	"github.com/Dxrk777/Dxrk-Ai/internal/pipeline"
)

// BuildPipeline constructs a StagePlan from a TrainConfig.
// Prepare stage: validate data.
// Apply stage: train, optionally push hub, optionally convert GGUF.
func BuildPipeline(cfg *TrainConfig) pipeline.StagePlan {
	var prepare []pipeline.Step
	var apply []pipeline.Step

	// ── Prepare ──
	prepare = append(prepare, &ValidateDataStep{Config: cfg})

	// ── Apply ──
	apply = append(apply, &TrainStep{Config: cfg})

	if cfg.PushToHub {
		apply = append(apply, &PushHubStep{Config: cfg})
	}

	if cfg.ExportGGUF {
		apply = append(apply, &ConvertGGUFStep{
			Config:     cfg,
			OutputPath: fmt.Sprintf("./%s.gguf", cfg.Model.HubID),
		})
	}

	return pipeline.StagePlan{
		Prepare: prepare,
		Apply:   apply,
	}
}
