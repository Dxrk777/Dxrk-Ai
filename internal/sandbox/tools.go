// SPDX-License-Identifier: MIT
package sandbox

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Dxrk777/Dxrk-Ai/internal/strconst"
	"github.com/Dxrk777/Dxrk-Ai/internal/tools"
)

const (
	valObject      = strconst.StrObject
	keyProperties  = strconst.StrProperties
	keyType        = "type"
	valString      = strconst.StrString
	keyDescription = strconst.StrDescription
	keyTimeout     = strconst.StrTimeout
	valInteger     = strconst.StrInteger
	keyRequired    = strconst.StrRequired
	keyStdout      = strconst.StrStdout
	keyStderr      = strconst.StrStderr
	keyExitCode    = "exit_code"
	keyDurationMs  = "duration_ms"
	keyProjectPath = "project_path"
	keyTimedOut    = "timed_out"
)

// RegisterTools registers sandbox tools into the given registry.
func RegisterTools(reg *tools.Registry) error {
	toolDefs := []tools.ToolDef{
		{
			Name:        "sandbox_run_code",
			Description: "Ejecuta código en un contenedor aislado. Soporta: go, python, node, bash, rust, typescript.",
			InputSchema: map[string]any{
				keyType: valObject,
				keyProperties: map[string]any{
					"code": map[string]any{
						keyType:        valString,
						keyDescription: "Código a ejecutar",
					},
					"language": map[string]any{
						keyType:        valString,
						keyDescription: "Lenguaje: go, python, node, bash, rust, typescript",
						"enum":         []string{"go", "python", "node", "bash", "rust", "typescript"},
					},
					keyTimeout: map[string]any{
						keyType:        valInteger,
						keyDescription: "Timeout en segundos (default: 120)",
					},
				},
				keyRequired: []string{"code", "language"},
			},
			Execute: func(ctx tools.Context, input map[string]any) (any, error) {
				code, _ := input["code"].(string)
				lang, _ := input["language"].(string)
				timeoutSec, _ := input[strconst.StrTimeout].(int)

				if code == "" {
					return nil, fmt.Errorf("code is required")
				}

				pool, err := getPoolFromContext(ctx)
				if err != nil {
					return nil, err
				}

				opts := ExecOptions{}
				if timeoutSec > 0 {
					opts.Timeout = time.Duration(timeoutSec) * time.Second
				}

				result, err := pool.ExecScript(context.Background(), code, Language(lang), opts)
				if err != nil {
					return nil, err
				}

				return map[string]any{
					keyStdout:     result.Stdout,
					keyStderr:     result.Stderr,
					keyExitCode:   result.ExitCode,
					keyTimedOut:   result.TimedOut,
					keyDurationMs: result.Duration.Milliseconds(),
				}, nil
			},
		},
		{
			Name:        "sandbox_run_tests",
			Description: "Ejecuta tests en un contenedor aislado. Soporta: go test, pytest, npm test.",
			InputSchema: map[string]any{
				keyType: valObject,
				keyProperties: map[string]any{
					keyProjectPath: map[string]any{
						keyType:        valString,
						keyDescription: "Ruta absoluta del proyecto a testear",
					},
					"command": map[string]any{
						keyType:        valString,
						keyDescription: "Commando de test (default: auto-detect)",
					},
					keyTimeout: map[string]any{
						keyType:        valInteger,
						keyDescription: "Timeout en segundos (default: 300)",
					},
				},
				keyRequired: []string{keyProjectPath},
			},
			Execute: func(ctx tools.Context, input map[string]any) (any, error) {
				projectPath, _ := input[keyProjectPath].(string)
				cmdStr, _ := input["command"].(string)
				timeoutSec, _ := input[strconst.StrTimeout].(int)

				if projectPath == "" {
					return nil, fmt.Errorf("project_path is required")
				}

				if cmdStr == "" {
					cmdStr = detectTestCommand(projectPath)
				}

				pool, err := getPoolFromContext(ctx)
				if err != nil {
					return nil, err
				}

				timeout := 5 * time.Minute
				if timeoutSec > 0 {
					timeout = time.Duration(timeoutSec) * time.Second
				}

				config := ContainerConfig{
					Image:       testImageForProject(projectPath),
					WorkDir:     "/workspace",
					NetworkMode: "none",
					Cmd:         []string{"sh", "-c", cmdStr},
					Timeout:     timeout,
				}

				result, err := pool.Exec(context.Background(), config)
				if err != nil {
					return nil, err
				}

				passed := result.ExitCode == 0
				return map[string]any{
					"passed":           passed,
					strconst.StrStdout: result.Stdout,
					strconst.StrStderr: result.Stderr,
					"exit_code":        result.ExitCode,
					"timed_out":        result.TimedOut,
					"duration_ms":      result.Duration.Milliseconds(),
				}, nil
			},
		},
		{
			Name:        "sandbox_build",
			Description: "Compila un proyecto en un contenedor aislado. Soporta: go build, cargo build, npm build.",
			InputSchema: map[string]any{
				keyType: valObject,
				keyProperties: map[string]any{
					keyProjectPath: map[string]any{
						keyType:        valString,
						keyDescription: "Ruta absoluta del proyecto",
					},
					"command": map[string]any{
						keyType:        valString,
						keyDescription: "Commando de build (default: auto-detect)",
					},
					keyTimeout: map[string]any{
						keyType:        valInteger,
						keyDescription: "Timeout en segundos (default: 300)",
					},
				},
				keyRequired: []string{keyProjectPath},
			},
			Execute: func(ctx tools.Context, input map[string]any) (any, error) {
				projectPath, _ := input[keyProjectPath].(string)
				cmdStr, _ := input["command"].(string)
				timeoutSec, _ := input[strconst.StrTimeout].(int)

				if projectPath == "" {
					return nil, fmt.Errorf("project_path is required")
				}

				if cmdStr == "" {
					cmdStr = detectBuildCommand(projectPath)
				}

				pool, err := getPoolFromContext(ctx)
				if err != nil {
					return nil, err
				}

				timeout := 5 * time.Minute
				if timeoutSec > 0 {
					timeout = time.Duration(timeoutSec) * time.Second
				}

				config := ContainerConfig{
					Image:       testImageForProject(projectPath),
					WorkDir:     "/workspace",
					NetworkMode: "none",
					Cmd:         []string{"sh", "-c", cmdStr},
					Timeout:     timeout,
				}

				result, err := pool.Exec(context.Background(), config)
				if err != nil {
					return nil, err
				}

				success := result.ExitCode == 0
				return map[string]any{
					strconst.StrSuccess: success,
					keyStdout:           result.Stdout,
					keyStderr:           result.Stderr,
					keyExitCode:         result.ExitCode,
					keyDurationMs:       result.Duration.Milliseconds(),
				}, nil
			},
		},
	}

	for _, td := range toolDefs {
		t, err := tools.Build(td)
		if err != nil {
			return fmt.Errorf("build %s: %w", td.Name, err)
		}
		if err := reg.Register(t); err != nil {
			return fmt.Errorf("register %s: %w", td.Name, err)
		}
	}
	return nil
}

// SandboxContextKey is the context key for the pool.
type SandboxContextKey struct{}

func getPoolFromContext(ctx tools.Context) (*Pool, error) {
	if ctx.Context == nil {
		return nil, fmt.Errorf("no context available")
	}
	pool, ok := ctx.Value(SandboxContextKey{}).(*Pool)
	if !ok || pool == nil {
		return nil, fmt.Errorf("sandbox not configured: set sandbox.Pool in context")
	}
	return pool, nil
}

func detectTestCommand(projectPath string) string {
	if hasFile(projectPath, "go.mod") {
		return "go test ./... 2>&1"
	}
	if hasFile(projectPath, "Cargo.toml") {
		return "cargo test 2>&1"
	}
	if hasFile(projectPath, "package.json") {
		return "npm test 2>&1"
	}
	if hasFile(projectPath, "pyproject.toml") || hasFile(projectPath, "requirements.txt") {
		return "python3 -m pytest 2>&1 || python3 -m unittest 2>&1"
	}
	return "echo 'No test command detected' && exit 1"
}

func detectBuildCommand(projectPath string) string {
	if hasFile(projectPath, "go.mod") {
		return "go build ./... 2>&1"
	}
	if hasFile(projectPath, "Cargo.toml") {
		return "cargo build 2>&1"
	}
	if hasFile(projectPath, "package.json") {
		return "npm run build 2>&1"
	}
	if hasFile(projectPath, "Makefile") || hasFile(projectPath, "makefile") {
		return "make 2>&1"
	}
	return "echo 'No build command detected' && exit 1"
}

func testImageForProject(projectPath string) string {
	if hasFile(projectPath, "go.mod") {
		return golangAlpineImage
	}
	if hasFile(projectPath, "Cargo.toml") {
		return "rust:1.82-alpine"
	}
	if hasFile(projectPath, "package.json") {
		return nodeAlpineImage
	}
	if hasFile(projectPath, "pyproject.toml") || hasFile(projectPath, "requirements.txt") {
		return "python:3.13-alpine"
	}
	return defaultAlpineImage
}

func hasFile(dir, name string) bool {
	info, err := os.Stat(filepath.Join(dir, name))
	return err == nil && !info.IsDir()
}
