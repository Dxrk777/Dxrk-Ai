package hooks

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"
)

func TestHookRegistry_Register(t *testing.T) {
	reg := NewHookRegistry()
	config := HookConfig{
		ID:      "test-hook",
		Command: "echo",
		Args:    []string{"hello"},
		Match:   HookMatch{ToolNames: []string{"bash"}},
		Enabled: true,
	}

	err := reg.Register(config)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	hooks := reg.GetByType(PreToolUse)
	if len(hooks) != 1 {
		t.Errorf("Expected 1 hook, got %d", len(hooks))
	}
	if hooks[0] != "test-hook" {
		t.Errorf("Expected hook ID 'test-hook', got '%s'", hooks[0])
	}
}

func TestHookRegistry_Unregister(t *testing.T) {
	reg := NewHookRegistry()
	config := HookConfig{
		ID:      "test-hook",
		Command: "echo",
		Args:    []string{"hello"},
		Match:   HookMatch{ToolNames: []string{"bash"}},
		Enabled: true,
	}
	reg.Register(config)

	ok := reg.Unregister("test-hook")
	if !ok {
		t.Fatalf("Unregister failed")
	}

	hooks := reg.GetByType(PreToolUse)
	if len(hooks) != 0 {
		t.Errorf("Expected 0 hooks after unregister, got %d", len(hooks))
	}
}

func TestHookRegistry_Matcher(t *testing.T) {
	reg := NewHookRegistry()
	config := HookConfig{
		ID:      "glob-hook",
		Command: "echo",
		Args:    []string{"glob"},
		Match:   HookMatch{ToolNames: []string{"*"}, Paths: []string{"*.go"}},
		Enabled: true,
	}
	reg.Register(config)

	hooks := reg.GetByType(PreToolUse)
	if len(hooks) != 1 {
		t.Errorf("Expected 1 hook for file_edit, got %d", len(hooks))
	}
}

func TestHookExecutor_Execute(t *testing.T) {
	exec := NewHookExecutor(
		WithExecutorTimeout(5*time.Second),
		WithExecutorRetries(0),
		WithCircuitBreaker(NewCircuitBreaker(5, 2, 30*time.Second)),
	)

	event := HookEvent{
		Type:      PreToolUse,
		ToolName:  "bash",
		ToolInput: []byte(`{"command": "echo test"}`),
	}

	result := exec.Execute(context.Background(), HookConfig{
		ID:      "test",
		Command: "echo",
		Args:    []string{"hook-output"},
		Enabled: true,
	}, event)

	if !result.Success {
		t.Errorf("Expected success, got error: %s", result.Error)
	}
}

func TestHookMatcher_MatchToolName(t *testing.T) {
	matcher := NewHookMatcher()

	tests := []struct {
		name     string
		matcher  HookMatch
		toolName string
		want     bool
	}{
		{"exact tool match", HookMatch{ToolNames: []string{"bash"}}, "bash", true},
		{"multi tool match", HookMatch{ToolNames: []string{"bash", "file_read"}}, "file_read", true},
		{"no tool match", HookMatch{ToolNames: []string{"bash"}}, "file_read", false},
		{"single tool no match", HookMatch{ToolName: "bash"}, "file_read", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := matcher.MatchToolName(tc.matcher, tc.toolName)
			if got != tc.want {
				t.Errorf("MatchToolName() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestHookMatcher_MatchPath(t *testing.T) {
	matcher := NewHookMatcher()

	tests := []struct {
		name    string
		matcher HookMatch
		path    string
		want    bool
	}{
		{"glob path match", HookMatch{Glob: "*.go"}, "test.go", true},
		{"glob path no match", HookMatch{Glob: "*.go"}, "test.py", false},
		{"exact path match", HookMatch{Paths: []string{"main.go"}}, "main.go", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := matcher.MatchPath(tc.matcher, tc.path)
			if got != tc.want {
				t.Errorf("MatchPath() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestHookMatcher_MatchEvent(t *testing.T) {
	matcher := NewHookMatcher()

	tests := []struct {
		name    string
		matcher HookMatch
		event   HookEvent
		want    bool
	}{
		{"match all", HookMatch{ToolNames: []string{"bash"}}, HookEvent{Type: PreToolUse, ToolName: "bash"}, true},
		{"no match", HookMatch{ToolNames: []string{"bash"}}, HookEvent{Type: PreToolUse, ToolName: "file_read"}, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := matcher.MatchEvent(tc.matcher, tc.event)
			if got != tc.want {
				t.Errorf("MatchEvent() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestHookQueue_Submit(t *testing.T) {
	queue := NewHookQueue(
		WithQueueWorkers(2),
		WithQueueBuffer(10),
	)
	queue.Start()
	defer queue.Stop(context.Background())

	ctx := context.Background()
	result, err := queue.Submit(ctx, HookEvent{Type: PreToolUse, ToolName: "bash"}, HookConfig{
		ID:      "test",
		Command: "echo",
		Args:    []string{"hook"},
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("Submit failed: %v", err)
	}
	if !result.Success {
		t.Errorf("Expected success, got error: %s", result.Error)
	}

	stats := queue.Stats()
	if stats.Processed != 1 {
		t.Errorf("Expected 1 processed, got %d", stats.Processed)
	}
}

func TestHookQueue_SubmitAsync(t *testing.T) {
	queue := NewHookQueue(
		WithQueueWorkers(2),
		WithQueueBuffer(10),
	)
	queue.Start()
	defer queue.Stop(context.Background())

	err := queue.SubmitAsync(HookEvent{Type: PreToolUse, ToolName: "bash"}, HookConfig{
		ID:      "test-async",
		Command: "echo",
		Args:    []string{"hook"},
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("SubmitAsync failed: %v", err)
	}
}

func TestHookQueue_Stats(t *testing.T) {
	queue := NewHookQueue(
		WithQueueWorkers(2),
		WithQueueBuffer(10),
	)
	queue.Start()
	defer queue.Stop(context.Background())

	queue.Submit(context.Background(), HookEvent{Type: PreToolUse, ToolName: "bash"}, HookConfig{
		ID:      "test",
		Command: "echo",
		Enabled: true,
	})

	stats := queue.Stats()
	if stats.Workers != 2 {
		t.Errorf("Expected 2 workers, got %d", stats.Workers)
	}
	if stats.QueueCap != 10 {
		t.Errorf("Expected queue cap 10, got %d", stats.QueueCap)
	}
}

func TestCircuitBreaker(t *testing.T) {
	cb := NewCircuitBreaker(3, 2, time.Second)

	for i := 0; i < 3; i++ {
		cb.Execute(context.Background(), func(ctx context.Context) error {
			return errors.New("boom")
		})
	}

	if cb.State() != CircuitOpen {
		t.Errorf("Expected open circuit after 3 failures, got %v", cb.State())
	}

	time.Sleep(2 * time.Second)

	for i := 0; i < 2; i++ {
		if err := cb.Execute(context.Background(), func(ctx context.Context) error {
			return nil
		}); err != nil {
			t.Fatalf("Execute after timeout should succeed: %v", err)
		}
	}

	if cb.State() != CircuitClosed {
		t.Errorf("Expected closed circuit after successes, got %v", cb.State())
	}
}

func TestCircuitBreaker_Execute(t *testing.T) {
	cb := NewCircuitBreaker(3, 2, time.Second)

	err := cb.Execute(context.Background(), func(ctx context.Context) error {
		return nil
	})
	if err != nil {
		t.Errorf("Execute should succeed: %v", err)
	}

	for i := 0; i < 3; i++ {
		cb.Execute(context.Background(), func(ctx context.Context) error {
			return errors.New("boom")
		})
	}

	err = cb.Execute(context.Background(), func(ctx context.Context) error {
		return nil
	})
	if err != ErrCircuitOpen {
		t.Errorf("Execute should fail with ErrCircuitOpen when circuit is open, got %v", err)
	}
}

func TestHookLogger_Log(t *testing.T) {
	var buf bytes.Buffer
	logger := NewHookLogger(&buf, LogLevelDebug)

	event := HookEvent{
		Type:      PreToolUse,
		ToolName:  "bash",
		ToolInput: []byte(`{"command": "ls"}`),
	}

	logger.Log(HookLogEntry{
		Timestamp: time.Now(),
		Level:     LogLevelDebug,
		HookID:    "test-hook",
		HookType:  PreToolUse,
		Event:     event,
		Result:    HookResult{Stdout: "files", Duration: time.Millisecond},
	})

	recent := logger.RecentEntries(10)
	if len(recent) != 1 {
		t.Errorf("Expected 1 log entry, got %d", len(recent))
	}
	if recent[0].HookID != "test-hook" {
		t.Errorf("Expected HookID 'test-hook', got '%s'", recent[0].HookID)
	}
}

func TestHookLogger_RecentEntries(t *testing.T) {
	var buf bytes.Buffer
	logger := NewHookLogger(&buf, LogLevelInfo)

	for i := 0; i < 3; i++ {
		logger.Log(HookLogEntry{
			Timestamp: time.Now(),
			Level:     LogLevelInfo,
			HookID:    "test-hook",
			HookType:  PreToolUse,
			Result:    HookResult{Stdout: "ok", Duration: time.Millisecond},
		})
	}

	recent := logger.RecentEntries(3)
	if len(recent) != 3 {
		t.Errorf("Expected 3 entries, got %d", len(recent))
	}
}

func TestHookLogger_Metrics(t *testing.T) {
	var buf bytes.Buffer
	logger := NewHookLogger(&buf, LogLevelDebug)

	now := time.Now()
	logger.Log(HookLogEntry{Timestamp: now, Level: LogLevelDebug, HookID: "a", HookType: PreToolUse, Result: HookResult{Success: true, Duration: time.Millisecond}})
	logger.Log(HookLogEntry{Timestamp: now, Level: LogLevelDebug, HookID: "b", HookType: PostToolUse, Result: HookResult{Success: true, Duration: time.Millisecond}})
	logger.Log(HookLogEntry{Timestamp: now, Level: LogLevelDebug, HookID: "c", HookType: PreToolUse, Result: HookResult{Success: false, Error: "error", Duration: time.Millisecond}})

	metrics := logger.Metrics()
	if metrics.TotalExecutions != 3 {
		t.Errorf("Expected 3 total, got %d", metrics.TotalExecutions)
	}
	if metrics.SuccessCount != 2 {
		t.Errorf("Expected 2 success, got %d", metrics.SuccessCount)
	}
	if metrics.FailureCount != 1 {
		t.Errorf("Expected 1 failed, got %d", metrics.FailureCount)
	}
}
