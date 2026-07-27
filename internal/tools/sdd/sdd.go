// SPDX-License-Identifier: MIT
// Package sdd implements SDD phase tools that wrap SDD skills as MCP-registrable tools.
package sdd

import (
	"fmt"

	"github.com/Dxrk777/Dxrk-Ai/internal/tools"
)

const keyProjectDir = "project_dir"

type phaseInfo struct {
	Name        string
	Description string
}

var phases = []phaseInfo{
	{Name: "sdd_init", Description: "Bootstrap SDD context and project configuration"},
	{Name: "sdd_explore", Description: "Investigate codebase and think through ideas before committing"},
	{Name: "sdd_propose", Description: "Create change proposals from explorations"},
	{Name: "sdd_spec", Description: "Write detailed specifications from proposals"},
	{Name: "sdd_design", Description: "Create technical design from proposals"},
	{Name: "sdd_tasks", Description: "Break down specs and designs into implementation tasks"},
	{Name: "sdd_apply", Description: "Implement code changes from task definitions"},
	{Name: "sdd_verify", Description: "Validate implementation against specs"},
	{Name: "sdd_archive", Description: "Archive completed change artifacts"},
	{Name: "sdd_onboard", Description: "Guide user through a complete SDD cycle using their real codebase"},
}

// RegisterAll registers all SDD skills as tools in the given registry.
func RegisterAll(reg *tools.Registry) error {
	for _, p := range phases {
		t, err := tools.Build(tools.ToolDef{
			Name:        p.Name,
			Description: p.Description,
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					keyProjectDir: map[string]any{
						"type":        "string",
						"description": "Project directory to run the SDD phase in",
					},
					"phase_data": map[string]any{
						"type":        "string",
						"description": "Optional context or data for the phase",
					},
				},
				"required": []string{keyProjectDir},
			},
			Validate: func(input map[string]any) error {
				if input == nil || input[keyProjectDir] == nil {
					return fmt.Errorf("project_dir is required")
				}
				return nil
			},
			Execute: func(ctx tools.Context, input map[string]any) (any, error) {
				projectDir, _ := input[keyProjectDir].(string)
				phaseData, _ := input["phase_data"].(string)
				if ctx.Logger != nil {
					ctx.Logger.Info("sdd phase invoked",
						"phase", p.Name,
						keyProjectDir, projectDir,
					)
				}
				return map[string]any{
					"phase":       p.Name,
					"status":      "invoked",
					keyProjectDir: projectDir,
					"phase_data":  phaseData,
					"message":     fmt.Sprintf("SDD phase %q invoked for project %q", p.Name, projectDir),
				}, nil
			},
			IsReadOnly: tools.DefaultDisabled(),
		})
		if err != nil {
			return fmt.Errorf("build %q: %w", p.Name, err)
		}
		if err := reg.Register(t); err != nil {
			return fmt.Errorf("register %q: %w", p.Name, err)
		}
	}
	return nil
}
