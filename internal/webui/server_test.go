// SPDX-License-Identifier: MIT
package webui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Dxrk777/Dxrk/internal/config"
	"github.com/Dxrk777/Dxrk/internal/router"
	"github.com/Dxrk777/Dxrk/internal/version"
)

func TestNewServer(t *testing.T) {
	s := NewServer(
		&config.WebUIConfig{Host: "127.0.0.1", Port: 8080},
		nil, nil, nil, nil, nil, nil, nil,
	)
	if s == nil {
		t.Fatal("expected non-nil Server")
	}
}

func TestNewServerWithNilHub(t *testing.T) {
	s := NewServer(
		&config.WebUIConfig{Host: "0.0.0.0", Port: 9090},
		nil, nil, nil, nil, nil, nil, nil,
	)
	if s.hub == nil {
		t.Fatal("expected hub to be initialized when nil is passed")
	}
}

func TestHealthEndpoint(t *testing.T) {
	s := NewServer(
		&config.WebUIConfig{Host: "127.0.0.1", Port: 8080},
		nil, nil, nil, nil, nil, nil, nil,
	)

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	w := httptest.NewRecorder()
	s.HandleHealth(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("expected 'ok', got %q", body["status"])
	}
}

func TestStatusEndpoint(t *testing.T) {
	s := NewServer(
		&config.WebUIConfig{Host: "127.0.0.1", Port: 8080},
		nil, nil, nil, nil, nil, nil, nil,
	)

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	w := httptest.NewRecorder()
	s.HandleStatus(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var status StatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if status.Version != version.Version {
		t.Fatalf("expected version %q, got %q", version.Version, status.Version)
	}
	if status.Status != "ok" {
		t.Fatalf("expected status 'ok', got %q", status.Status)
	}
	if status.Uptime == "" {
		t.Fatal("expected non-empty uptime")
	}
}

func TestStatusEndpointWithRouter(t *testing.T) {
	providers := []router.ProviderEntry{
		{Name: "test-provider", Model: "gpt-4o"},
	}
	r := router.NewRouter(providers)
	s := NewServer(
		&config.WebUIConfig{Host: "127.0.0.1", Port: 8080},
		r, nil, nil, nil, nil, nil, nil,
	)

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	w := httptest.NewRecorder()
	s.HandleStatus(w, req)

	var status StatusResponse
	if err := json.NewDecoder(w.Body).Decode(&status); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(status.Providers) != 1 {
		t.Fatalf("expected 1 provider, got %d", len(status.Providers))
	}
	if status.Providers[0].Name != "test-provider" {
		t.Fatalf("expected 'test-provider', got %q", status.Providers[0].Name)
	}
}

func TestConfigEndpoint(t *testing.T) {
	cfg := &config.WebUIConfig{Host: "0.0.0.0", Port: 3000}
	s := NewServer(cfg, nil, nil, nil, nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	w := httptest.NewRecorder()
	s.HandleConfig(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var returned config.WebUIConfig
	if err := json.NewDecoder(resp.Body).Decode(&returned); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if returned.Host != "0.0.0.0" {
		t.Fatalf("expected host '0.0.0.0', got %q", returned.Host)
	}
	if returned.Port != 3000 {
		t.Fatalf("expected port 3000, got %d", returned.Port)
	}
}

func TestProvidersEndpoint(t *testing.T) {
	s := NewServer(
		&config.WebUIConfig{Host: "127.0.0.1", Port: 8080},
		nil, nil, nil, nil, nil, nil, nil,
	)

	req := httptest.NewRequest(http.MethodGet, "/api/providers", nil)
	w := httptest.NewRecorder()
	s.HandleProviders(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var providers []providerStatus
	if err := json.NewDecoder(resp.Body).Decode(&providers); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(providers) != 0 {
		t.Fatalf("expected 0 providers, got %d", len(providers))
	}
}

func TestProvidersEndpointWithRouter(t *testing.T) {
	providers := []router.ProviderEntry{
		{Name: "alpha", Model: "gpt-4o"},
		{Name: "beta", Model: "claude-sonnet-4"},
	}
	r := router.NewRouter(providers)
	s := NewServer(
		&config.WebUIConfig{Host: "127.0.0.1", Port: 8080},
		r, nil, nil, nil, nil, nil, nil,
	)

	req := httptest.NewRequest(http.MethodGet, "/api/providers", nil)
	w := httptest.NewRecorder()
	s.HandleProviders(w, req)

	var result []providerStatus
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(result))
	}
}

func TestRequestCounter(t *testing.T) {
	s := NewServer(
		&config.WebUIConfig{Host: "127.0.0.1", Port: 8080},
		nil, nil, nil, nil, nil, nil, nil,
	)

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
		w := httptest.NewRecorder()
		s.HandleStatus(w, req)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	w := httptest.NewRecorder()
	s.HandleStatus(w, req)

	var status StatusResponse
	json.NewDecoder(w.Body).Decode(&status)
	if status.Requests != 4 {
		t.Fatalf("expected 4 requests, got %d", status.Requests)
	}
}

func TestWebSocketHub(t *testing.T) {
	hub := NewWebSocketHub()
	if hub == nil {
		t.Fatal("expected non-nil WebSocketHub")
	}
}

func TestBroadcastEvent(t *testing.T) {
	s := NewServer(
		&config.WebUIConfig{Host: "127.0.0.1", Port: 8080},
		nil, nil, nil, nil, nil, nil, nil,
	)

	s.BroadcastEvent("test", map[string]string{"key": "value"})
}

func TestStatusEndpointContentType(t *testing.T) {
	s := NewServer(
		&config.WebUIConfig{Host: "127.0.0.1", Port: 8080},
		nil, nil, nil, nil, nil, nil, nil,
	)

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	w := httptest.NewRecorder()
	s.HandleStatus(w, req)

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Fatalf("expected 'application/json', got %q", ct)
	}
}
