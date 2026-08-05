// SPDX-License-Identifier: MIT
package pipeline

import (
	"time"

	"github.com/Dxrk777/Dxrk-Ai/internal/strconst"
)

type StepStatus string

const (
	StepStatusPending    StepStatus = strconst.StrPending
	StepStatusRunning    StepStatus = strconst.StrRunning
	StepStatusSucceeded  StepStatus = "succeeded"
	StepStatusFailed     StepStatus = strconst.StrFailed
	StepStatusRolledBack StepStatus = "rolled-back"
	StepStatusSkipped    StepStatus = "skipped"
)

type StepResult struct {
	StepID     string
	Status     StepStatus
	StartedAt  time.Time
	FinishedAt time.Time
	Err        error
}

type StageResult struct {
	Stage   Stage
	Steps   []StepResult
	Success bool
	Err     error
}

type ExecutionResult struct {
	Prepare  StageResult
	Apply    StageResult
	Rollback StageResult
	Err      error
}
