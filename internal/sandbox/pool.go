// SPDX-License-Identifier: MIT
package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	defaultAlpineImage = "alpine:latest"
	defaultWorkDir     = "/workspace"
)

// Pool implements a container pool using docker/podman CLI.
type Pool struct {
	mu         sync.Mutex
	config     PoolConfig
	containers map[string]string // sessionID -> containerID
	stats      PoolStats
	idleTicker *time.Ticker
	stopCh     chan struct{}
	wg         sync.WaitGroup
}

// safePathRe matches characters allowed in a shell path to prevent command injection.
var safePathRe = regexp.MustCompile(`[^a-zA-Z0-9_\-./]`)

// sanitizePath strips characters that could enable shell injection in Docker exec commands.
func sanitizePath(p string) string {
	return safePathRe.ReplaceAllString(p, "")
}

// NewPool creates a new container pool.
func NewPool(config PoolConfig) (*Pool, error) {
	if config.MaxContainers <= 0 {
		config.MaxContainers = 10
	}
	if config.IdleTimeout <= 0 {
		config.IdleTimeout = 5 * time.Minute
	}
	if config.DefaultImage == "" {
		config.DefaultImage = defaultAlpineImage
	}
	if config.DockerCmd == "" {
		config.DockerCmd = "docker"
	}

	if _, err := exec.LookPath(config.DockerCmd); err != nil {
		return nil, fmt.Errorf("%s not found: %w", config.DockerCmd, err)
	}

	p := &Pool{
		config:     config,
		containers: make(map[string]string),
		stopCh:     make(chan struct{}),
	}

	p.idleTicker = time.NewTicker(config.IdleTimeout)
	p.wg.Add(1)
	go p.idleReaper()

	return p, nil
}

// Exec runs a command in a container and returns the result.
func (p *Pool) Exec(ctx context.Context, config ContainerConfig) (ExecutionResult, error) {
	start := time.Now()

	if config.Image == "" {
		config.Image = p.config.DefaultImage
	}
	if config.WorkDir == "" {
		config.WorkDir = defaultWorkDir
	}
	if config.Timeout == 0 {
		config.Timeout = 2 * time.Minute
	}

	args := []string{"run", "--rm", "-i"}
	args = append(args, "-w", config.WorkDir)

	for k, v := range config.Env {
		args = append(args, "-e", k+"="+v)
	}

	if config.NetworkMode != "" {
		args = append(args, "--network", config.NetworkMode)
	} else {
		args = append(args, "--network", "none")
	}

	args = append(args, config.Image)
	if len(config.Cmd) > 0 {
		args = append(args, config.Cmd...)
	}

	if config.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, config.Timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, p.config.DockerCmd, args...) //nolint:gosec

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	duration := time.Since(start)

	var result ExecutionResult
	result.Duration = duration
	result.Stdout = strings.TrimSpace(stdout.String())
	result.Stderr = strings.TrimSpace(stderr.String())

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			result.TimedOut = true
			result.ExitCode = -1
		} else if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			return result, fmt.Errorf("docker exec: %w", err)
		}
	}

	p.recordResult(result)
	return result, nil
}

// ExecScript executes code via stdin piping. No mount/perm issues.
func (p *Pool) ExecScript(ctx context.Context, script string, lang Language, opts ExecOptions) (ExecutionResult, error) {
	image := ImageForLanguage(lang)

	workDir := opts.WorkDir
	if workDir == "" {
		workDir = defaultWorkDir
	}

	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 2 * time.Minute
	}

	workDir = sanitizePath(workDir)
	wrapScript := fmt.Sprintf("mkdir -p %s && cd %s\n%s", workDir, workDir, script)

	args := []string{"run", "--rm", "-i"}
	args = append(args, "-w", workDir)

	for k, v := range opts.Env {
		args = append(args, "-e", k+"="+v)
	}

	nm := opts.NetworkMode
	if nm == "" {
		nm = "none"
	}
	args = append(args, "--network", nm)

	prefix := shInterpreter(lang)
	args = append(args, image)
	args = append(args, "/bin/sh")
	if prefix != "" {
		args = append(args, "-c", prefix+"\n"+wrapScript)
	} else {
		args = append(args, "-c", wrapScript)
	}

	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, p.config.DockerCmd, args...) //nolint:gosec
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start)

	var result ExecutionResult
	result.Duration = duration
	result.Stdout = strings.TrimSpace(stdout.String())
	result.Stderr = strings.TrimSpace(stderr.String())

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			result.TimedOut = true
			result.ExitCode = -1
		} else if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			return result, fmt.Errorf("docker exec: %w", err)
		}
	}

	p.recordResult(result)
	return result, nil
}

func shInterpreter(lang Language) string {
	switch lang {
	case LanguageGo:
		return "cat > main.go && go run main.go"
	case LanguagePython:
		return "cat > main.py && python3 main.py"
	case LanguageNode:
		return "cat > main.js && node main.js"
	case LanguageRust:
		return "cat > main.rs && rustc main.rs -o /tmp/out && /tmp/out"
	case LanguageTypeScript:
		return "cat > main.ts && npx ts-node main.ts"
	case LanguageBash:
		return ""
	default:
		return ""
	}
}

// GetContainer creates a persistent session container.
func (p *Pool) GetContainer(ctx context.Context, sessionID string) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if cid, ok := p.containers[sessionID]; ok {
		return cid, nil
	}

	if len(p.containers) >= p.config.MaxContainers {
		return "", fmt.Errorf("pool exhausted: max %d containers", p.config.MaxContainers)
	}

	name := "dxrk-sandbox-" + sessionID
	args := make([]string, 0, 14)
	args = append(args, "run", "-d", "-i", "-t", "--name", name)
	args = append(args, "-w", defaultWorkDir)
	args = append(args, "--network", "none")
	args = append(args, p.config.DefaultImage)
	args = append(args, "sh", "-c", "while true; do sleep 3600; done")

	cmd := exec.CommandContext(ctx, p.config.DockerCmd, args...) //nolint:gosec
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("create session container: %s: %w", strings.TrimSpace(stderr.String()), err)
	}

	containerID := strings.TrimSpace(string(out))
	p.containers[sessionID] = containerID
	p.stats.TotalContainers++
	p.stats.ActiveContainers++

	return containerID, nil
}

// ExecInSession runs a command in an existing session container.
func (p *Pool) ExecInSession(ctx context.Context, sessionID, workDir string, cmdArgs []string) (ExecutionResult, error) {
	p.mu.Lock()
	containerID, ok := p.containers[sessionID]
	p.mu.Unlock()

	if !ok {
		return ExecutionResult{}, fmt.Errorf("session not found: %s", sessionID)
	}

	args := []string{"exec", "-i"}
	if workDir != "" {
		args = append(args, "-w", sanitizePath(workDir))
	}
	args = append(args, containerID)
	args = append(args, cmdArgs...)

	cmd := exec.CommandContext(ctx, p.config.DockerCmd, args...) //nolint:gosec
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start)

	var result ExecutionResult
	result.Duration = duration
	result.Stdout = strings.TrimSpace(stdout.String())
	result.Stderr = strings.TrimSpace(stderr.String())

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		}
	} else {
		result.ExitCode = 0
	}

	return result, nil
}

// ReleaseContainer stops and removes a session container.
func (p *Pool) ReleaseContainer(ctx context.Context, sessionID string) error {
	p.mu.Lock()
	containerID, ok := p.containers[sessionID]
	if ok {
		delete(p.containers, sessionID)
	}
	p.mu.Unlock()

	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	_ = exec.CommandContext(ctx, p.config.DockerCmd, "rm", "-f", containerID).Run() //nolint:gosec
	return nil
}

// Stats returns pool statistics.
func (p *Pool) Stats() PoolStats {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.stats
}

// Close shuts down the pool.
func (p *Pool) Close() error {
	close(p.stopCh)
	p.idleTicker.Stop()
	p.wg.Wait()

	ctx := context.Background()
	p.mu.Lock()
	defer p.mu.Unlock()

	for sessionID, containerID := range p.containers {
		_ = exec.CommandContext(ctx, p.config.DockerCmd, "rm", "-f", containerID).Run() //nolint:gosec
		delete(p.containers, sessionID)
	}
	return nil
}

func (p *Pool) recordResult(result ExecutionResult) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stats.TotalExecutions++
	if result.ExitCode != 0 || result.TimedOut {
		p.stats.FailedExecutions++
	}
}

func (p *Pool) idleReaper() {
	defer p.wg.Done()
	for {
		select {
		case <-p.idleTicker.C:
			p.reapIdle()
		case <-p.stopCh:
			return
		}
	}
}

func (p *Pool) reapIdle() {
	p.mu.Lock()
	defer p.mu.Unlock()

	ctx := context.Background()
	for sessionID, containerID := range p.containers {
		_ = exec.CommandContext(ctx, p.config.DockerCmd, "rm", "-f", containerID).Run() //nolint:gosec
		delete(p.containers, sessionID)
		p.stats.TotalContainers--
		p.stats.ActiveContainers--
	}
}
