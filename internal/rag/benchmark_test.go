// SPDX-License-Identifier: MIT
package rag

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func BenchmarkChunkFile_Small(b *testing.B) {
	dir := b.TempDir()
	fpath := filepath.Join(dir, "main.go")
	content := generateCode(200)
	_ = os.WriteFile(fpath, []byte(content), 0o600)
	cfg := DefaultChunkConfig()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ChunkFile(fpath, cfg)
	}
}

func BenchmarkChunkFile_Large(b *testing.B) {
	dir := b.TempDir()
	fpath := filepath.Join(dir, "large.go")
	content := generateCode(5000)
	_ = os.WriteFile(fpath, []byte(content), 0o600)
	cfg := DefaultChunkConfig()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ChunkFile(fpath, cfg)
	}
}

func BenchmarkChunkFile_Overlapping(b *testing.B) {
	dir := b.TempDir()
	fpath := filepath.Join(dir, "big.go")
	content := generateCode(2000)
	_ = os.WriteFile(fpath, []byte(content), 0o600)
	cfg := ChunkConfig{ChunkSize: 150, ChunkOverlap: 30}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ChunkFile(fpath, cfg)
	}
}

func BenchmarkVectorStore_Insert_100(b *testing.B) {
	s := NewVectorStore(1536, "")
	recs := make([]VectorRecord, 100)
	for i := range recs {
		recs[i] = VectorRecord{
			ID:        fmt.Sprintf("rec-%d", i),
			Embedding: randomEmbedding(),
			Chunk:     Chunk{Text: fmt.Sprintf("chunk %d", i), FilePath: "test.go", StartLine: i*10 + 1, EndLine: (i + 1) * 10, Language: "go"},
		}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Insert(recs)
	}
}

func BenchmarkVectorStore_Search_1000(b *testing.B) {
	s := NewVectorStore(1536, "")
	for i := 0; i < 1000; i++ {
		s.Insert([]VectorRecord{{
			ID:        fmt.Sprintf("rec-%d", i),
			Embedding: randomEmbedding(),
		}})
	}
	query := randomEmbedding()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Search(query, 10)
	}
}

func BenchmarkVectorStore_Search_10000(b *testing.B) {
	s := NewVectorStore(1536, "")
	for i := 0; i < 10000; i++ {
		s.Insert([]VectorRecord{{
			ID:        fmt.Sprintf("rec-%d", i),
			Embedding: randomEmbedding(),
		}})
	}
	query := randomEmbedding()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Search(query, 10)
	}
}

func BenchmarkCosineSimilarity(b *testing.B) {
	a := randomEmbedding()
	c := randomEmbedding()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cosineSimilarity(a, c)
	}
}

func BenchmarkIsCodeFile(b *testing.B) {
	extensions := []string{".go", ".tsx", ".py", ".rs", ".cpp", ".md", ".yaml", ".css", ".html", ".vue"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		IsCodeFile("path/to/file" + extensions[i%len(extensions)])
	}
}

func BenchmarkLanguageFromExt(b *testing.B) {
	extensions := []string{".go", ".ts", ".tsx", ".js", ".py", ".rs", ".java", ".cpp", ".md", ".sh", ".yaml", ".json", ".html", ".css"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		LanguageFromExt("path/to/file" + extensions[i%len(extensions)])
	}
}

func generateCode(lines int) string {
	var sb strings.Builder
	sb.WriteString("package main\n\nimport \"fmt\"\n\nfunc main() {\n")
	for i := 0; i < lines-5; i++ {
		fmt.Fprintf(&sb, "\t// line %d: processing step\n\tfmt.Println(\"step %d\")\n", i, i)
	}
	sb.WriteString("}\n")
	return sb.String()
}

func randomEmbedding() []float64 {
	v := make([]float64, 1536)
	for i := range v {
		v[i] = rand.Float64()*2 - 1 //nolint:gosec
	}
	return v
}
