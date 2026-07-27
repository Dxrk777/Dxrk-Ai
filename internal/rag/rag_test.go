// SPDX-License-Identifier: MIT
package rag

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsCodeFile(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"main.go", true},
		{"main.ts", true},
		{"app.tsx", true},
		{"index.js", true},
		{"component.jsx", true},
		{"main.py", true},
		{"lib.rs", true},
		{"main.java", true},
		{"main.c", true},
		{"main.h", true},
		{"main.cpp", true},
		{"main.cs", true},
		{"main.rb", true},
		{"main.php", true},
		{"main.swift", true},
		{"main.kt", true},
		{"main.sh", true},
		{"config.yaml", true},
		{"config.json", true},
		{"Cargo.toml", true},
		{"README.md", true},
		{"style.css", true},
		{"index.html", true},
		{"App.svelte", true},
		{"file.vue", true},
		{"image.png", false},
		{"file.pdf", false},
		{"data.csv", false},
		{"archive.zip", false},
		{"binary", false},
	}
	for _, tt := range tests {
		got := IsCodeFile(tt.path)
		if got != tt.want {
			t.Errorf("IsCodeFile(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestLanguageFromExt(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"main.go", "go"},
		{"main.ts", "typescript"},
		{"app.tsx", "typescript"},
		{"main.py", "python"},
		{"lib.rs", "rust"},
		{"main.java", "java"},
		{"main.c", "c"},
		{"main.cpp", "cpp"},
		{"main.sh", "shell"},
		{"config.yaml", "yaml"},
		{"data.json", "json"},
		{"README.md", "markdown"},
		{"unknown.xyz", "text"},
	}
	for _, tt := range tests {
		got := LanguageFromExt(tt.path)
		if got != tt.want {
			t.Errorf("LanguageFromExt(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestChunkFile_SmallFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.go")
	content := "package main\n\nfunc main() {\n\tprintln(\"hello\")\n}\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	chunks, err := ChunkFile(path, DefaultChunkConfig())
	if err != nil {
		t.Fatal(err)
	}

	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if chunks[0].Text != content {
		t.Errorf("chunk text mismatch")
	}
	if chunks[0].Language != "go" {
		t.Errorf("expected go language, got %s", chunks[0].Language)
	}
	if chunks[0].StartLine != 1 {
		t.Errorf("expected start 1, got %d", chunks[0].StartLine)
	}
	if chunks[0].EndLine != 6 {
		t.Errorf("expected end 6, got %d", chunks[0].EndLine)
	}
}

func TestChunkFile_LargeFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.py")

	var lines []string
	for i := 0; i < 100; i++ {
		lines = append(lines, "# line "+strings.Repeat("x", 10))
	}
	content := strings.Join(lines, "\n")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := ChunkConfig{ChunkSize: 30, ChunkOverlap: 5}
	chunks, err := ChunkFile(path, cfg)
	if err != nil {
		t.Fatal(err)
	}

	if len(chunks) < 3 {
		t.Fatalf("expected at least 3 chunks for 100 lines, got %d", len(chunks))
	}

	for i, c := range chunks {
		if c.FilePath != path {
			t.Errorf("chunk %d: wrong filepath", i)
		}
		if c.Language != "python" {
			t.Errorf("chunk %d: expected python", i)
		}
	}
}

func TestChunkFile_NonUTF8(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "binary.bin")
	data := []byte{0xFF, 0xFE, 0x00, 0x01}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	chunks, err := ChunkFile(path, DefaultChunkConfig())
	if err != nil {
		t.Fatal(err)
	}
	if chunks != nil {
		t.Errorf("expected nil for non-utf8, got %d chunks", len(chunks))
	}
}

func TestChunkFile_NonCodeFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "image.png")
	if err := os.WriteFile(path, []byte("fake png"), 0o600); err != nil {
		t.Fatal(err)
	}

	// IsCodeFile returns false for .png but ChunkFile still tries to read it.
	// It will return a text chunk since it's valid UTF-8.
	chunks, err := ChunkFile(path, DefaultChunkConfig())
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if chunks[0].Language != "text" {
		t.Errorf("expected language 'text', got %s", chunks[0].Language)
	}
}

func TestVectorStore_InsertAndSearch(t *testing.T) {
	store := NewVectorStore(3, "")

	store.Insert([]VectorRecord{
		{ID: "1", Chunk: Chunk{Text: "vector search in go"}, Embedding: []float64{1, 0, 0}},
		{ID: "2", Chunk: Chunk{Text: "golang concurrency patterns"}, Embedding: []float64{0, 1, 0}},
		{ID: "3", Chunk: Chunk{Text: "machine learning in python"}, Embedding: []float64{0, 0, 1}},
	})

	if store.Len() != 3 {
		t.Fatalf("expected 3 records, got %d", store.Len())
	}

	// Search for something close to record 1
	results := store.Search([]float64{0.9, 0.1, 0}, 2)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Record.ID != "1" {
		t.Errorf("expected first result to be ID 1, got %s", results[0].Record.ID)
	}

	// Test empty search
	empty := store.Search([]float64{0, 0, 0}, 5)
	if len(empty) != 3 {
		t.Errorf("expected 3 results for all-zero query, got %d", len(empty))
	}
}

func TestVectorStore_Delete(t *testing.T) {
	store := NewVectorStore(2, "")
	store.Insert([]VectorRecord{
		{ID: "a", Chunk: Chunk{Text: "hello"}, Embedding: []float64{1, 0}},
	})
	if store.Len() != 1 {
		t.Fatal("expected 1 record after insert")
	}

	store.Delete("a")
	if store.Len() != 0 {
		t.Fatal("expected 0 records after delete")
	}
}

func TestVectorStore_Clear(t *testing.T) {
	store := NewVectorStore(2, "")
	store.Insert([]VectorRecord{
		{ID: "a", Chunk: Chunk{Text: "hello"}, Embedding: []float64{1, 0}},
		{ID: "b", Chunk: Chunk{Text: "world"}, Embedding: []float64{0, 1}},
	})
	store.Clear()
	if store.Len() != 0 {
		t.Fatal("expected 0 records after clear")
	}
}

func TestVectorStore_Persistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "vectors.json")

	store := NewVectorStore(3, path)
	store.Insert([]VectorRecord{
		{ID: "1", Chunk: Chunk{Text: "hello"}, Embedding: []float64{1, 0, 0}},
	})

	// Re-create store from same path
	store2 := NewVectorStore(3, path)
	if store2.Len() != 1 {
		t.Fatalf("expected 1 record from persistence, got %d", store2.Len())
	}
}

func TestCosineSimilarity(t *testing.T) {
	tests := []struct {
		name string
		a, b []float64
		want float64
	}{
		{"identical", []float64{1, 0}, []float64{1, 0}, 1},
		{"orthogonal", []float64{1, 0}, []float64{0, 1}, 0},
		{"opposite", []float64{1, 0}, []float64{-1, 0}, -1},
		{"partial", []float64{1, 1}, []float64{1, 0}, 0.7071067811865475},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cosineSimilarity(tt.a, tt.b)
			if math.Abs(got-tt.want) > 0.0001 {
				t.Errorf("cosineSimilarity(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestDefaultIgnoreDirs(t *testing.T) {
	dirs := []string{".git", "node_modules", "__pycache__", ".venv", "vendor", "target", "dist", "build"}
	for _, d := range dirs {
		if !DefaultIgnoreDirs[d] {
			t.Errorf("expected %q to be in DefaultIgnoreDirs", d)
		}
	}
}

func TestDefaultChunkConfig(t *testing.T) {
	cfg := DefaultChunkConfig()
	if cfg.ChunkSize != 512 {
		t.Errorf("expected 512, got %d", cfg.ChunkSize)
	}
	if cfg.ChunkOverlap != 64 {
		t.Errorf("expected 64, got %d", cfg.ChunkOverlap)
	}
}

func TestRAG_New(t *testing.T) {
	dir := t.TempDir()
	rag, err := New(Config{
		Enabled:        true,
		ChunkSize:      128,
		ChunkOverlap:   16,
		MaxResults:     3,
		EmbeddingModel: "text-embedding-3-small",
		RootDir:        dir,
		PersistPath:    filepath.Join(dir, "rag.json"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !rag.IsEnabled() {
		t.Error("expected RAG to be enabled")
	}
	if rag.Indexer == nil {
		t.Error("expected indexer to be initialized")
	}
	if rag.Store == nil {
		t.Error("expected store to be initialized")
	}
}

func TestRAG_QueryDisabled(t *testing.T) {
	dir := t.TempDir()
	rag, err := New(Config{
		Enabled:     false,
		RootDir:     dir,
		PersistPath: filepath.Join(dir, "rag.json"),
	})
	if err != nil {
		t.Fatal(err)
	}

	results, err := rag.Query("test", 5)
	if err != nil {
		t.Fatal(err)
	}
	if results != nil {
		t.Error("expected nil results when disabled")
	}
}

func TestRAG_New_Defaults(t *testing.T) {
	dir := t.TempDir()
	rag, err := New(Config{
		Enabled:     true,
		RootDir:     dir,
		PersistPath: filepath.Join(dir, "rag.json"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if rag.Indexer.cfg.ChunkSize != 512 {
		t.Errorf("expected default chunk size 512, got %d", rag.Indexer.cfg.ChunkSize)
	}
	if rag.Indexer.cfg.ChunkOverlap != 64 {
		t.Errorf("expected default chunk overlap 64, got %d", rag.Indexer.cfg.ChunkOverlap)
	}
	if rag.Embedder.Model() != "text-embedding-3-small" {
		t.Errorf("expected default model, got %s", rag.Embedder.Model())
	}
}

func TestRAG_Query_DefaultMaxResults(t *testing.T) {
	store := NewVectorStore(3, "")
	embedder := &mockEmbedder{}
	idx := NewIndexer(store, embedder, t.TempDir(), DefaultChunkConfig())

	store.Insert([]VectorRecord{
		{ID: "1", Chunk: Chunk{Text: "alpha"}, Embedding: []float64{1, 0, 0}},
		{ID: "2", Chunk: Chunk{Text: "beta"}, Embedding: []float64{0, 1, 0}},
		{ID: "3", Chunk: Chunk{Text: "gamma"}, Embedding: []float64{0, 0, 1}},
		{ID: "4", Chunk: Chunk{Text: "delta"}, Embedding: []float64{0.9, 0, 0.1}},
		{ID: "5", Chunk: Chunk{Text: "epsilon"}, Embedding: []float64{0.8, 0.2, 0}},
		{ID: "6", Chunk: Chunk{Text: "zeta"}, Embedding: []float64{0, 0.8, 0.2}},
	})

	rag := &RAG{
		Indexer:  idx,
		Store:    store,
		Embedder: embedder,
		enabled:  true,
	}

	results, err := rag.Query("test", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 5 {
		t.Errorf("expected 5 results with maxResults=0, got %d", len(results))
	}
}

func TestRAGContextKey(t *testing.T) {
	var key RAGContextKey
	ctx := context.WithValue(context.Background(), key, "value")
	v := ctx.Value(key)
	if v == nil {
		t.Error("expected value to be retrievable from context")
	}
	if v.(string) != "value" {
		t.Errorf("expected 'value', got %v", v)
	}
}

func TestChunkFile_NotFound(t *testing.T) {
	_, err := ChunkFile("/nonexistent/path/file.go", DefaultChunkConfig())
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestChunkFile_OverlapGtSize(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.go")
	var lines []string
	for i := 0; i < 50; i++ {
		lines = append(lines, fmt.Sprintf("line %d", i))
	}
	content := strings.Join(lines, "\n")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := ChunkConfig{ChunkSize: 10, ChunkOverlap: 20}
	chunks, err := ChunkFile(path, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) == 0 {
		t.Fatal("expected at least 1 chunk")
	}
	// With step=1 and 50 lines, we should get 41 chunks (indices 0-40)
	if len(chunks) != 41 {
		t.Errorf("expected 41 chunks (step=1 due to overlap>=chunkSize), got %d", len(chunks))
	}
}

func TestLanguageFromExt_AllBranches(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"file.js", "javascript"},
		{"file.jsx", "javascript"},
		{"file.h", "c"},
		{"file.hpp", "cpp"},
		{"file.cc", "cpp"},
		{"file.cs", "csharp"},
		{"file.rb", "ruby"},
		{"file.php", "php"},
		{"file.swift", "swift"},
		{"file.kt", "kotlin"},
		{"file.bash", "shell"},
		{"file.zsh", "shell"},
		{"file.fish", "shell"},
		{"file.yml", "yaml"},
		{"file.toml", "toml"},
		{"file.svelte", "html"},
		{"file.vue", "html"},
		{"file.css", "css"},
		{"file.scss", "css"},
		{"file.less", "css"},
		{"file.txt", "text"},
		{"file", "text"},
	}
	for _, tt := range tests {
		got := LanguageFromExt(tt.path)
		if got != tt.want {
			t.Errorf("LanguageFromExt(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestVectorStore_InsertEmptyID(t *testing.T) {
	store := NewVectorStore(3, "")
	store.Insert([]VectorRecord{
		{ID: "", Chunk: Chunk{Text: "no-id"}, Embedding: []float64{1, 0, 0}},
		{ID: "valid", Chunk: Chunk{Text: "valid"}, Embedding: []float64{0, 1, 0}},
	})
	if store.Len() != 1 {
		t.Errorf("expected 1 record (empty ID skipped), got %d", store.Len())
	}
}

func TestVectorStore_SearchEmpty(t *testing.T) {
	store := NewVectorStore(3, "")
	results := store.Search([]float64{1, 0, 0}, 5)
	if results != nil {
		t.Error("expected nil results from empty store")
	}

	results = store.Search([]float64{1, 0, 0}, 0)
	if results != nil {
		t.Error("expected nil results with maxResults=0")
	}
}

func TestVectorStore_Clear_WithPersist(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "vectors.json")
	store := NewVectorStore(3, path)
	store.Insert([]VectorRecord{
		{ID: "a", Chunk: Chunk{Text: "hello"}, Embedding: []float64{1, 0, 0}},
		{ID: "b", Chunk: Chunk{Text: "world"}, Embedding: []float64{0, 1, 0}},
	})
	store.Clear()
	if store.Len() != 0 {
		t.Fatal("expected 0 records after clear")
	}
	// Verify persistence file was updated (should exist and be empty array)
	if _, err := os.Stat(path); err != nil {
		t.Fatal("expected persist file to exist after clear with persist")
	}
}

func TestVectorStore_Stats(t *testing.T) {
	store := NewVectorStore(5, "")
	count, dims := store.Stats()
	if count != 0 {
		t.Errorf("expected 0 count, got %d", count)
	}
	if dims != 5 {
		t.Errorf("expected 5 dims, got %d", dims)
	}

	store.Insert([]VectorRecord{
		{ID: "1", Chunk: Chunk{Text: "a"}, Embedding: []float64{1, 0, 0, 0, 0}},
	})
	count, dims = store.Stats()
	if count != 1 {
		t.Errorf("expected 1 count, got %d", count)
	}
}

func TestVectorStore_Persistence_NoFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent.json")
	store := NewVectorStore(3, path)
	if store.Len() != 0 {
		t.Error("expected empty store from nonexistent persist file")
	}
}

func TestVectorStore_Persistence_Corrupt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "corrupt.json")
	if err := os.WriteFile(path, []byte("{invalid json}"), 0o600); err != nil {
		t.Fatal(err)
	}
	store := NewVectorStore(3, path)
	if store.Len() != 0 {
		t.Error("expected empty store from corrupt persist file")
	}
}

func TestVectorStore_Persistence_SaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "vectors.json")

	store1 := NewVectorStore(3, path)
	store1.Insert([]VectorRecord{
		{ID: "a", Chunk: Chunk{Text: "hello"}, Embedding: []float64{1, 0, 0}},
		{ID: "b", Chunk: Chunk{Text: "world"}, Embedding: []float64{0, 1, 0}},
	})
	store1.Delete("b")

	store2 := NewVectorStore(3, path)
	if store2.Len() != 1 {
		t.Errorf("expected 1 record after save+load, got %d", store2.Len())
	}
	if store2.persist != path {
		t.Errorf("expected persist path to be %q", path)
	}
}

func TestCosineSimilarity_EdgeCases(t *testing.T) {
	tests := []struct {
		name string
		a, b []float64
		want float64
	}{
		{"mismatched lengths", []float64{1, 0}, []float64{1, 0, 0}, 0},
		{"both empty", nil, nil, 0},
		{"zero norm A", []float64{0, 0}, []float64{1, 1}, 0},
		{"zero norm B", []float64{1, 1}, []float64{0, 0}, 0},
		{"both zero", []float64{0, 0}, []float64{0, 0}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cosineSimilarity(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("cosineSimilarity = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIndexer_ShouldIgnoreDir(t *testing.T) {
	store := NewVectorStore(3, "")
	idx := NewIndexer(store, &mockEmbedder{}, t.TempDir(), DefaultChunkConfig())

	if !idx.shouldIgnoreDir(".git") {
		t.Error("expected .git to be ignored")
	}
	if !idx.shouldIgnoreDir("node_modules") {
		t.Error("expected node_modules to be ignored")
	}
	if !idx.shouldIgnoreDir(".hidden") {
		t.Error("expected hidden dir to be ignored")
	}
	if idx.shouldIgnoreDir("src") {
		t.Error("expected src not to be ignored")
	}
}

func TestIndexer_AddIgnoreDir(t *testing.T) {
	store := NewVectorStore(3, "")
	idx := NewIndexer(store, &mockEmbedder{}, t.TempDir(), DefaultChunkConfig())

	idx.AddIgnoreDir("mydir")
	if !idx.shouldIgnoreDir("mydir") {
		t.Error("expected mydir to be ignored after AddIgnoreDir")
	}
}

func TestIndexer_LastRun(t *testing.T) {
	store := NewVectorStore(3, "")
	idx := NewIndexer(store, &mockEmbedder{}, t.TempDir(), DefaultChunkConfig())

	zero := idx.LastRun()
	if !zero.IsZero() {
		t.Error("expected zero time before any index run")
	}
}

func TestIndexer_TotalVectors(t *testing.T) {
	store := NewVectorStore(3, "")
	idx := NewIndexer(store, &mockEmbedder{}, t.TempDir(), DefaultChunkConfig())

	if n := idx.TotalVectors(); n != 0 {
		t.Errorf("expected 0 vectors, got %d", n)
	}

	store.Insert([]VectorRecord{
		{ID: "1", Chunk: Chunk{Text: "a"}, Embedding: []float64{1, 0, 0}},
		{ID: "2", Chunk: Chunk{Text: "b"}, Embedding: []float64{0, 1, 0}},
	})
	if n := idx.TotalVectors(); n != 2 {
		t.Errorf("expected 2 vectors, got %d", n)
	}
}

func TestIndexer_Index_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	store := NewVectorStore(3, "")
	idx := NewIndexer(store, &mockEmbedder{}, dir, DefaultChunkConfig())

	stats, err := idx.Index()
	if err != nil {
		t.Fatal(err)
	}
	if stats.FilesScanned != 0 {
		t.Errorf("expected 0 files scanned, got %d", stats.FilesScanned)
	}
	if stats.FilesIndexed != 0 {
		t.Errorf("expected 0 files indexed, got %d", stats.FilesIndexed)
	}
	if stats.DurationMs < 0 {
		t.Error("expected non-negative duration")
	}
}

func TestIndexer_Index_WithFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\nfunc main() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "util.py"), []byte("def foo():\n    pass\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "data.csv"), []byte("a,b,c\n1,2,3\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	store := NewVectorStore(3, "")
	idx := NewIndexer(store, &mockEmbedder{}, dir, DefaultChunkConfig())

	stats, err := idx.Index()
	if err != nil {
		t.Fatal(err)
	}
	if stats.FilesScanned != 2 {
		t.Errorf("expected 2 files scanned (go+py), got %d", stats.FilesScanned)
	}
	if stats.FilesIndexed != 2 {
		t.Errorf("expected 2 files indexed, got %d", stats.FilesIndexed)
	}
	if stats.ChunksCreated < 1 {
		t.Errorf("expected at least 1 chunk, got %d", stats.ChunksCreated)
	}
	if stats.TotalVectors < 1 {
		t.Errorf("expected at least 1 total vector, got %d", stats.TotalVectors)
	}
	if stats.LastRun == "" {
		t.Error("expected non-empty last_run")
	}

	// Second index should be no-op (files unchanged)
	stats2, err := idx.Index()
	if err != nil {
		t.Fatal(err)
	}
	if stats2.FilesIndexed != 0 {
		t.Errorf("expected 0 files indexed on second run, got %d", stats2.FilesIndexed)
	}
}

func TestIndexer_Index_IgnoreDirs(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "node_modules", "pkg"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "node_modules", "pkg", "index.js"), []byte("const x = 1;\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\nfunc main() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	store := NewVectorStore(3, "")
	idx := NewIndexer(store, &mockEmbedder{}, dir, DefaultChunkConfig())

	stats, err := idx.Index()
	if err != nil {
		t.Fatal(err)
	}
	if stats.FilesScanned != 1 {
		t.Errorf("expected 1 file scanned (ignoring node_modules), got %d", stats.FilesScanned)
	}
}

func TestIndexer_Index_NonUTF8File(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "binary.go"), []byte{0xFF, 0xFE, 0x00, 0x01}, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\nfunc main() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	store := NewVectorStore(3, "")
	idx := NewIndexer(store, &mockEmbedder{}, dir, DefaultChunkConfig())

	stats, err := idx.Index()
	if err != nil {
		t.Fatal(err)
	}
	// Binary file should be skipped (ChunkFile returns nil,nil for invalid UTF-8),
	// only main.go should be indexed
	if stats.FilesIndexed != 1 {
		t.Errorf("expected 1 file indexed (binary skipped), got %d", stats.FilesIndexed)
	}
}

func TestIndexer_Index_WalkError(t *testing.T) {
	store := NewVectorStore(3, "")
	idx := NewIndexer(store, &mockEmbedder{}, "/nonexistent/path", DefaultChunkConfig())

	_, err := idx.Index()
	if err == nil {
		t.Error("expected error for non-existent root dir")
	}
}

func TestIndexer_Index_EmbedError(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\nfunc main() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	store := NewVectorStore(3, "")
	idx := NewIndexer(store, &errEmbedder{}, dir, DefaultChunkConfig())

	stats, err := idx.Index()
	if err != nil {
		t.Fatal(err)
	}
	// Embed error causes the file to be skipped
	if stats.FilesIndexed != 0 {
		t.Errorf("expected 0 files indexed (embed error), got %d", stats.FilesIndexed)
	}
	if stats.FilesScanned != 1 {
		t.Errorf("expected 1 file scanned, got %d", stats.FilesScanned)
	}
}

func TestIndexer_Query_EmptyEmbeddings(t *testing.T) {
	store := NewVectorStore(3, "")
	idx := NewIndexer(store, &emptyEmbedder{}, t.TempDir(), DefaultChunkConfig())

	results, err := idx.Query("test", 5)
	if err != nil {
		t.Fatal(err)
	}
	if results != nil {
		t.Error("expected nil results when embedder returns empty")
	}
}

func TestIndexer_Query_EmbedError(t *testing.T) {
	store := NewVectorStore(3, "")
	idx := NewIndexer(store, &errEmbedder{}, t.TempDir(), DefaultChunkConfig())

	_, err := idx.Query("test", 5)
	if err == nil {
		t.Error("expected error from errEmbedder")
	}
}

func TestFileHash(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	content := "hello world"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	hash, err := fileHash(path)
	if err != nil {
		t.Fatal(err)
	}
	if hash == "" {
		t.Error("expected non-empty hash")
	}
	if len(hash) != 64 {
		t.Errorf("expected 64 char hex hash, got %d", len(hash))
	}

	// Verify it's deterministic
	hash2, err := fileHash(path)
	if err != nil {
		t.Fatal(err)
	}
	if hash != hash2 {
		t.Error("expected same hash for same content")
	}
}

func TestFileHash_Error(t *testing.T) {
	_, err := fileHash("/nonexistent/file.txt")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

type mockEmbedder struct{}

func (m *mockEmbedder) Embed(texts []string) ([][]float64, error) {
	result := make([][]float64, len(texts))
	for i := range result {
		result[i] = []float64{1, 0, 0}
	}
	return result, nil
}

func (m *mockEmbedder) Model() string   { return "mock" }
func (m *mockEmbedder) Dimensions() int { return 3 }

type errEmbedder struct{}

func (e *errEmbedder) Embed(texts []string) ([][]float64, error) {
	return nil, fmt.Errorf("embed error")
}

func (e *errEmbedder) Model() string   { return "error" }
func (e *errEmbedder) Dimensions() int { return 3 }

type emptyEmbedder struct{}

func (e *emptyEmbedder) Embed(texts []string) ([][]float64, error) {
	return nil, nil
}

func (e *emptyEmbedder) Model() string   { return "empty" }
func (e *emptyEmbedder) Dimensions() int { return 0 }
