// SPDX-License-Identifier: MIT
package permissions

import (
	"path/filepath"
	"strings"

	"github.com/Dxrk777/Dxrk/internal/strconst"
)

// ---- Tool Categories ----

// ToolCategory classifies a tool by its primary function.
type ToolCategory int

const (
	// FileSystem covers file read/write/manipulation tools.
	FileSystem ToolCategory = iota
	// Shell covers command execution tools.
	Shell
	// Network covers HTTP, DNS, and other network tools.
	Network
	// UserInteraction covers confirmation prompts and TUI tools.
	UserInteraction
	// Internal covers internal/system tools.
	Internal
)

func (tc ToolCategory) String() string {
	switch tc {
	case FileSystem:
		return "filesystem"
	case Shell:
		return "shell"
	case Network:
		return "network"
	case UserInteraction:
		return "user_interaction"
	case Internal:
		return "internal"
	default:
		return strconst.StrUnknown
	}
}

// ---- Resource Types ----

// ResourceType identifies what kind of resource is being accessed.
type ResourceType int

const (
	// File represents a single file.
	File ResourceType = iota
	// Directory represents a directory.
	Directory
	// URL represents a web URL.
	URL
	// Command represents a shell command.
	Command
	// EnvVar represents an environment variable.
	EnvVar
	// Config represents a configuration value.
	Config
)

func (rt ResourceType) String() string {
	switch rt {
	case File:
		return "file"
	case Directory:
		return "directory"
	case URL:
		return "url"
	case Command:
		return "command"
	case EnvVar:
		return "env_var"
	case Config:
		return "config"
	default:
		return strconst.StrUnknown
	}
}

// ---- Risk Levels ----

// RiskLevel represents the severity of a permission request.
type RiskLevel int

const (
	Low RiskLevel = iota
	Medium
	High
	Critical
)

func (rl RiskLevel) String() string {
	switch rl {
	case Low:
		return "low"
	case Medium:
		return strconst.StrMedium2
	case High:
		return "high"
	case Critical:
		return strconst.StrCritical
	default:
		return strconst.StrUnknown
	}
}

// RequireConfirmation returns true if the risk level warrants user confirmation.
func RequireConfirmation(level RiskLevel) bool {
	return level >= Medium
}

// ---- Classification Maps ----

var toolCategories = map[string]ToolCategory{
	"Read":                FileSystem,
	strconst.StrWrite:     FileSystem,
	"Edit":                FileSystem,
	"Glob":                FileSystem,
	"Grep":                FileSystem,
	"LS":                  FileSystem,
	strconst.StrListfiles: FileSystem,
	"Bash":                Shell,
	strconst.StrExecute:   Shell,
	strconst.StrWebfetch:  Network,
	strconst.StrWebsearch: Network,
	strconst.StrTodoread:  Internal,
	"TodoWrite":           Internal,
	"Webpage":             Network,
	"Task":                Internal,
	"AskUser":             UserInteraction,
	"Confirm":             UserInteraction,
	"Notify":              UserInteraction,
}

var readOnlyTools = map[string]bool{
	"Read":                true,
	"Glob":                true,
	"Grep":                true,
	"LS":                  true,
	strconst.StrListfiles: true,
	strconst.StrWebfetch:  true,
	strconst.StrWebsearch: true,
	strconst.StrTodoread:  true,
	"AskUser":             true,
}

var sensitiveToolResources = map[string]RiskLevel{
	"Bash":               High,
	strconst.StrExecute:  High,
	strconst.StrWrite:    Medium,
	"Edit":               Medium,
	strconst.StrWebfetch: Low,
	"Read":               Low,
	"Glob":               Low,
	"Grep":               Low,
	"LS":                 Low,
}

// DangerousCommandPrefixes lists command prefixes that increase risk.
var DangerousCommandPrefixes = []string{
	"rm ", "rm\t", "rmdir",
	"sudo", "su ", "doas",
	"dd ", "mkfs", strconst.StrFormat,
	"curl ", "wget ",
	"eval ", "exec ",
	"chmod 777", "chown root",
	"> /dev/", ">> /dev/",
	"git push", "git commit",
	"npm publish", "pip upload",
	"docker run", "kubectl exec",
	"DROP TABLE", "DELETE FROM",
	"TRUNCATE",
}

// ---- Classification Functions ----

// ClassifyTool returns the category of a tool by name.
func ClassifyTool(toolName string) ToolCategory {
	if cat, ok := toolCategories[toolName]; ok {
		return cat
	}
	return Internal
}

// ClassifyResource determines the resource type from a resource string.
func ClassifyResource(resource string) ResourceType {
	if resource == "" {
		return Command
	}
	lower := strings.ToLower(resource)

	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return URL
	}
	if strings.HasPrefix(lower, "$") || strings.HasPrefix(lower, "env:") {
		return EnvVar
	}
	if strings.Contains(lower, "config") || strings.HasSuffix(lower, ".json") ||
		strings.HasSuffix(lower, ".yaml") || strings.HasSuffix(lower, ".yml") ||
		strings.HasSuffix(lower, ".toml") || strings.HasSuffix(lower, ".env") {
		return Config
	}
	if strings.HasSuffix(resource, "/") || resource == "." || resource == ".." {
		return Directory
	}
	ext := filepath.Ext(resource)
	if ext != "" {
		return File
	}
	if strings.ContainsAny(resource, ";&|`$(){}[]!") {
		return Command
	}
	return File
}

// AssessRisk evaluates the combined risk of a tool and resource.
func AssessRisk(tool string, resource string) RiskLevel {
	base, ok := sensitiveToolResources[tool]
	if !ok {
		base = Medium
	}

	if tool == "Bash" || tool == strconst.StrExecute {
		for _, prefix := range DangerousCommandPrefixes {
			if strings.Contains(strings.ToLower(resource), strings.ToLower(prefix)) {
				if base < High {
					base = High
				}
				if strings.HasPrefix(strings.ToLower(prefix), "rm ") ||
					strings.HasPrefix(strings.ToLower(prefix), "sudo") ||
					strings.HasPrefix(strings.ToLower(prefix), "dd ") ||
					strings.Contains(strings.ToLower(prefix), "drop table") {
					return Critical
				}
			}
		}
	}

	if ClassifyResource(resource) == URL && base < Medium {
		base = Medium
	}

	if tool == strconst.StrWrite || tool == "Edit" {
		if strings.Contains(resource, "..") || strings.HasPrefix(resource, "/") {
			if base < High {
				base = High
			}
		}
	}

	return base
}

// IsReadOnly returns true if the tool performs no side effects.
func IsReadOnly(tool string) bool {
	return readOnlyTools[tool]
}

// ToolRiskSummary returns a human-readable risk summary for a tool+resource pair.
func ToolRiskSummary(tool, resource string) string {
	level := AssessRisk(tool, resource)
	cat := ClassifyTool(tool)
	resType := ClassifyResource(resource)

	return strings.Join([]string{
		"tool=" + tool,
		"category=" + cat.String(),
		"resource_type=" + resType.String(),
		"risk=" + level.String(),
	}, " ")
}
