// SPDX-License-Identifier: MIT
package catalog

import "github.com/Dxrk777/Dxrk-Ai/internal/model"

type Component struct {
	ID          model.ComponentID
	Name        string
	Description string
}

var mvpComponents = []Component{
	{ID: model.ComponentDxrkMemory, Name: "Engram", Description: "Persistent cross-session memory"},
	{ID: model.ComponentSDD, Name: "SDD", Description: "Spec-driven development workflow"},
	{ID: model.ComponentSkills, Name: "Skills", Description: "Curated coding skill library"},
	{ID: model.ComponentContext7, Name: "Context7", Description: "Latest framework and library docs"},
	{ID: model.ComponentPersona, Name: "Persona", Description: "Dxrk, neutral or custom behavior"},
	{ID: model.ComponentPermission, Name: "Permissions", Description: "Security-first defaults and guardrails"},
	{ID: model.ComponentDxrkGuardian, Name: "Dxrk Guardian", Description: "Dxrk Guardian Angel — AI provider switcher"},
	{ID: model.ComponentTheme, Name: "Theme", Description: "Dxrk Kanagawa theme overlay"},
	{ID: model.ComponentClaudeTheme, Name: "Claude Dxrk Theme", Description: "Claude Code Dxrk custom theme"},
	{ID: model.ComponentOpenCodeDxrkLogo, Name: "OpenCode Dxrk Logo", Description: "OpenCode home logo TUI plugin with Braille rose"},
}

func MVPComponents() []Component {
	components := make([]Component, len(mvpComponents))
	copy(components, mvpComponents)
	return components
}
