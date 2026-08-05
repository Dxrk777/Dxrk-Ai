package hooks

import (
	"encoding/json"
	"time"

	"github.com/Dxrk777/Dxrk-Ai/internal/strconst"
)

// HookType represents the type of hook event.
type HookType int

const (
	// PreToolUse runs before a tool executes.
	PreToolUse HookType = iota
	// PostToolUse runs after a tool completes.
	PostToolUse
	// UserPromptSubmit runs when user submits a prompt.
	UserPromptSubmit
	// Notification runs on system notifications.
	Notification
	// Stop runs when agent stops.
	Stop
	// SubagentStop runs when a subagent stops.
	SubagentStop
)

func (ht HookType) String() string {
	names := [...]string{
		"pre_tool_use",
		"post_tool_use",
		"user_prompt_submit",
		"notification",
		"stop",
		"subagent_stop",
	}
	if int(ht) < len(names) {
		return names[ht]
	}
	return strconst.StrUnknown
}

func ParseHookType(s string) (HookType, bool) {
	switch s {
	case "pre_tool_use":
		return PreToolUse, true
	case "post_tool_use":
		return PostToolUse, true
	case "user_prompt_submit":
		return UserPromptSubmit, true
	case "notification":
		return Notification, true
	case "stop":
		return Stop, true
	case "subagent_stop":
		return SubagentStop, true
	}
	return 0, false
}

// HookEvent represents a hook trigger event with context.
type HookEvent struct {
	Type       HookType          `json:"type"`
	ToolName   string            `json:"tool_name,omitempty"`
	ToolInput  json.RawMessage   `json:"tool_input,omitempty"`
	ToolOutput json.RawMessage   `json:"tool_output,omitempty"`
	Prompt     string            `json:"prompt,omitempty"`
	Message    string            `json:"message,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
	Timestamp  time.Time         `json:"timestamp"`
	SessionID  string            `json:"session_id,omitempty"`
}

// HookMatch defines pattern matching criteria for a hook.
type HookMatch struct {
	ToolName  string   `json:"tool_name,omitempty"`
	ToolNames []string `json:"tool_names,omitempty"`
	Path      string   `json:"path,omitempty"`
	Paths     []string `json:"paths,omitempty"`
	Glob      string   `json:"glob,omitempty"`
	Regex     string   `json:"regex,omitempty"`
	Command   string   `json:"command,omitempty"`
	Commands  []string `json:"commands,omitempty"`
}

// HookResult represents the outcome of hook execution.
type HookResult struct {
	Success       bool              `json:"success"`
	ExitCode      int               `json:"exit_code,omitempty"`
	Stdout        string            `json:"stdout,omitempty"`
	Stderr        string            `json:"stderr,omitempty"`
	Error         string            `json:"error,omitempty"`
	Duration      time.Duration     `json:"duration"`
	ModifiedInput json.RawMessage   `json:"modified_input,omitempty"`
	SkipTool      bool              `json:"skip_tool,omitempty"`
	AbortReason   string            `json:"abort_reason,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

// HookConfig holds the configuration for a single hook.
type HookConfig struct {
	ID          string        `json:"id"`
	Type        HookType      `json:"type"`
	Match       HookMatch     `json:"match"`
	Command     string        `json:"command"`
	Args        []string      `json:"args,omitempty"`
	Env         []string      `json:"env,omitempty"`
	Timeout     time.Duration `json:"timeout,omitempty"`
	MaxRetries  int           `json:"max_retries,omitempty"`
	RetryDelay  time.Duration `json:"retry_delay,omitempty"`
	Enabled     bool          `json:"enabled"`
	Description string        `json:"description,omitempty"`
	Priority    int           `json:"priority,omitempty"`
}

// HookExecutionContext provides runtime context for hook execution.
type HookExecutionContext struct {
	Event     HookEvent
	Config    HookConfig
	StartTime time.Time
	Attempt   int
}
