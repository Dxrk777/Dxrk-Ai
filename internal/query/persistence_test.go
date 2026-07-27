// SPDX-License-Identifier: MIT
package query

import (
	"errors"
	"sync"
	"testing"
)

type mockPersistence struct {
	mu          sync.Mutex
	turns       []turnRecord
	projectCtx  string
	searchRes   []SearchResult
	saveErr     error
	contextErr  error
	searchErr   error
	closeCalled bool
}

type turnRecord struct {
	sessionID    string
	userMsg      string
	assistantMsg string
}

func (m *mockPersistence) SaveTurn(sessionID, userMsg, assistantMsg string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.saveErr != nil {
		return m.saveErr
	}
	m.turns = append(m.turns, turnRecord{sessionID, userMsg, assistantMsg})
	return nil
}

func (m *mockPersistence) GetProjectContext(project string) (string, error) {
	if m.contextErr != nil {
		return "", m.contextErr
	}
	return m.projectCtx, nil
}

func (m *mockPersistence) Search(query, project string) ([]SearchResult, error) {
	if m.searchErr != nil {
		return nil, m.searchErr
	}
	return m.searchRes, nil
}

func (m *mockPersistence) Close() error {
	m.closeCalled = true
	return nil
}

func TestMockPersistence_SaveTurn(t *testing.T) {
	m := &mockPersistence{}

	err := m.SaveTurn("session-1", "Hello", "Hi there!")
	if err != nil {
		t.Fatalf("SaveTurn() error = %v", err)
	}

	m.mu.Lock()
	if len(m.turns) != 1 {
		m.mu.Unlock()
		t.Fatalf("len(turns) = %d, want 1", len(m.turns))
	}
	if m.turns[0].sessionID != "session-1" {
		m.mu.Unlock()
		t.Fatalf("sessionID = %q, want %q", m.turns[0].sessionID, "session-1")
	}
	if m.turns[0].userMsg != "Hello" {
		m.mu.Unlock()
		t.Fatalf("userMsg = %q, want %q", m.turns[0].userMsg, "Hello")
	}
	if m.turns[0].assistantMsg != "Hi there!" {
		m.mu.Unlock()
		t.Fatalf("assistantMsg = %q, want %q", m.turns[0].assistantMsg, "Hi there!")
	}
	m.mu.Unlock()

	err = m.SaveTurn("session-2", "How are you?", "I'm fine!")
	if err != nil {
		t.Fatalf("SaveTurn() error = %v", err)
	}

	m.mu.Lock()
	if len(m.turns) != 2 {
		m.mu.Unlock()
		t.Fatalf("len(turns) = %d, want 2", len(m.turns))
	}
	m.mu.Unlock()
}

func TestMockPersistence_SaveTurnError(t *testing.T) {
	expectedErr := errors.New("save failed")
	m := &mockPersistence{saveErr: expectedErr}

	err := m.SaveTurn("s1", "user", "assistant")
	if err != expectedErr {
		t.Fatalf("SaveTurn() error = %v, want %v", err, expectedErr)
	}
}

func TestMockPersistence_GetProjectContext(t *testing.T) {
	m := &mockPersistence{projectCtx: "project context"}

	ctx, err := m.GetProjectContext("my-project")
	if err != nil {
		t.Fatalf("GetProjectContext() error = %v", err)
	}
	if ctx != "project context" {
		t.Fatalf("context = %q, want %q", ctx, "project context")
	}
}

func TestMockPersistence_GetProjectContextError(t *testing.T) {
	expectedErr := errors.New("context error")
	m := &mockPersistence{contextErr: expectedErr}

	_, err := m.GetProjectContext("my-project")
	if err != expectedErr {
		t.Fatalf("GetProjectContext() error = %v, want %v", err, expectedErr)
	}
}

func TestMockPersistence_Search(t *testing.T) {
	m := &mockPersistence{
		searchRes: []SearchResult{
			{Title: "result 1", Content: "content 1", Type: "memory", Score: 0.95},
			{Title: "result 2", Content: "content 2", Type: "memory", Score: 0.85},
		},
	}

	results, err := m.Search("query", "project")
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}
	if results[0].Title != "result 1" {
		t.Fatalf("Title[0] = %q, want %q", results[0].Title, "result 1")
	}
	if results[1].Content != "content 2" {
		t.Fatalf("Content[1] = %q, want %q", results[1].Content, "content 2")
	}
}

func TestMockPersistence_SearchError(t *testing.T) {
	expectedErr := errors.New("search error")
	m := &mockPersistence{searchErr: expectedErr}

	_, err := m.Search("query", "project")
	if err != expectedErr {
		t.Fatalf("Search() error = %v, want %v", err, expectedErr)
	}
}

func TestMockPersistence_Close(t *testing.T) {
	m := &mockPersistence{}
	if err := m.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if !m.closeCalled {
		t.Fatal("Close() was not called")
	}
}

func TestMockPersistence_EmptyResults(t *testing.T) {
	m := &mockPersistence{}

	results, err := m.Search("query", "project")
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if results != nil {
		t.Fatalf("results = %v, want nil", results)
	}

	ctx, err := m.GetProjectContext("p")
	if err != nil {
		t.Fatalf("GetProjectContext() error = %v", err)
	}
	if ctx != "" {
		t.Fatalf("context = %q, want empty", ctx)
	}
}

func TestPersistenceInterfaceCompileTime(t *testing.T) {
	var _ Persistence = (*mockPersistence)(nil)
}

func TestDxrkMemoryBackendNilSafety(t *testing.T) {
	var e *DxrkMemoryBackend

	err := e.SaveTurn("s1", "user", "asst")
	if err != nil {
		t.Fatalf("SaveTurn on nil backend error = %v", err)
	}

	ctx, err := e.GetProjectContext("p")
	if err != nil {
		t.Fatalf("GetProjectContext on nil backend error = %v", err)
	}
	if ctx != "" {
		t.Fatalf("context = %q, want empty", ctx)
	}

	results, err := e.Search("q", "p")
	if err != nil {
		t.Fatalf("Search on nil backend error = %v", err)
	}
	if results != nil {
		t.Fatalf("results = %v, want nil", results)
	}

	err = e.Close()
	if err != nil {
		t.Fatalf("Close on nil backend error = %v", err)
	}
}

func TestDxrkMemoryBackendClosedIdempotent(t *testing.T) {
	e := &DxrkMemoryBackend{closed: true}

	err := e.Close()
	if err != nil {
		t.Fatalf("Close on already closed backend error = %v", err)
	}
}
