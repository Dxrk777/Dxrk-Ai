// SPDX-License-Identifier: MIT
package sandbox

import (
	"context"
	"time"
)

const (
	golangAlpineImage = "golang:1.25-alpine"
	nodeAlpineImage   = "node:22-alpine"
)

// ContainerConfig holds configuration for a sandbox container.
type ContainerConfig struct {
	Image       string
	WorkDir     string
	Env         map[string]string
	NetworkMode string
	AutoRemove  bool
	Entrypoint  []string
	Cmd         []string
	Timeout     time.Duration
}

// ExecutionResult holds the result of a command execution.
type ExecutionResult struct {
	ExitCode    int
	Stdout      string
	Stderr      string
	Duration    time.Duration
	TimedOut    bool
	OOMKilled   bool
	ContainerID string
}

// PoolConfig configures the container pool.
type PoolConfig struct {
	MaxContainers int
	IdleTimeout   time.Duration
	DefaultImage  string
	DockerCmd     string // "docker" or "podman"
}

// PoolStats provides pool statistics.
type PoolStats struct {
	TotalContainers  int
	ActiveContainers int
	IdleContainers   int
	TotalExecutions  int64
	FailedExecutions int64
}

// Sandbox defines the interface for code execution sandbox.
type Sandbox interface {
	Exec(ctx context.Context, config ContainerConfig) (ExecutionResult, error)
	ExecScript(ctx context.Context, script string, lang Language, opts ExecOptions) (ExecutionResult, error)
	GetContainer(ctx context.Context, sessionID string) (string, error)
	ReleaseContainer(ctx context.Context, sessionID string) error
	Stats() PoolStats
	Close() error
}

// Language represents a programming language for script execution.
type Language string

const (
	LanguageGo         Language = "go"
	LanguagePython     Language = "python"
	LanguageNode       Language = "node"
	LanguageBash       Language = "bash"
	LanguageRust       Language = "rust"
	LanguageTypeScript Language = "typescript"
)

// ExecOptions configures script execution.
type ExecOptions struct {
	WorkDir     string
	Env         map[string]string
	Files       map[string]string // filename -> content
	Timeout     time.Duration
	NetworkMode string
}

// DefaultPoolConfig returns sensible pool defaults.
func DefaultPoolConfig() PoolConfig {
	return PoolConfig{
		MaxContainers: 10,
		IdleTimeout:   5 * time.Minute,
		DefaultImage:  golangAlpineImage,
		DockerCmd:     "docker",
	}
}

// ImageForLanguage returns a suitable Docker image for a language.
func ImageForLanguage(lang Language) string {
	switch lang {
	case LanguageGo:
		return golangAlpineImage
	case LanguagePython:
		return "python:3.13-alpine"
	case LanguageNode:
		return nodeAlpineImage
	case LanguageBash:
		return defaultAlpineImage
	case LanguageRust:
		return "rust:1.82-alpine"
	case LanguageTypeScript:
		return nodeAlpineImage
	default:
		return defaultAlpineImage
	}
}

// CommandForLanguage returns the command to run a script for a language.
func CommandForLanguage(lang Language, scriptName string) []string {
	switch lang {
	case LanguageGo:
		return []string{"go", "run", scriptName}
	case LanguagePython:
		return []string{"python3", scriptName}
	case LanguageNode:
		return []string{"node", scriptName}
	case LanguageBash:
		return []string{"sh", scriptName}
	case LanguageRust:
		return []string{"sh", "-c", "rustc " + scriptName + " -o /tmp/out && /tmp/out"}
	case LanguageTypeScript:
		return []string{"npx", "ts-node", scriptName}
	default:
		return []string{"sh", scriptName}
	}
}

func scriptFileName(lang Language) string {
	switch lang {
	case LanguageGo:
		return "main.go"
	case LanguagePython:
		return "main.py"
	case LanguageNode:
		return "main.js"
	case LanguageBash:
		return "script.sh"
	case LanguageRust:
		return "main.rs"
	case LanguageTypeScript:
		return "main.ts"
	default:
		return "script.sh"
	}
}
