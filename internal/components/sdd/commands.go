// SPDX-License-Identifier: MIT
package sdd

import "github.com/Dxrk777/Dxrk/internal/strconst"

type OpenCodeCommand struct {
	Name        string
	Description string
	Body        string
}

const (
	jsonDescription = strconst.StrDescription
	jsonPrompt      = "prompt"
)

const (
	PhaseInit         = "sdd-init"
	PhaseNew          = "sdd-new"
	PhaseContinue     = "sdd-continue"
	PhaseExplore      = "sdd-explore"
	PhaseFF           = "sdd-ff"
	PhaseApply        = "sdd-apply"
	PhaseVerify       = "sdd-verify"
	PhaseArchive      = "sdd-archive"
	PhaseOnboard      = "sdd-onboard"
	PhasePropose      = "sdd-propose"
	PhaseSpec         = "sdd-spec"
	PhaseDesign       = "sdd-design"
	PhaseTasks        = "sdd-tasks"
	PhaseOrchestrator = "sdd-orchestrator"
	PhaseJudgmentDay  = "judgment-day"
	PhaseDefault      = "default"
)

func OpenCodeCommands() []OpenCodeCommand {
	return []OpenCodeCommand{
		{Name: PhaseInit, Description: "Initialize SDD context", Body: "/" + PhaseInit},
		{Name: PhaseNew, Description: "Start a new SDD change", Body: "/" + PhaseNew + " ${change-name}"},
		{Name: PhaseContinue, Description: "Continue next pending artifact", Body: "/" + PhaseContinue + " ${change-name}"},
		{Name: PhaseExplore, Description: "Explore an idea before committing", Body: "/" + PhaseExplore + " ${topic}"},
		{Name: PhaseFF, Description: "Generate all planning artifacts", Body: "/" + PhaseFF + " ${change-name}"},
		{Name: PhaseApply, Description: "Implement tasks", Body: "/" + PhaseApply + " ${change-name}"},
		{Name: PhaseVerify, Description: "Verify implementation", Body: "/" + PhaseVerify + " ${change-name}"},
		{Name: PhaseArchive, Description: "Archive completed change", Body: "/" + PhaseArchive + " ${change-name}"},
		{Name: PhaseOnboard, Description: "Guided SDD walkthrough", Body: "/" + PhaseOnboard},
	}
}
