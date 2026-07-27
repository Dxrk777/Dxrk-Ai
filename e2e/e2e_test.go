// SPDX-License-Identifier: MIT
package e2e

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Dxrk777/Dxrk-Ai/internal/config"
	"github.com/Dxrk777/Dxrk-Ai/internal/pipeline"
	"github.com/Dxrk777/Dxrk-Ai/internal/query"
	"github.com/Dxrk777/Dxrk-Ai/internal/rag"
	"github.com/Dxrk777/Dxrk-Ai/internal/router"
	"github.com/Dxrk777/Dxrk-Ai/internal/vault"
	"github.com/Dxrk777/Dxrk-Ai/internal/webui"
)

type mockProvider struct {
	name  string
	delay time.Duration
}

func (m *mockProvider) Generate(ctx context.Context, msgs []query.Message, tools []query.ToolSchema) (query.Response, error) {
	if m.delay > 0 {
		select {
		case <-ctx.Done():
			return query.Response{}, ctx.Err()
		case <-time.After(m.delay):
		}
	}

	// Check if this is a review prompt
	isReview := false
	for _, msg := range msgs {
		if strings.Contains(strings.ToLower(msg.Content), "review") {
			isReview = true
			break
		}
	}

	var text string
	if isReview {
		text = ""
	} else {
		text = fmt.Sprintf("```go\npackage main\n\nfunc main() {\n\tfmt.Println(\"hello from %s\")\n}\n```", m.name)
	}

	return query.Response{
		Text: text,
		Usage: query.Usage{
			InputTokens:  100,
			OutputTokens: 50,
		},
	}, nil
}

func TestMain(m *testing.M) {
	exitCode := m.Run()
	os.Exit(exitCode)
}

func TestConfigLoad(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "dxrk.yaml")

	cfg := config.Default()
	err := config.Save(cfgPath, cfg)
	if err != nil {
		t.Fatalf("save config: %v", err)
	}

	loaded, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if loaded.Project.Name != cfg.Project.Name {
		t.Fatal("project name mismatch")
	}
}

func TestRouterWithCache(t *testing.T) {
	r := router.NewRouter([]router.ProviderEntry{
		{Name: "test", Model: "gpt-4o-mini", Provider: &mockProvider{name: "test"}},
	})
	cache := router.NewSemanticCache()
	cr := router.NewCachingRouter(r, cache)

	ctx := context.Background()
	msgs := []query.Message{{Role: query.RoleUser, Content: "test"}}

	resp1, err := cr.CachedGenerate(ctx, msgs, nil)
	if err != nil {
		t.Fatal(err)
	}

	resp2, err := cr.CachedGenerate(ctx, msgs, nil)
	if err != nil {
		t.Fatal(err)
	}

	if resp1.Text != resp2.Text {
		t.Fatal("cached response should match")
	}
}

func TestPipelineExecution(t *testing.T) {
	r := router.NewRouter([]router.ProviderEntry{
		{Name: "test", Model: "gpt-4o-mini", Provider: &mockProvider{name: "test", delay: 10 * time.Millisecond}},
	})

	cfg := pipeline.DefaultPipelineConfig(r)
	cfg.MaxIterations = 2
	cfg.ReviewPrompt = "You are a code reviewer. Return PASS with no issues."

	p := pipeline.NewPipeline(cfg)
	p.Stop()

	task := pipeline.PipelineTask{
		ID:          "e2e-task",
		Description: "Write a Fibonacci function in Go",
		Language:    "go",
	}

	result := p.Execute(context.Background(), task)

	if !result.Success {
		t.Fatalf("pipeline failed: iterations=%d", result.Iterations)
	}
	if result.Code == "" {
		t.Fatal("expected code output")
	}
	if result.Tests == "" {
		t.Fatal("expected tests output")
	}
	if result.Iterations < 1 {
		t.Fatal("expected at least 1 iteration")
	}
}

func TestVaultPersist(t *testing.T) {
	tmp := t.TempDir()
	vaultPath := filepath.Join(tmp, "vault.enc")
	_ = os.Setenv("DXRK_VAULT_KEY_E2E", "test-master-key-12345")
	defer func() { _ = os.Unsetenv("DXRK_VAULT_KEY_E2E") }()

	v1, err := vault.New(vaultPath, "DXRK_VAULT_KEY_E2E")
	if err != nil {
		t.Fatal(err)
	}

	if err := v1.Set("api-key", "sk-test123"); err != nil {
		t.Fatal(err)
	}

	v2, err := vault.New(vaultPath, "DXRK_VAULT_KEY_E2E")
	if err != nil {
		t.Fatal(err)
	}

	val, ok := v2.Get("api-key")
	if !ok {
		t.Fatal("expected to find persisted key")
	}
	if val != "sk-test123" {
		t.Fatalf("expected sk-test123, got %q", val)
	}
}

func TestRAGChunker(t *testing.T) {
	tmp := t.TempDir()
	goFile := filepath.Join(tmp, "test.go")
	_ = os.WriteFile(goFile, []byte("package main\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n"), 0o600)

	chunks, err := rag.ChunkFile(goFile, rag.DefaultChunkConfig())
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) == 0 {
		t.Fatal("expected at least one chunk")
	}
	if chunks[0].FilePath != goFile {
		t.Fatalf("expected file path %s, got %s", goFile, chunks[0].FilePath)
	}
	if chunks[0].Language != "go" {
		t.Fatalf("expected language go, got %s", chunks[0].Language)
	}
}

func TestWebUIHandlers(t *testing.T) {
	r := router.NewRouter([]router.ProviderEntry{
		{Name: "test", Model: "gpt-4o-mini", Provider: &mockProvider{name: "test"}},
	})

	hub := webui.NewWebSocketHub()
	srv := webui.NewServer(&config.WebUIConfig{Enabled: true, Port: 8080, Host: "127.0.0.1"}, r, nil, nil, nil, nil, nil, hub)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/status", srv.HandleStatus)
	mux.HandleFunc("/api/health", srv.HandleHealth)
	mux.HandleFunc("/api/config", srv.HandleConfig)
	mux.HandleFunc("/api/providers", srv.HandleProviders)

	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/health")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	resp, err = http.Get(ts.URL + "/api/status")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(resp.Body)
	_ = resp.Body.Close()
	if buf.Len() == 0 {
		t.Fatal("empty response")
	}
}

func TestFullStackConfig(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "dxrk.yaml")

	cfg := config.Default()
	cfg.Vault = &config.VaultConfig{
		Enabled:      true,
		Path:         filepath.Join(tmp, "vault.enc"),
		MasterKeyEnv: "DXRK_VAULT_KEY_E2E",
	}
	cfg.Cache = &config.CacheConfig{
		Enabled:           true,
		MaxSize:           100,
		TTLSeconds:        60,
		SemanticEnabled:   false,
		SemanticThreshold: 0.9,
	}

	err := config.Save(cfgPath, cfg)
	if err != nil {
		t.Fatal(err)
	}

	loaded, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}

	if !loaded.Vault.Enabled {
		t.Fatal("vault config not persisted")
	}
	if !loaded.Cache.Enabled {
		t.Fatal("cache config not persisted")
	}
}
