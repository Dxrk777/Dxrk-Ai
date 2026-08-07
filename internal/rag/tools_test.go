// SPDX-License-Identifier: MIT
package rag

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Dxrk777/Dxrk/internal/tools"
)

func TestRegisterTools(t *testing.T) {
	reg := tools.New()
	err := RegisterTools(reg)
	if err != nil {
		t.Fatal(err)
	}

	if reg.Len() != 2 {
		t.Fatalf("expected 2 tools registered, got %d", reg.Len())
	}

	tool1, ok := reg.Get("codebase_query")
	if !ok {
		t.Fatal("expected codebase_query to be registered")
	}
	if tool1.Name() != "codebase_query" {
		t.Errorf("expected name codebase_query, got %s", tool1.Name())
	}
	if !tool1.IsEnabled() {
		t.Error("expected tool to be enabled")
	}

	tool2, ok := reg.Get("codebase_index")
	if !ok {
		t.Fatal("expected codebase_index to be registered")
	}
	if tool2.Name() != "codebase_index" {
		t.Errorf("expected name codebase_index, got %s", tool2.Name())
	}
}

func TestGetRAGFromContext_NilContext(t *testing.T) {
	ctx := tools.Context{Context: nil}
	_, err := getRAGFromContext(ctx)
	if err == nil {
		t.Fatal("expected error for nil context")
	}
}

func TestGetRAGFromContext_NoRAG(t *testing.T) {
	ctx := tools.Context{Context: context.Background()}
	_, err := getRAGFromContext(ctx)
	if err == nil {
		t.Fatal("expected error when RAG not in context")
	}
}

func TestGetRAGFromContext_WrongType(t *testing.T) {
	baseCtx := context.WithValue(context.Background(), RAGContextKey{}, "not-a-rag")
	ctx := tools.Context{Context: baseCtx}
	_, err := getRAGFromContext(ctx)
	if err == nil {
		t.Fatal("expected error when wrong type in context")
	}
}

func TestGetRAGFromContext_Success(t *testing.T) {
	rag := &RAG{enabled: true}
	baseCtx := context.WithValue(context.Background(), RAGContextKey{}, rag)
	ctx := tools.Context{Context: baseCtx}

	result, err := getRAGFromContext(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result != rag {
		t.Error("expected same RAG instance")
	}
	if !result.IsEnabled() {
		t.Error("expected RAG to be enabled")
	}
}

func TestTool_CodebaseQuery_EmptyQuery(t *testing.T) {
	reg := tools.New()
	if err := RegisterTools(reg); err != nil {
		t.Fatal(err)
	}

	tool, _ := reg.Get("codebase_query")
	_, err := tool.Execute(tools.Context{Context: context.Background()}, map[string]any{
		"query": "",
	})
	if err == nil {
		t.Fatal("expected error for empty query")
	}
}

func TestTool_CodebaseQuery_NoRAGInContext(t *testing.T) {
	reg := tools.New()
	if err := RegisterTools(reg); err != nil {
		t.Fatal(err)
	}

	tool, _ := reg.Get("codebase_query")
	_, err := tool.Execute(tools.Context{Context: context.Background()}, map[string]any{
		"query": "test query",
	})
	if err == nil {
		t.Fatal("expected error when RAG not in context")
	}
}

func TestTool_CodebaseQuery_RAGDisabled(t *testing.T) {
	rag := &RAG{enabled: false}
	baseCtx := context.WithValue(context.Background(), RAGContextKey{}, rag)
	toolCtx := tools.Context{Context: baseCtx}

	reg := tools.New()
	if err := RegisterTools(reg); err != nil {
		t.Fatal(err)
	}

	tool, _ := reg.Get("codebase_query")
	result, err := tool.Execute(toolCtx, map[string]any{
		"query": "test query",
	})
	if err != nil {
		t.Fatal(err)
	}
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %T", result)
	}
	if enabled, _ := m["enabled"].(bool); enabled {
		t.Error("expected enabled=false")
	}
}

func TestTool_CodebaseQuery_DefaultMaxResults(t *testing.T) {
	store := NewVectorStore(3, "")
	embedder := &mockEmbedder{}
	idx := NewIndexer(store, embedder, t.TempDir(), DefaultChunkConfig())
	rag := &RAG{Indexer: idx, Store: store, Embedder: embedder, enabled: true}

	store.Insert([]VectorRecord{
		{ID: "1", Chunk: Chunk{Text: "alpha", FilePath: "a.go", StartLine: 1, EndLine: 2, Language: "go"}, Embedding: []float64{1, 0, 0}},
	})

	baseCtx := context.WithValue(context.Background(), RAGContextKey{}, rag)
	toolCtx := tools.Context{Context: baseCtx}

	reg := tools.New()
	if err := RegisterTools(reg); err != nil {
		t.Fatal(err)
	}

	tool, _ := reg.Get("codebase_query")
	result, err := tool.Execute(toolCtx, map[string]any{
		"query": "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %T", result)
	}
	if enabled, _ := m["enabled"].(bool); !enabled {
		t.Error("expected enabled=true")
	}
	total, _ := m["total"].(int)
	if total != 1 {
		t.Errorf("expected total=1, got %d", total)
	}
}

func TestTool_CodebaseIndex_NoRAG(t *testing.T) {
	reg := tools.New()
	if err := RegisterTools(reg); err != nil {
		t.Fatal(err)
	}

	tool, _ := reg.Get("codebase_index")
	_, err := tool.Execute(tools.Context{Context: context.Background()}, map[string]any{})
	if err == nil {
		t.Fatal("expected error when RAG not in context")
	}
}

func TestTool_CodebaseIndex_Disabled(t *testing.T) {
	rag := &RAG{enabled: false}
	baseCtx := context.WithValue(context.Background(), RAGContextKey{}, rag)
	toolCtx := tools.Context{Context: baseCtx}

	reg := tools.New()
	if err := RegisterTools(reg); err != nil {
		t.Fatal(err)
	}

	tool, _ := reg.Get("codebase_index")
	result, err := tool.Execute(toolCtx, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %T", result)
	}
	if enabled, _ := m["enabled"].(bool); enabled {
		t.Error("expected enabled=false")
	}
}

func TestTool_CodebaseIndex_PathOverride(t *testing.T) {
	rag := &RAG{enabled: true}
	baseCtx := context.WithValue(context.Background(), RAGContextKey{}, rag)
	toolCtx := tools.Context{Context: baseCtx}

	reg := tools.New()
	if err := RegisterTools(reg); err != nil {
		t.Fatal(err)
	}

	tool, _ := reg.Get("codebase_index")
	_, err := tool.Execute(toolCtx, map[string]any{
		"path": "/some/path",
	})
	if err == nil {
		t.Fatal("expected error for path override")
	}
}

func TestTool_CodebaseQuery_QueryError(t *testing.T) {
	dir := t.TempDir()
	if err := writeFile(filepath.Join(dir, "main.go"), "package main\nfunc main() {}\n"); err != nil {
		t.Fatal(err)
	}

	store := NewVectorStore(3, "")
	idx := NewIndexer(store, &errEmbedder{}, dir, DefaultChunkConfig())
	rag := &RAG{Indexer: idx, Store: store, Embedder: &errEmbedder{}, enabled: true}

	baseCtx := context.WithValue(context.Background(), RAGContextKey{}, rag)
	toolCtx := tools.Context{Context: baseCtx}

	reg := tools.New()
	if err := RegisterTools(reg); err != nil {
		t.Fatal(err)
	}

	tool, _ := reg.Get("codebase_query")
	_, err := tool.Execute(toolCtx, map[string]any{
		"query": "test",
	})
	if err == nil {
		t.Fatal("expected error when query fails")
	}
}

func TestTool_CodebaseQuery_EmptyResults(t *testing.T) {
	store := NewVectorStore(3, "")
	idx := NewIndexer(store, &mockEmbedder{}, t.TempDir(), DefaultChunkConfig())
	rag := &RAG{Indexer: idx, Store: store, Embedder: &mockEmbedder{}, enabled: true}

	baseCtx := context.WithValue(context.Background(), RAGContextKey{}, rag)
	toolCtx := tools.Context{Context: baseCtx}

	reg := tools.New()
	if err := RegisterTools(reg); err != nil {
		t.Fatal(err)
	}

	tool, _ := reg.Get("codebase_query")
	result, err := tool.Execute(toolCtx, map[string]any{
		"query": "nonexistent",
	})
	if err != nil {
		t.Fatal(err)
	}
	m := result.(map[string]any)
	if _, ok := m["message"]; !ok {
		t.Error("expected message in result for empty results")
	}
}

func TestTool_CodebaseIndex_IndexError(t *testing.T) {
	store := NewVectorStore(3, "")
	idx := NewIndexer(store, &mockEmbedder{}, "/nonexistent", DefaultChunkConfig())
	rag := &RAG{Indexer: idx, Store: store, Embedder: &mockEmbedder{}, enabled: true}

	baseCtx := context.WithValue(context.Background(), RAGContextKey{}, rag)
	toolCtx := tools.Context{Context: baseCtx}

	reg := tools.New()
	if err := RegisterTools(reg); err != nil {
		t.Fatal(err)
	}

	tool, _ := reg.Get("codebase_index")
	_, err := tool.Execute(toolCtx, map[string]any{})
	if err == nil {
		t.Fatal("expected error when index fails")
	}
}

func TestTool_CodebaseQuery_CustomMaxResults(t *testing.T) {
	store := NewVectorStore(3, "")
	store.Insert([]VectorRecord{
		{ID: "1", Chunk: Chunk{Text: "alpha", FilePath: "a.go", StartLine: 1, EndLine: 2, Language: "go"}, Embedding: []float64{1, 0, 0}},
		{ID: "2", Chunk: Chunk{Text: "beta", FilePath: "b.go", StartLine: 1, EndLine: 2, Language: "go"}, Embedding: []float64{0, 1, 0}},
		{ID: "3", Chunk: Chunk{Text: "gamma", FilePath: "c.go", StartLine: 1, EndLine: 2, Language: "go"}, Embedding: []float64{0, 0, 1}},
		{ID: "4", Chunk: Chunk{Text: "delta", FilePath: "d.go", StartLine: 1, EndLine: 2, Language: "go"}, Embedding: []float64{0.9, 0.1, 0}},
	})
	idx := NewIndexer(store, &mockEmbedder{}, t.TempDir(), DefaultChunkConfig())
	rag := &RAG{Indexer: idx, Store: store, Embedder: &mockEmbedder{}, enabled: true}

	baseCtx := context.WithValue(context.Background(), RAGContextKey{}, rag)
	toolCtx := tools.Context{Context: baseCtx}

	reg := tools.New()
	if err := RegisterTools(reg); err != nil {
		t.Fatal(err)
	}

	tool, _ := reg.Get("codebase_query")
	result, err := tool.Execute(toolCtx, map[string]any{
		"query":       "test",
		"max_results": 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	m := result.(map[string]any)
	if enabled, _ := m["enabled"].(bool); !enabled {
		t.Error("expected enabled=true")
	}
	total, _ := m["total"].(int)
	if total != 3 {
		t.Errorf("expected total=3 (capped by max_results), got %d", total)
	}
}

func TestTool_CodebaseIndex_Success(t *testing.T) {
	dir := t.TempDir()
	if err := writeFile(filepath.Join(dir, "main.go"), "package main\nfunc main() {}\n"); err != nil {
		t.Fatal(err)
	}

	store := NewVectorStore(3, "")
	embedder := &mockEmbedder{}
	idx := NewIndexer(store, embedder, dir, DefaultChunkConfig())
	rag := &RAG{Indexer: idx, Store: store, Embedder: embedder, enabled: true}

	baseCtx := context.WithValue(context.Background(), RAGContextKey{}, rag)
	toolCtx := tools.Context{Context: baseCtx}

	reg := tools.New()
	if err := RegisterTools(reg); err != nil {
		t.Fatal(err)
	}

	tool, _ := reg.Get("codebase_index")
	result, err := tool.Execute(toolCtx, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %T", result)
	}
	scanned, _ := m["files_scanned"].(int)
	if scanned != 1 {
		t.Errorf("expected 1 file scanned, got %d", scanned)
	}
	indexed, _ := m["files_indexed"].(int)
	if indexed != 1 {
		t.Errorf("expected 1 file indexed, got %d", indexed)
	}
}

// writeFile is a helper used by TestTool_CodebaseIndex_Success.
// Using the existing os.WriteFile via the tools package in tests is fine,
// but we define a local helper to keep the test self-contained.
func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o600)
}
