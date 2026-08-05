// SPDX-License-Identifier: MIT
package query

import (
	"context"
	"fmt"
	"os/exec"
	"sync"
	"time"

	"github.com/Dxrk777/Dxrk-Ai/internal/mcp"
	"github.com/Dxrk777/Dxrk-Ai/internal/strconst"
)

const (
	scopeProject   = strconst.StrProject
	keyModel       = "model"
	keyFunction    = "function"
	keyType        = "type"
	valObject      = strconst.StrObject
	keyProperties  = strconst.StrProperties
	keyParameters  = "parameters"
	keyDescription = strconst.StrDescription
)

// lookPathFn is mockable for tests (similar to osStatPathFn in tui/model.go).
var lookPathFn = exec.LookPath

// Persistence defines the interface for cross-session persistence.
// Implementations save and retrieve conversation context across sessions.
type Persistence interface {
	SaveTurn(sessionID, userMsg, assistantMsg string) error
	GetProjectContext(project string) (string, error)
	Search(query, project string) ([]SearchResult, error)
	Close() error
}

// SearchResult is a memory search hit.
type SearchResult struct {
	Title   string
	Content string
	Type    string
	Score   float64
}

// DxrkMemoryBackend persists conversation state via Engram MCP.
type DxrkMemoryBackend struct {
	client  *mcp.Client
	closeMu sync.Mutex
	closed  bool
}

// NewDxrkMemoryBackend starts an dxrk-memory MCP subprocess and returns a backend.
// If the dxrk-memory binary is not found, returns nil, nil (graceful degradation).
func NewDxrkMemoryBackend(ctx context.Context) (*DxrkMemoryBackend, error) {
	path, err := lookPathFn("dxrk-memory")
	if err != nil {
		return nil, nil //nolint:nilerr
	}

	transport, err := mcp.NewStdioTransport(ctx, path, "mcp", "--tools=agent")
	if err != nil {
		return nil, fmt.Errorf("start dxrk-memory mcp: %w", err)
	}

	client := mcp.NewClient(transport)
	if _, err := client.Initialize(ctx); err != nil {
		_ = transport.Close()
		return nil, fmt.Errorf("initialize dxrk-memory mcp: %w", err)
	}

	return &DxrkMemoryBackend{client: client}, nil
}

// SaveTurn persists a user-assistant exchange as a memory observation.
func (e *DxrkMemoryBackend) SaveTurn(sessionID, userMsg, assistantMsg string) error {
	if e == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	title := userMsg
	if len(title) > 80 {
		title = title[:80]
	}

	content := fmt.Sprintf("**User**: %s\n\n**Assistant**: %s", userMsg, assistantMsg)

	params := map[string]any{
		"memory":          content,
		"type":            "manual",
		strconst.StrTitle: title,
		"scope":           scopeProject,
	}
	if sessionID != "" {
		params["session_id"] = sessionID
	}
	if _, err := e.client.CallTool(ctx, "mem_save", params); err != nil {
		return fmt.Errorf("mem_save: %w", err)
	}
	return nil
}

// GetProjectContext retrieves the formatted context for a project from DxrkMemory.
func (e *DxrkMemoryBackend) GetProjectContext(project string) (string, error) {
	if e == nil {
		return "", nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	params := map[string]any{
		strconst.StrProject: project,
	}
	result, err := e.client.CallTool(ctx, "mem_context", params)
	if err != nil {
		return "", fmt.Errorf("mem_context: %w", err)
	}
	return parseTextContent(result), nil
}

// Search queries DxrkMemory's FTS5 index for relevant memories.
func (e *DxrkMemoryBackend) Search(query, project string) ([]SearchResult, error) {
	if e == nil {
		return nil, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	params := map[string]any{
		strconst.StrQuery:   query,
		strconst.StrProject: project,
	}
	result, err := e.client.CallTool(ctx, "mem_search", params)
	if err != nil {
		return nil, fmt.Errorf("mem_search: %w", err)
	}

	text := parseTextContent(result)
	if text == "" {
		return nil, nil
	}

	return []SearchResult{{
		Title:   "Dxrk Memory",
		Content: text,
		Type:    "memory",
	}}, nil
}

// Close shuts down the dxrk-memory subprocess.
func (e *DxrkMemoryBackend) Close() error {
	if e == nil {
		return nil
	}
	e.closeMu.Lock()
	defer e.closeMu.Unlock()
	if e.closed {
		return nil
	}
	e.closed = true
	return e.client.Close()
}

// parseTextContent extracts text content from a CallToolResult.
func parseTextContent(result *mcp.CallToolResult) string {
	if result == nil {
		return ""
	}
	for _, c := range result.Content {
		if c.Text != "" {
			return c.Text
		}
	}
	return ""
}

// Compile-time check: *DxrkMemoryBackend implements Persistence.
var _ Persistence = (*DxrkMemoryBackend)(nil)
