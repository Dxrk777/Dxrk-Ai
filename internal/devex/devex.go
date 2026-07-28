// SPDX-License-Identifier: MIT
package devex

import (
	"context"
	"fmt"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/Dxrk777/Dxrk-Ai/internal/log"
)

type Timer struct {
	name     string
	start    time.Time
	stopped  bool
	duration time.Duration
}

func (t *Timer) Stop() time.Duration {
	if t.stopped {
		return t.duration
	}
	t.stopped = true
	t.duration = time.Since(t.start)
	return t.duration
}

func (t *Timer) String() string {
	d := t.duration
	if !t.stopped {
		d = time.Since(t.start)
	}
	return fmt.Sprintf("%s took %s", t.name, d.Round(time.Millisecond))
}

type Manager struct {
	mu        sync.Mutex
	logger    log.Logger
	analytics *Analytics
	timers    []*Timer
}

func New(logger log.Logger, analytics *Analytics) *Manager {
	if logger == nil {
		logger = log.NewNop()
	}
	if analytics == nil {
		analytics = NewAnalytics(make(map[string]int))
	}
	return &Manager{
		logger:    logger,
		analytics: analytics,
	}
}

func (m *Manager) TrackEvent(event string, props map[string]string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.analytics.Increment(context.Background(), event)
	m.logger.Info("track", "event", event, "props", props)
}

func (m *Manager) StartTimer(name string) *Timer {
	t := &Timer{name: name, start: time.Now()}
	m.mu.Lock()
	m.timers = append(m.timers, t)
	m.mu.Unlock()
	m.logger.Debug("timer started", "name", name)
	return t
}

func (m *Manager) ShowTip(ctx context.Context) string {
	tips := []string{
		"Run `dxrk sdd init` to bootstrap Spec-Driven Development in any project",
		"Use `dxrk agent install --all` to install agents across all supported IDEs",
		"Dxrk Memory persists across sessions — run `dxrk dxrk-memory sync` to refresh",
		"SDD workflow: propose → spec → design → tasks → apply → verify → archive",
		"`dxrk query \"your question\"` asks all installed agents in parallel",
		"Enable telemetry with `dxrk telemetry enable` to track usage locally",
		"Backup agent configs with `dxrk backup create` before major upgrades",
		"`dxrk restore list` shows available snapshots for recovery",
		"Use `dxrk completion bash` to enable shell autocompletion",
		"Agent builder lets you scaffold custom agents: `dxrk agent-builder create`",
		"Set DXRK_DEFAULT_PROVIDER env to override the default LLM provider",
		"Run `dxrk chat` to enter interactive conversation mode",
		"Skills extend agent capabilities — browse them with `dxrk catalog skills`",
		"Dxrk supports Claude, GPT, Gemini, Ollama, and more providers",
		"Use `dxrk provider list` to see all configured LLM providers",
		"`dxrk sync` propagates agent configs across all installed IDEs",
		"Check version with `dxrk version` and update with `dxrk upgrade`",
		"MCP servers enable tool use — configure them in your agent config",
		"`dxrk validate` checks your project for compatibility issues",
		"Run `dxrk dryrun` before any destructive operation to preview changes",
	}

	m.analytics.Increment(ctx, "tip_shown")
	return tips[rand.IntN(len(tips))] //nolint:gosec // tips are not security-sensitive
}
