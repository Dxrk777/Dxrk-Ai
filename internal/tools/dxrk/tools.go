// SPDX-License-Identifier: MIT
// Package dxrk implements concrete Dxrk tools using the tools framework.
package dxrk

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Dxrk777/Dxrk/internal/agents"
	"github.com/Dxrk777/Dxrk/internal/model"
	"github.com/Dxrk777/Dxrk/internal/skillregistry"
	"github.com/Dxrk777/Dxrk/internal/strconst"
	"github.com/Dxrk777/Dxrk/internal/system"
	"github.com/Dxrk777/Dxrk/internal/tools"
)

const (
	valObject      = strconst.StrObject
	keyProperties  = strconst.StrProperties
	valString      = strconst.StrString
	keyDescription = strconst.StrDescription
	keyAgent       = "agent"
	keyRequired    = strconst.StrRequired
	valInteger     = strconst.StrInteger
	keyPattern     = strconst.StrPattern
	keyError       = strconst.StrError
	keyFiles       = strconst.StrFiles
	keyAgents      = "agents"
	keyType        = "type"
	keyCount       = strconst.StrCount
	keyResults     = "results"
)

// RegisterAll registers all Dxrk tools into the given registry.
func RegisterAll(reg *tools.Registry, agentReg *agents.Registry) error {
	for _, fn := range []func(*tools.Registry, *agents.Registry) error{
		registerDetectAgents,
		registerDetectAgent,
		registerSystemInfo,
		registerListSkills,
		registerRunDiagnostic,
		registerReadFile,
		registerGrepSearch,
		registerGlobSearch,
	} {
		if err := fn(reg, agentReg); err != nil {
			return err
		}
	}
	return nil
}

func registerDetectAgents(reg *tools.Registry, agentReg *agents.Registry) error {
	t, err := tools.Build(tools.ToolDef{
		Name:        "detect_agents",
		Description: "List all installed AI coding agents with their config directories",
		InputSchema: map[string]any{
			keyType: valObject,
			keyProperties: map[string]any{
				"home_dir": map[string]any{
					"type":         strconst.StrString,
					keyDescription: "Home directory (defaults to $HOME)",
				},
			},
		},
		Execute: func(ctx tools.Context, input map[string]any) (any, error) {
			homeDir := getHomeDir(input)
			installed := agents.DiscoverInstalled(agentReg, homeDir)
			if len(installed) == 0 {
				return map[string]any{keyAgents: []any{}, keyCount: 0}, nil
			}
			result := make([]map[string]any, len(installed))
			for i, a := range installed {
				result[i] = map[string]any{
					"id":         string(a.ID),
					"config_dir": a.ConfigDir,
				}
			}
			return map[string]any{keyAgents: result, keyCount: len(result)}, nil
		},
		IsReadOnly: defaultTrue(),
	})
	if err != nil {
		return err
	}
	return reg.Register(t)
}

func registerDetectAgent(reg *tools.Registry, agentReg *agents.Registry) error {
	t, err := tools.Build(tools.ToolDef{
		Name:        "detect_agent",
		Description: "Check if a specific AI coding agent is installed",
		InputSchema: map[string]any{
			keyType: valObject,
			keyProperties: map[string]any{
				keyAgent: map[string]any{
					keyType:        valString,
					keyDescription: "Agent ID (e.g. claude-code, opencode, cursor)",
				},
				"home_dir": map[string]any{
					"type":         strconst.StrString,
					keyDescription: "Home directory (defaults to $HOME)",
				},
			},
			keyRequired: []string{keyAgent},
		},
		Validate: func(input map[string]any) error {
			if input == nil || input[keyAgent] == nil {
				return fmt.Errorf("agent is required")
			}
			return nil
		},
		Execute: func(ctx tools.Context, input map[string]any) (any, error) {
			homeDir := getHomeDir(input)
			agentID := model.AgentID(input[keyAgent].(string))
			adapter, err := agents.NewAdapter(agentID)
			if err != nil {
				return nil, fmt.Errorf("unknown agent %q", agentID)
			}
			installed, binaryPath, configPath, configFound, err := adapter.Detect(context.Background(), homeDir)
			if err != nil {
				return nil, fmt.Errorf("detect %q: %w", agentID, err)
			}
			return map[string]any{
				keyAgent:       string(agentID),
				"installed":    installed,
				"binary_path":  binaryPath,
				"config_path":  configPath,
				"config_found": configFound,
				"tier":         string(adapter.Tier()),
			}, nil
		},
		IsReadOnly: defaultTrue(),
	})
	if err != nil {
		return err
	}
	return reg.Register(t)
}

func registerSystemInfo(reg *tools.Registry, _ *agents.Registry) error {
	t, err := tools.Build(tools.ToolDef{
		Name:        "system_info",
		Description: "Get detailed system information (OS, arch, shell, tools, configs)",
		InputSchema: map[string]any{
			"type":        strconst.StrObject,
			keyProperties: map[string]any{},
		},
		Execute: func(ctx tools.Context, _ map[string]any) (any, error) {
			result, err := system.Detect(context.Background())
			if err != nil {
				return nil, fmt.Errorf("system detect: %w", err)
			}
			tools := make(map[string]string)
			for name, status := range result.Tools {
				tools[name] = status.Path
				if !status.Installed {
					tools[name] = "(not installed)"
				}
			}
			configs := make([]map[string]any, len(result.Configs))
			for i, c := range result.Configs {
				configs[i] = map[string]any{
					"agent": c.Agent, "path": c.Path,
					"exists": c.Exists, "is_dir": c.IsDirectory,
				}
			}
			return map[string]any{
				"os":           result.System.OS,
				"arch":         result.System.Arch,
				"shell":        result.System.Shell,
				"supported":    result.System.Supported,
				"tools":        tools,
				"configs":      configs,
				"dependencies": result.Dependencies,
			}, nil
		},
		IsReadOnly: defaultTrue(),
	})
	if err != nil {
		return err
	}
	return reg.Register(t)
}

func registerListSkills(reg *tools.Registry, _ *agents.Registry) error {
	t, err := tools.Build(tools.ToolDef{
		Name:        "list_skills",
		Description: "List all available skills from project and user directories",
		InputSchema: map[string]any{
			keyType: valObject,
			keyProperties: map[string]any{
				"project_dir": map[string]any{
					keyType:        valString,
					keyDescription: "Project directory (defaults to CWD)",
				},
			},
		},
		Execute: func(ctx tools.Context, input map[string]any) (any, error) {
			projectDir := getProjectDir(input)
			homeDir, _ := os.UserHomeDir()
			dirs := findSkillDirs(projectDir, homeDir)
			skills := make([]map[string]any, 0)
			seen := make(map[string]bool)
			for _, dir := range dirs {
				entries, err := os.ReadDir(dir)
				if err != nil {
					continue
				}
				for _, entry := range entries {
					if !entry.IsDir() {
						continue
					}
					skillPath := filepath.Join(dir, entry.Name(), "SKILL.md")
					if _, err := os.Stat(skillPath); err != nil {
						continue
					}
					name := entry.Name()
					if seen[name] {
						continue
					}
					seen[name] = true
					skills = append(skills, map[string]any{
						"name": name,
						"path": skillPath,
						"type": strconst.StrLocal,
					})
				}
			}
			sort.Slice(skills, func(i, j int) bool {
				return skills[i]["name"].(string) < skills[j]["name"].(string)
			})
			return map[string]any{"skills": skills, keyCount: len(skills)}, nil
		},
		IsReadOnly: defaultTrue(),
	})
	if err != nil {
		return err
	}
	return reg.Register(t)
}

func registerRunDiagnostic(reg *tools.Registry, agentReg *agents.Registry) error {
	t, err := tools.Build(tools.ToolDef{
		Name:        "run_diagnostic",
		Description: "Run a comprehensive system diagnostic: agents, tools, configs, dependencies",
		InputSchema: map[string]any{
			keyType: valObject,
			keyProperties: map[string]any{
				"include_configs": map[string]any{
					"type":         "boolean",
					keyDescription: "Include detailed config file inspection (default: true)",
				},
			},
		},
		Execute: func(ctx tools.Context, input map[string]any) (any, error) {
			homeDir, _ := os.UserHomeDir()
			includeConfigs := true
			if v, ok := input["include_configs"].(bool); ok {
				includeConfigs = v
			}

			sysResult, err := system.Detect(context.Background())
			if err != nil {
				return nil, err
			}

			installed := agents.DiscoverInstalled(agentReg, homeDir)
			agentList := make([]string, len(installed))
			for i, a := range installed {
				agentList[i] = string(a.ID)
			}

			toolList := make([]map[string]any, 0)
			for name, status := range sysResult.Tools {
				toolList = append(toolList, map[string]any{
					"name": name, "installed": status.Installed, "path": status.Path,
				})
			}
			sort.Slice(toolList, func(i, j int) bool {
				return toolList[i]["name"].(string) < toolList[j]["name"].(string)
			})

			diag := map[string]any{
				strconst.StrSystem: map[string]any{
					"os": sysResult.System.OS, "arch": sysResult.System.Arch,
					"shell": sysResult.System.Shell, "supported": sysResult.System.Supported,
				},
				"agents":      agentList,
				"agent_count": len(agentList),
				"tools":       toolList,
			}
			if includeConfigs {
				configs := make([]map[string]any, len(sysResult.Configs))
				for i, c := range sysResult.Configs {
					configs[i] = map[string]any{
						"agent": c.Agent, "exists": c.Exists,
					}
				}
				diag["configs"] = configs
			}
			return diag, nil
		},
		IsReadOnly: defaultTrue(),
	})
	if err != nil {
		return err
	}
	return reg.Register(t)
}

func registerReadFile(reg *tools.Registry, _ *agents.Registry) error {
	t, err := tools.Build(tools.ToolDef{
		Name:        "read_file",
		Description: "Read the contents of a file from disk",
		InputSchema: map[string]any{
			keyType: valObject,
			keyProperties: map[string]any{
				"path": map[string]any{
					keyType:        valString,
					keyDescription: "Absolute path to the file to read",
				},
				"max_size": map[string]any{
					keyType:        valInteger,
					keyDescription: "Maximum bytes to read (default 1MB)",
				},
			},
			keyRequired: []string{"path"},
		},
		Validate: func(input map[string]any) error {
			if input == nil || input["path"] == nil {
				return fmt.Errorf("path is required")
			}
			return nil
		},
		Execute: func(ctx tools.Context, input map[string]any) (any, error) {
			path := input["path"].(string)
			maxSize := 1024 * 1024 // 1MB default
			if m, ok := input["max_size"].(float64); ok {
				maxSize = int(m)
			} else if m, ok := input["max_size"].(int); ok {
				maxSize = m
			}
			info, err := os.Stat(path)
			if err != nil {
				return nil, fmt.Errorf("stat %q: %w", path, err)
			}
			if info.IsDir() {
				return nil, fmt.Errorf("%q is a directory, not a file", path)
			}
			if info.Size() > int64(maxSize) {
				return nil, fmt.Errorf("file %q exceeds max_size (%d > %d)", path, info.Size(), maxSize)
			}
			data, err := os.ReadFile(path) //nolint:gosec
			if err != nil {
				return nil, fmt.Errorf("read %q: %w", path, err)
			}
			return map[string]any{
				"path":              path,
				"size":              len(data),
				strconst.StrContent: string(data),
			}, nil
		},
		IsReadOnly: defaultTrue(),
	})
	if err != nil {
		return err
	}
	return reg.Register(t)
}

func checkRgAvailable() error {
	if _, err := exec.LookPath("rg"); err != nil {
		return fmt.Errorf("ripgrep (rg) is required but not found in PATH; install it via 'brew install ripgrep', 'apt install ripgrep', or 'cargo install ripgrep'")
	}
	return nil
}

func registerGrepSearch(reg *tools.Registry, _ *agents.Registry) error {
	t, err := tools.Build(tools.ToolDef{
		Name:        "grep_search",
		Description: "Search file contents using regex patterns (wraps ripgrep)",
		InputSchema: map[string]any{
			keyType: valObject,
			keyProperties: map[string]any{
				keyPattern: map[string]any{
					keyType:        valString,
					keyDescription: "Regex pattern to search for",
				},
				"path": map[string]any{
					keyType:        valString,
					keyDescription: "Directory to search in (defaults to CWD)",
				},
				"include": map[string]any{
					keyType:        valString,
					keyDescription: "File glob pattern (e.g. *.go)",
				},
				"max_results": map[string]any{
					keyType:        valInteger,
					keyDescription: "Maximum results (default 50)",
				},
			},
			keyRequired: []string{strconst.StrPattern},
		},
		Validate: func(input map[string]any) error {
			if input == nil || input[strconst.StrPattern] == nil {
				return fmt.Errorf("pattern is required")
			}
			return nil
		},
		Execute: func(ctx tools.Context, input map[string]any) (any, error) {
			if err := checkRgAvailable(); err != nil {
				return map[string]any{keyError: err.Error(), keyResults: []string{}, keyCount: 0}, nil //nolint:nilerr
			}
			pattern := input[strconst.StrPattern].(string)
			searchPath := getProjectDir(input)
			if p, ok := input["path"].(string); ok {
				searchPath = p
			}
			args := []string{"--no-heading", "--with-filename", "--line-number", "-i"}
			if include, ok := input["include"].(string); ok {
				args = append(args, "--glob", include)
			}
			maxResults := 50
			if m, ok := input["max_results"].(float64); ok {
				maxResults = int(m)
			}
			args = append(args, pattern, searchPath)
			cmd := exec.Command("rg", args...)
			output, err := cmd.Output()
			if err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok && len(exitErr.Stderr) > 0 {
					return nil, fmt.Errorf("rg: %s", string(exitErr.Stderr))
				}
				return map[string]any{keyResults: []string{}, keyCount: 0, keyError: err.Error()}, nil
			}
			lines := strings.Split(strings.TrimSpace(string(output)), "\n")
			if len(lines) > maxResults {
				lines = lines[:maxResults]
			}
			return map[string]any{
				keyResults: lines, keyCount: len(lines),
				strconst.StrTruncated: len(lines) > maxResults,
			}, nil
		},
		IsReadOnly: defaultTrue(),
	})
	if err != nil {
		return err
	}
	return reg.Register(t)
}

func registerGlobSearch(reg *tools.Registry, _ *agents.Registry) error {
	t, err := tools.Build(tools.ToolDef{
		Name:        "glob_search",
		Description: "Search for files by glob pattern (wraps ripgrep --files)",
		InputSchema: map[string]any{
			keyType: valObject,
			keyProperties: map[string]any{
				keyPattern: map[string]any{
					keyType:        valString,
					keyDescription: "Glob pattern (e.g. **/*.go)",
				},
				"path": map[string]any{
					keyType:        valString,
					keyDescription: "Directory to search in (defaults to CWD)",
				},
				"max_results": map[string]any{
					keyType:        valInteger,
					keyDescription: "Maximum results (default 100)",
				},
			},
			keyRequired: []string{strconst.StrPattern},
		},
		Validate: func(input map[string]any) error {
			if input == nil || input[strconst.StrPattern] == nil {
				return fmt.Errorf("pattern is required")
			}
			return nil
		},
		Execute: func(ctx tools.Context, input map[string]any) (any, error) {
			if err := checkRgAvailable(); err != nil {
				return map[string]any{keyError: err.Error(), keyFiles: []string{}, keyCount: 0}, nil //nolint:nilerr
			}
			pattern := input[strconst.StrPattern].(string)
			searchPath := getProjectDir(input)
			if p, ok := input["path"].(string); ok {
				searchPath = p
			}
			maxResults := 100
			if m, ok := input["max_results"].(float64); ok {
				maxResults = int(m)
			}
			cmd := exec.Command("rg", "--files", "--glob", pattern, searchPath) //nolint:gosec
			output, err := cmd.Output()
			if err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok && len(exitErr.Stderr) > 0 {
					return nil, fmt.Errorf("rg --files: %s", string(exitErr.Stderr))
				}
				return map[string]any{strconst.StrFiles: []string{}, strconst.StrCount: 0, strconst.StrError: err.Error()}, nil
			}
			files := strings.Split(strings.TrimSpace(string(output)), "\n")
			if len(files) > maxResults {
				files = files[:maxResults]
			}
			return map[string]any{
				strconst.StrFiles: files, strconst.StrCount: len(files),
				strconst.StrTruncated: len(files) > maxResults,
			}, nil
		},
		IsReadOnly: defaultTrue(),
	})
	if err != nil {
		return err
	}
	return reg.Register(t)
}

func getHomeDir(input map[string]any) string {
	if input != nil {
		if h, ok := input["home_dir"].(string); ok && h != "" {
			return h
		}
	}
	home, _ := os.UserHomeDir()
	return home
}

func getProjectDir(input map[string]any) string {
	if input != nil {
		if d, ok := input["project_dir"].(string); ok && d != "" {
			return d
		}
	}
	cwd, _ := os.Getwd()
	return cwd
}

func findSkillDirs(projectDir, homeDir string) []string {
	dirs := skillregistry.ProjectSkillDirs(projectDir)
	dirs = append(dirs, skillregistry.UserSkillDirs(homeDir)...)
	var existing []string
	for _, d := range dirs {
		if info, err := os.Stat(d); err == nil && info.IsDir() {
			existing = append(existing, d)
		}
	}
	return existing
}

func defaultTrue() *bool { b := true; return &b }
