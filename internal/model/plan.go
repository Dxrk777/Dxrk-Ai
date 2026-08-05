// SPDX-License-Identifier: MIT
package model

import "github.com/Dxrk777/Dxrk-Ai/internal/strconst"

type PlanStatus string

const (
	PlanStatusPending   PlanStatus = strconst.StrPending
	PlanStatusRunning   PlanStatus = strconst.StrRunning
	PlanStatusSucceeded PlanStatus = "succeeded"
	PlanStatusFailed    PlanStatus = strconst.StrFailed
)

type RunResult string

const (
	RunResultSkipped RunResult = "skipped"
	RunResultSuccess RunResult = strconst.StrSuccess
	RunResultFailed  RunResult = strconst.StrFailed
)

type PlanStep struct {
	ID     string
	Name   string
	Status PlanStatus
	Result RunResult
	Error  string
}

type Plan struct {
	ID        string
	Selection Selection
	Status    PlanStatus
	Steps     []PlanStep
}
