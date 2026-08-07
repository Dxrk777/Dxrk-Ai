// SPDX-License-Identifier: MIT
// Package bridge provides a WebSocket bridge for remote control sessions,
// based on Claude Code CLI's bridge architecture.
package bridge

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/Dxrk777/Dxrk/internal/strconst"
)

// SpawnMode controls how sessions are isolated.
type SpawnMode string

const (
	SpawnSingleSession SpawnMode = "single-session"
	SpawnWorktree      SpawnMode = "worktree"
	SpawnSameDir       SpawnMode = "same-dir"
)

// SessionStatus represents the state of a bridge session.
type SessionStatus string

const (
	StatusPending SessionStatus = strconst.StrPending
	StatusRunning SessionStatus = strconst.StrRunning
	StatusDone    SessionStatus = "done"
	StatusFailed  SessionStatus = strconst.StrFailed
	StatusKilled  SessionStatus = "killed"
)

// SessionDoneStatus is the final status of a completed session.
type SessionDoneStatus string

const (
	DoneCompleted   SessionDoneStatus = strconst.StrCompleted
	DoneFailed      SessionDoneStatus = strconst.StrFailed
	DoneInterrupted SessionDoneStatus = "interrupted"
)

// SessionActivityType describes a session activity event.
type SessionActivityType string

const (
	ActivityToolStart SessionActivityType = "tool_start"
	ActivityText      SessionActivityType = "text"
	ActivityResult    SessionActivityType = strconst.StrResult
	ActivityError     SessionActivityType = strconst.StrError
)

// BridgeConfig holds configuration for a bridge instance.
type BridgeConfig struct {
	Dir                string    `json:"dir"`
	MachineName        string    `json:"machine_name"`
	Branch             string    `json:"branch"`
	GitRepoURL         string    `json:"git_repo_url,omitempty"`
	MaxSessions        int       `json:"max_sessions"`
	SpawnMode          SpawnMode `json:"spawn_mode"`
	Verbose            bool      `json:"verbose"`
	Sandbox            bool      `json:"sandbox"`
	BridgeID           string    `json:"bridge_id"`
	WorkerType         string    `json:"worker_type"`
	EnvironmentID      string    `json:"environment_id"`
	ReuseEnvironmentID string    `json:"reuse_environment_id,omitempty"`
	APIBaseURL         string    `json:"api_base_url"`
	SessionIngressURL  string    `json:"session_ingress_url"`
	SessionTimeoutMs   int64     `json:"session_timeout_ms,omitempty"`
}

// SessionActivity records a recent activity in a session.
type SessionActivity struct {
	Type      SessionActivityType `json:"type"`
	Summary   string              `json:"summary"`
	Timestamp int64               `json:"timestamp"`
}

// SessionHandle represents a running session.
type SessionHandle struct {
	ID              string            `json:"id"`
	Status          SessionStatus     `json:"status"`
	Dir             string            `json:"dir"`
	CreatedAt       time.Time         `json:"created_at"`
	StartedAt       *time.Time        `json:"started_at,omitempty"`
	DoneAt          *time.Time        `json:"done_at,omitempty"`
	Duration        time.Duration     `json:"duration"`
	Activities      []SessionActivity `json:"activities"`
	CurrentActivity *SessionActivity  `json:"current_activity,omitempty"`
	AccessToken     string            `json:"access_token"`
	Stderr          []string          `json:"stderr,omitempty"`
	Error           string            `json:"error,omitempty"`
}

// WorkData represents incoming work from the polling API.
type WorkData struct {
	Type string `json:"type"` // "session" or "healthcheck"
	ID   string `json:"id"`
}

// WorkResponse is the server response to a poll.
type WorkResponse struct {
	ID            string    `json:"id"`
	Type          string    `json:"type"`
	EnvironmentID string    `json:"environment_id"`
	State         string    `json:"state"`
	Data          WorkData  `json:"data"`
	Secret        string    `json:"secret"`
	CreatedAt     time.Time `json:"created_at"`
}

// WorkSecret is the decoded session secret.
type WorkSecret struct {
	Version             int               `json:"version"`
	SessionIngressToken string            `json:"session_ingress_token"`
	APIBaseURL          string            `json:"api_base_url"`
	Sources             []WorkSource      `json:"sources"`
	Auth                []WorkAuth        `json:"auth"`
	ClaudeCodeArgs      map[string]string `json:"claude_code_args,omitempty"`
	MCPConfig           interface{}       `json:"mcp_config,omitempty"`
	EnvironmentVars     map[string]string `json:"environment_variables,omitempty"`
	UseCodeSessions     bool              `json:"use_code_sessions,omitempty"`
}

type WorkSource struct {
	Type    string   `json:"type"`
	GitInfo *GitInfo `json:"git_info,omitempty"`
}

type GitInfo struct {
	Type  string `json:"type"`
	Repo  string `json:"repo"`
	Ref   string `json:"ref,omitempty"`
	Token string `json:"token,omitempty"`
}

type WorkAuth struct {
	Type  string `json:"type"`
	Token string `json:"token"`
}

// Bridge represents a remote control bridge server.
type Bridge struct {
	mu       sync.RWMutex
	config   BridgeConfig
	sessions map[string]*SessionHandle
	active   int
	done     chan struct{}
}

// NewBridge creates a new bridge with the given config.
func NewBridge(config BridgeConfig) *Bridge {
	if config.BridgeID == "" {
		config.BridgeID = generateID()
	}
	if config.EnvironmentID == "" {
		config.EnvironmentID = generateID()
	}
	if config.MaxSessions <= 0 {
		config.MaxSessions = 1
	}
	if config.SessionTimeoutMs <= 0 {
		config.SessionTimeoutMs = 24 * 60 * 60 * 1000 // 24 hours
	}

	return &Bridge{
		config:   config,
		sessions: make(map[string]*SessionHandle),
		done:     make(chan struct{}),
	}
}

// Config returns the bridge configuration.
func (b *Bridge) Config() BridgeConfig {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.config
}

// SpawnSession creates a new session handle.
func (b *Bridge) SpawnSession(sessionID string) (*SessionHandle, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.active >= b.config.MaxSessions {
		return nil, fmt.Errorf("max sessions (%d) reached", b.config.MaxSessions)
	}

	now := time.Now()
	handle := &SessionHandle{
		ID:          sessionID,
		Status:      StatusPending,
		Dir:         b.config.Dir,
		CreatedAt:   now,
		AccessToken: generateToken(),
		Activities:  make([]SessionActivity, 0, 10),
	}

	b.sessions[sessionID] = handle
	b.active++

	return handle, nil
}

// StartSession marks a session as running.
func (b *Bridge) StartSession(sessionID string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	session, ok := b.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session %s not found", sessionID)
	}

	now := time.Now()
	session.Status = StatusRunning
	session.StartedAt = &now
	return nil
}

// CompleteSession marks a session as done.
func (b *Bridge) CompleteSession(sessionID string, status SessionDoneStatus) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	session, ok := b.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session %s not found", sessionID)
	}

	now := time.Now()
	session.DoneAt = &now
	session.Duration = now.Sub(session.CreatedAt)

	switch status {
	case DoneCompleted:
		session.Status = StatusDone
	case DoneFailed:
		session.Status = StatusFailed
	case DoneInterrupted:
		session.Status = StatusKilled
	}

	b.active--
	return nil
}

// KillSession force-kills a session.
func (b *Bridge) KillSession(sessionID string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	session, ok := b.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session %s not found", sessionID)
	}

	now := time.Now()
	session.Status = StatusKilled
	session.DoneAt = &now
	session.Duration = now.Sub(session.CreatedAt)
	b.active--
	return nil
}

// GetSession returns a session by ID.
func (b *Bridge) GetSession(sessionID string) (*SessionHandle, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	session, ok := b.sessions[sessionID]
	return session, ok
}

// ActiveSessions returns all running sessions.
func (b *Bridge) ActiveSessions() []*SessionHandle {
	b.mu.RLock()
	defer b.mu.RUnlock()

	var active []*SessionHandle
	for _, s := range b.sessions {
		if s.Status == StatusRunning || s.Status == StatusPending {
			active = append(active, s)
		}
	}
	return active
}

// AddActivity records an activity on a session.
func (b *Bridge) AddActivity(sessionID string, actType SessionActivityType, summary string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	session, ok := b.sessions[sessionID]
	if !ok {
		return
	}

	activity := SessionActivity{
		Type:      actType,
		Summary:   summary,
		Timestamp: time.Now().UnixMilli(),
	}

	session.CurrentActivity = &activity
	session.Activities = append(session.Activities, activity)

	// Keep ring buffer of last 10.
	if len(session.Activities) > 10 {
		session.Activities = session.Activities[len(session.Activities)-10:]
	}
}

// ArchiveSession removes a session from the active list.
func (b *Bridge) ArchiveSession(sessionID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.sessions, sessionID)
}

// IsAtCapacity returns true if the bridge cannot accept more sessions.
func (b *Bridge) IsAtCapacity() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.active >= b.config.MaxSessions
}

// SessionCount returns the number of active sessions.
func (b *Bridge) SessionCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.active
}

func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func generateToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// MarshalJSON custom JSON serialization for Bridge.
func (b *Bridge) MarshalJSON() ([]byte, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return json.Marshal(struct {
		Config   BridgeConfig              `json:"config"`
		Sessions map[string]*SessionHandle `json:"sessions"`
		Active   int                       `json:"active"`
	}{
		Config:   b.config,
		Sessions: b.sessions,
		Active:   b.active,
	})
}
