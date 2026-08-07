// SPDX-License-Identifier: MIT
package webui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Dxrk777/Dxrk/internal/autonomy"
	"github.com/Dxrk777/Dxrk/internal/config"
	"github.com/Dxrk777/Dxrk/internal/rag"
	"github.com/Dxrk777/Dxrk/internal/router"
	"github.com/Dxrk777/Dxrk/internal/vault"
	"github.com/Dxrk777/Dxrk/internal/version"

	"golang.org/x/net/websocket"
)

// discardFirstWSMessage reads and discards the first WebSocket message
// (the "connected" broadcast sent when a client joins).
func discardFirstWSMessage(ws *websocket.Conn) {
	var ignored string
	_ = websocket.Message.Receive(ws, &ignored)
}

// ─── itoa ───────────────────────────────────────────────────────────

func TestItoa_Zero(t *testing.T) {
	if got := itoa(0); got != "8080" {
		t.Fatalf("itoa(0) = %q, want %q", got, "8080")
	}
}

func TestItoa_NonZero(t *testing.T) {
	if got := itoa(8080); got != "8080" {
		t.Fatalf("itoa(8080) = %q, want %q", got, "8080")
	}
}

func TestItoa_CustomPort(t *testing.T) {
	if got := itoa(3000); got != "3000" {
		t.Fatalf("itoa(3000) = %q, want %q", got, "3000")
	}
}

// ─── writeJSON ──────────────────────────────────────────────────────

func TestWriteJSON_SetsContentType(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, map[string]string{"k": "v"})

	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q, want %q", ct, "application/json")
	}
}

func TestWriteJSON_Body(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, map[string]int{"x": 42})

	var body map[string]int
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["x"] != 42 {
		t.Fatalf("body[x] = %d, want 42", body["x"])
	}
}

// ─── withMiddleware ─────────────────────────────────────────────────

func TestMiddleware_CORSHeaders(t *testing.T) {
	h := withMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if origin := w.Header().Get("Access-Control-Allow-Origin"); origin != "*" {
		t.Fatalf("CORS origin = %q, want %q", origin, "*")
	}
	if methods := w.Header().Get("Access-Control-Allow-Methods"); methods != "GET, POST, OPTIONS, PUT" {
		t.Fatalf("CORS methods = %q", methods)
	}
	if headers := w.Header().Get("Access-Control-Allow-Headers"); headers != "Content-Type, Authorization" {
		t.Fatalf("CORS headers = %q", headers)
	}
}

func TestMiddleware_VersionHeader(t *testing.T) {
	h := withMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if v := w.Header().Get("X-Dxrk-Version"); v != version.Version {
		t.Fatalf("version header = %q, want %q", v, version.Version)
	}
}

func TestMiddleware_OPTIONS_Returns204(t *testing.T) {
	h := withMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodOptions, "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("OPTIONS status = %d, want 204", w.Code)
	}
}

func TestMiddleware_OPTIONS_SkipsNext(t *testing.T) {
	called := false
	h := withMiddleware(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		called = true
	}))
	req := httptest.NewRequest(http.MethodOptions, "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if called {
		t.Fatal("next handler should not be called for OPTIONS")
	}
}

func TestMiddleware_GET_PassesThrough(t *testing.T) {
	called := false
	h := withMiddleware(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		called = true
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if !called {
		t.Fatal("next handler should be called for GET")
	}
}

// ─── generateMockLogs ───────────────────────────────────────────────

func TestGenerateMockLogs_Length(t *testing.T) {
	logs := generateMockLogs()
	if len(logs) != 6 {
		t.Fatalf("expected 6 log entries, got %d", len(logs))
	}
}

func TestGenerateMockLogs_Fields(t *testing.T) {
	logs := generateMockLogs()
	for i, l := range logs {
		if l.Timestamp == "" {
			t.Fatalf("log[%d] missing timestamp", i)
		}
		if l.Level == "" {
			t.Fatalf("log[%d] missing level", i)
		}
		if l.Source == "" {
			t.Fatalf("log[%d] missing source", i)
		}
		if l.Message == "" {
			t.Fatalf("log[%d] missing message", i)
		}
	}
}

func TestGenerateMockLogs_Levels(t *testing.T) {
	logs := generateMockLogs()
	hasInfo, hasWarn, hasError := false, false, false
	for _, l := range logs {
		switch l.Level {
		case "INFO":
			hasInfo = true
		case "WARN":
			hasWarn = true
		case "ERROR":
			hasError = true
		}
	}
	if !hasInfo {
		t.Fatal("expected at least one INFO entry")
	}
	if !hasWarn {
		t.Fatal("expected at least one WARN entry")
	}
	if !hasError {
		t.Fatal("expected at least one ERROR entry")
	}
}

// ─── Start() fallback route ────────────────────────────────────────

func TestStart_FallbackRoute(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"name":    "Dxrk.ai API",
			"version": version.Version,
			"status":  statusRunning,
		})
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var body map[string]string
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["name"] != "Dxrk.ai API" {
		t.Fatalf("name = %q", body["name"])
	}
	if body["version"] != version.Version {
		t.Fatalf("version = %q", body["version"])
	}
	if body["status"] != statusRunning {
		t.Fatalf("status = %q", body["status"])
	}
}

// ─── Status endpoint with all dependencies ─────────────────────────

func TestStatusEndpoint_WithAutonomy(t *testing.T) {
	auto := autonomy.New(
		&autonomy.AutonomyConfig{Enabled: true},
		".",
		nil,
	)
	s := NewServer(
		&config.WebUIConfig{Host: "127.0.0.1", Port: 8080},
		nil, auto, nil, nil, nil, nil, nil,
	)

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	w := httptest.NewRecorder()
	s.HandleStatus(w, req)

	var status StatusResponse
	if err := json.NewDecoder(w.Body).Decode(&status); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if status.Autonomy == nil {
		t.Fatal("expected autonomy status")
	}
	if !status.Autonomy.Enabled {
		t.Fatal("expected autonomy enabled")
	}
}

func TestStatusEndpoint_WithRAG(t *testing.T) {
	store := rag.NewVectorStore(128, "")
	r := &rag.RAG{Store: store}

	s := NewServer(
		&config.WebUIConfig{Host: "127.0.0.1", Port: 8080},
		nil, nil, r, nil, nil, nil, nil,
	)

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	w := httptest.NewRecorder()
	s.HandleStatus(w, req)

	var status StatusResponse
	if err := json.NewDecoder(w.Body).Decode(&status); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if status.RAG == nil {
		t.Fatal("expected RAG status")
	}
	if status.RAG.Dimensions != 128 {
		t.Fatalf("dimensions = %d, want 128", status.RAG.Dimensions)
	}
}

func TestStatusEndpoint_WithVault(t *testing.T) {
	v, err := vault.New("", "")
	if err != nil {
		t.Fatalf("vault.New: %v", err)
	}
	s := NewServer(
		&config.WebUIConfig{Host: "127.0.0.1", Port: 8080},
		nil, nil, nil, v, nil, nil, nil,
	)

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	w := httptest.NewRecorder()
	s.HandleStatus(w, req)

	var status StatusResponse
	if err := json.NewDecoder(w.Body).Decode(&status); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if status.Vault == nil {
		t.Fatal("expected vault status")
	}
	if !status.Vault.Enabled {
		t.Fatal("expected vault enabled")
	}
}

func TestStatusEndpoint_WithCache(t *testing.T) {
	cache := router.NewSemanticCache(
		router.WithMaxSize(500),
		router.WithTTL(10*time.Second),
	)
	s := NewServer(
		&config.WebUIConfig{Host: "127.0.0.1", Port: 8080},
		nil, nil, nil, nil, cache, nil, nil,
	)

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	w := httptest.NewRecorder()
	s.HandleStatus(w, req)

	var status StatusResponse
	if err := json.NewDecoder(w.Body).Decode(&status); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if status.Cache == nil {
		t.Fatal("expected cache status")
	}
	if status.Cache.MaxSize != 500 {
		t.Fatalf("max_size = %d, want 500", status.Cache.MaxSize)
	}
	if status.Cache.TTL != "10s" {
		t.Fatalf("ttl = %q, want %q", status.Cache.TTL, "10s")
	}
}

func TestStatusEndpoint_PipelineNotPopulated(t *testing.T) {
	// The Server constructor accepts pipelineInstance but does not store it.
	// Pipeline should always be nil in the response.
	s := NewServer(
		&config.WebUIConfig{Host: "127.0.0.1", Port: 8080},
		nil, nil, nil, nil, nil, nil, nil,
	)

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	w := httptest.NewRecorder()
	s.HandleStatus(w, req)

	var status StatusResponse
	if err := json.NewDecoder(w.Body).Decode(&status); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if status.Pipeline != nil {
		t.Fatal("expected pipeline to be nil")
	}
}

func TestStatusEndpoint_AllDependencies(t *testing.T) {
	providers := []router.ProviderEntry{
		{Name: "p1", Model: "gpt-4o"},
	}
	r := router.NewRouter(providers)
	auto := autonomy.New(
		&autonomy.AutonomyConfig{Enabled: true, Evolution: true},
		".",
		nil,
	)
	store := rag.NewVectorStore(256, "")
	ragInstance := &rag.RAG{Store: store}
	v, err := vault.New("", "")
	if err != nil {
		t.Fatalf("vault.New: %v", err)
	}
	cache := router.NewSemanticCache()

	s := NewServer(
		&config.WebUIConfig{Host: "0.0.0.0", Port: 8080},
		r, auto, ragInstance, v, cache, nil, nil,
	)

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	w := httptest.NewRecorder()
	s.HandleStatus(w, req)

	var status StatusResponse
	if err := json.NewDecoder(w.Body).Decode(&status); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if status.Version != version.Version {
		t.Fatalf("version = %q", status.Version)
	}
	if status.Status != "ok" {
		t.Fatalf("status = %q", status.Status)
	}
	if status.Uptime == "" {
		t.Fatal("uptime is empty")
	}
	if len(status.Providers) != 1 {
		t.Fatalf("providers = %d, want 1", len(status.Providers))
	}
	if status.Providers[0].Name != "p1" {
		t.Fatalf("provider name = %q", status.Providers[0].Name)
	}
	if status.Autonomy == nil {
		t.Fatal("expected autonomy")
	}
	if status.RAG == nil {
		t.Fatal("expected RAG")
	}
	if status.RAG.Dimensions != 256 {
		t.Fatalf("RAG dims = %d", status.RAG.Dimensions)
	}
	if status.Vault == nil {
		t.Fatal("expected vault")
	}
	if status.Cache == nil {
		t.Fatal("expected cache")
	}
	if status.Requests != 1 {
		t.Fatalf("requests = %d, want 1", status.Requests)
	}
}

func TestStatusEndpoint_RequestCounterIncrements(t *testing.T) {
	s := NewServer(
		&config.WebUIConfig{Host: "127.0.0.1", Port: 8080},
		nil, nil, nil, nil, nil, nil, nil,
	)

	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
		w := httptest.NewRecorder()
		s.HandleStatus(w, req)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	w := httptest.NewRecorder()
	s.HandleStatus(w, req)

	var status StatusResponse
	json.NewDecoder(w.Body).Decode(&status)
	if status.Requests != 6 {
		t.Fatalf("requests = %d, want 6", status.Requests)
	}
}

// ─── Providers ──────────────────────────────────────────────────────

func TestProvidersEndpoint_EmptyWithoutRouter(t *testing.T) {
	s := NewServer(
		&config.WebUIConfig{Host: "127.0.0.1", Port: 8080},
		nil, nil, nil, nil, nil, nil, nil,
	)

	req := httptest.NewRequest(http.MethodGet, "/api/providers", nil)
	w := httptest.NewRecorder()
	s.HandleProviders(w, req)

	var providers []providerStatus
	if err := json.NewDecoder(w.Body).Decode(&providers); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if providers == nil {
		t.Fatal("expected empty slice, not nil")
	}
	if len(providers) != 0 {
		t.Fatalf("len = %d, want 0", len(providers))
	}
}

func TestProvidersEndpoint_WithCostData(t *testing.T) {
	providers := []router.ProviderEntry{
		{Name: "costly", Model: "gpt-4o"},
	}
	r := router.NewRouter(providers)
	r.CostTracker().Add("gpt-4o", 1000, 200)

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
	if len(result) != 1 {
		t.Fatalf("len = %d, want 1", len(result))
	}
	if result[0].Cost <= 0 {
		t.Fatalf("expected positive cost, got %f", result[0].Cost)
	}
}

// ─── WebSocket Hub ─────────────────────────────────────────────────

func TestWebSocketHub_New(t *testing.T) {
	hub := NewWebSocketHub()
	if hub == nil {
		t.Fatal("expected non-nil hub")
	}
	if hub.clients == nil {
		t.Fatal("expected initialized clients map")
	}
}

func TestWebSocketHub_AddAndRemove(t *testing.T) {
	hub := NewWebSocketHub()
	s := NewServer(
		&config.WebUIConfig{Host: "127.0.0.1", Port: 8080},
		nil, nil, nil, nil, nil, nil, hub,
	)

	ts := httptest.NewServer(
		websocket.Handler(func(ws *websocket.Conn) {
			s.handleWebSocket(ws)
		}),
	)
	defer ts.Close()

	tsURL := strings.Replace(ts.URL, "http://", "ws://", 1)
	ws, err := websocket.Dial(tsURL, "", tsURL)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer ws.Close()

	discardFirstWSMessage(ws)

	hub.mu.RLock()
	count := len(hub.clients)
	hub.mu.RUnlock()
	if count == 0 {
		t.Fatal("expected at least one connected client")
	}
}

func TestWebSocketHub_BroadcastEmpty(t *testing.T) {
	hub := NewWebSocketHub()
	hub.Broadcast(map[string]string{"type": "test"})
}

func TestWebSocketHub_RemoveNonExistent(t *testing.T) {
	hub := NewWebSocketHub()
	hub.Remove(nil)
}

// ─── handleWebSocket ───────────────────────────────────────────────

func TestWebSocketHandler_PingPong(t *testing.T) {
	s := NewServer(
		&config.WebUIConfig{Host: "127.0.0.1", Port: 8080},
		nil, nil, nil, nil, nil, nil, nil,
	)

	ts := httptest.NewServer(
		websocket.Handler(func(ws *websocket.Conn) {
			s.handleWebSocket(ws)
		}),
	)
	defer ts.Close()

	tsURL := strings.Replace(ts.URL, "http://", "ws://", 1)
	ws, err := websocket.Dial(tsURL, "", tsURL)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer ws.Close()

	discardFirstWSMessage(ws)

	if err := websocket.Message.Send(ws, `{"method":"ping"}`); err != nil {
		t.Fatalf("send ping: %v", err)
	}

	var resp string
	if err := websocket.Message.Receive(ws, &resp); err != nil {
		t.Fatalf("receive pong: %v", err)
	}

	var decoded map[string]string
	if err := json.Unmarshal([]byte(resp), &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded["type"] != "pong" {
		t.Fatalf("type = %q, want %q", decoded["type"], "pong")
	}
}

func TestWebSocketHandler_GetStatus(t *testing.T) {
	providers := []router.ProviderEntry{
		{Name: "ws-test", Model: "gpt-4o"},
	}
	r := router.NewRouter(providers)
	s := NewServer(
		&config.WebUIConfig{Host: "127.0.0.1", Port: 8080},
		r, nil, nil, nil, nil, nil, nil,
	)

	ts := httptest.NewServer(
		websocket.Handler(func(ws *websocket.Conn) {
			s.handleWebSocket(ws)
		}),
	)
	defer ts.Close()

	tsURL := strings.Replace(ts.URL, "http://", "ws://", 1)
	ws, err := websocket.Dial(tsURL, "", tsURL)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer ws.Close()

	discardFirstWSMessage(ws)

	if err := websocket.Message.Send(ws, `{"method":"get_status"}`); err != nil {
		t.Fatalf("send get_status: %v", err)
	}

	var resp string
	if err := websocket.Message.Receive(ws, &resp); err != nil {
		t.Fatalf("receive status: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(resp), &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded["type"] != "status" {
		t.Fatalf("type = %v, want %q", decoded["type"], "status")
	}
	if _, ok := decoded["uptime"]; !ok {
		t.Fatal("missing uptime in status response")
	}
	if _, ok := decoded["cost"]; !ok {
		t.Fatal("missing cost in status response")
	}
}

func TestWebSocketHandler_GetStatusWithAutonomy(t *testing.T) {
	auto := autonomy.New(
		&autonomy.AutonomyConfig{Enabled: true},
		".",
		nil,
	)
	s := NewServer(
		&config.WebUIConfig{Host: "127.0.0.1", Port: 8080},
		nil, auto, nil, nil, nil, nil, nil,
	)

	ts := httptest.NewServer(
		websocket.Handler(func(ws *websocket.Conn) {
			s.handleWebSocket(ws)
		}),
	)
	defer ts.Close()

	tsURL := strings.Replace(ts.URL, "http://", "ws://", 1)
	ws, err := websocket.Dial(tsURL, "", tsURL)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer ws.Close()

	discardFirstWSMessage(ws)

	if err := websocket.Message.Send(ws, `{"method":"get_status"}`); err != nil {
		t.Fatalf("send get_status: %v", err)
	}

	var resp string
	if err := websocket.Message.Receive(ws, &resp); err != nil {
		t.Fatalf("receive status: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(resp), &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := decoded["autonomy"]; !ok {
		t.Fatal("missing autonomy in WS status")
	}
}

func TestWebSocketHandler_SubscribeLogs(t *testing.T) {
	s := NewServer(
		&config.WebUIConfig{Host: "127.0.0.1", Port: 8080},
		nil, nil, nil, nil, nil, nil, nil,
	)

	ts := httptest.NewServer(
		websocket.Handler(func(ws *websocket.Conn) {
			s.handleWebSocket(ws)
		}),
	)
	defer ts.Close()

	tsURL := strings.Replace(ts.URL, "http://", "ws://", 1)
	ws, err := websocket.Dial(tsURL, "", tsURL)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer ws.Close()

	discardFirstWSMessage(ws)

	if err := websocket.Message.Send(ws, `{"method":"subscribe_logs"}`); err != nil {
		t.Fatalf("send subscribe_logs: %v", err)
	}

	for i := 0; i < 6; i++ {
		var resp string
		if err := websocket.Message.Receive(ws, &resp); err != nil {
			t.Fatalf("receive log %d: %v", i, err)
		}
		var log logEntry
		if err := json.Unmarshal([]byte(resp), &log); err != nil {
			t.Fatalf("unmarshal log %d: %v", i, err)
		}
		if log.Level == "" || log.Message == "" {
			t.Fatalf("log %d has empty fields", i)
		}
	}
}

func TestWebSocketHandler_UnknownMethodIgnored(t *testing.T) {
	s := NewServer(
		&config.WebUIConfig{Host: "127.0.0.1", Port: 8080},
		nil, nil, nil, nil, nil, nil, nil,
	)

	ts := httptest.NewServer(
		websocket.Handler(func(ws *websocket.Conn) {
			s.handleWebSocket(ws)
		}),
	)
	defer ts.Close()

	tsURL := strings.Replace(ts.URL, "http://", "ws://", 1)
	ws, err := websocket.Dial(tsURL, "", tsURL)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer ws.Close()

	discardFirstWSMessage(ws)

	if err := websocket.Message.Send(ws, `{"method":"unknown"}`); err != nil {
		t.Fatalf("send: %v", err)
	}

	if err := websocket.Message.Send(ws, `{"method":"ping"}`); err != nil {
		t.Fatalf("send ping: %v", err)
	}

	var resp string
	if err := websocket.Message.Receive(ws, &resp); err != nil {
		t.Fatalf("receive: %v", err)
	}
	var decoded map[string]string
	json.Unmarshal([]byte(resp), &decoded)
	if decoded["type"] != "pong" {
		t.Fatalf("expected pong after unknown method, got %v", decoded)
	}
}

func TestWebSocketHandler_InvalidJSONIgnored(t *testing.T) {
	s := NewServer(
		&config.WebUIConfig{Host: "127.0.0.1", Port: 8080},
		nil, nil, nil, nil, nil, nil, nil,
	)

	ts := httptest.NewServer(
		websocket.Handler(func(ws *websocket.Conn) {
			s.handleWebSocket(ws)
		}),
	)
	defer ts.Close()

	tsURL := strings.Replace(ts.URL, "http://", "ws://", 1)
	ws, err := websocket.Dial(tsURL, "", tsURL)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer ws.Close()

	discardFirstWSMessage(ws)

	if err := websocket.Message.Send(ws, `not json`); err != nil {
		t.Fatalf("send: %v", err)
	}

	if err := websocket.Message.Send(ws, `{"method":"ping"}`); err != nil {
		t.Fatalf("send ping: %v", err)
	}

	var resp string
	if err := websocket.Message.Receive(ws, &resp); err != nil {
		t.Fatalf("receive: %v", err)
	}
	var decoded map[string]string
	json.Unmarshal([]byte(resp), &decoded)
	if decoded["type"] != "pong" {
		t.Fatalf("expected pong after invalid JSON, got %v", decoded)
	}
}

func TestWebSocketHandler_EmptyMethodIgnored(t *testing.T) {
	s := NewServer(
		&config.WebUIConfig{Host: "127.0.0.1", Port: 8080},
		nil, nil, nil, nil, nil, nil, nil,
	)

	ts := httptest.NewServer(
		websocket.Handler(func(ws *websocket.Conn) {
			s.handleWebSocket(ws)
		}),
	)
	defer ts.Close()

	tsURL := strings.Replace(ts.URL, "http://", "ws://", 1)
	ws, err := websocket.Dial(tsURL, "", tsURL)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer ws.Close()

	discardFirstWSMessage(ws)

	if err := websocket.Message.Send(ws, `{"foo":"bar"}`); err != nil {
		t.Fatalf("send: %v", err)
	}

	if err := websocket.Message.Send(ws, `{"method":"ping"}`); err != nil {
		t.Fatalf("send ping: %v", err)
	}

	var resp string
	if err := websocket.Message.Receive(ws, &resp); err != nil {
		t.Fatalf("receive: %v", err)
	}
	var decoded map[string]string
	json.Unmarshal([]byte(resp), &decoded)
	if decoded["type"] != "pong" {
		t.Fatalf("expected pong after empty method, got %v", decoded)
	}
}

// ─── sendStatusViaWS ───────────────────────────────────────────────

func TestSendStatusViaWS_WithRouter(t *testing.T) {
	providers := []router.ProviderEntry{
		{Name: "sp", Model: "gpt-4o"},
	}
	r := router.NewRouter(providers)
	s := NewServer(
		&config.WebUIConfig{Host: "127.0.0.1", Port: 8080},
		r, nil, nil, nil, nil, nil, nil,
	)

	ts := httptest.NewServer(
		websocket.Handler(func(ws *websocket.Conn) {
			s.sendStatusViaWS(ws)
		}),
	)
	defer ts.Close()

	tsURL := strings.Replace(ts.URL, "http://", "ws://", 1)
	ws, err := websocket.Dial(tsURL, "", tsURL)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer ws.Close()

	var resp string
	if err := websocket.Message.Receive(ws, &resp); err != nil {
		t.Fatalf("receive: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(resp), &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := decoded["cost"]; !ok {
		t.Fatal("expected cost field")
	}
}

func TestSendStatusViaWS_WithAutonomy(t *testing.T) {
	auto := autonomy.New(
		&autonomy.AutonomyConfig{Enabled: true},
		".",
		nil,
	)
	s := NewServer(
		&config.WebUIConfig{Host: "127.0.0.1", Port: 8080},
		nil, auto, nil, nil, nil, nil, nil,
	)

	ts := httptest.NewServer(
		websocket.Handler(func(ws *websocket.Conn) {
			s.sendStatusViaWS(ws)
		}),
	)
	defer ts.Close()

	tsURL := strings.Replace(ts.URL, "http://", "ws://", 1)
	ws, err := websocket.Dial(tsURL, "", tsURL)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer ws.Close()

	var resp string
	if err := websocket.Message.Receive(ws, &resp); err != nil {
		t.Fatalf("receive: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(resp), &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := decoded["autonomy"]; !ok {
		t.Fatal("expected autonomy field in WS status")
	}
}

// ─── BroadcastEvent ────────────────────────────────────────────────

func TestBroadcastEvent_NoClients(t *testing.T) {
	s := NewServer(
		&config.WebUIConfig{Host: "127.0.0.1", Port: 8080},
		nil, nil, nil, nil, nil, nil, nil,
	)
	s.BroadcastEvent("noop", "data")
}

func TestBroadcastEvent_ToSingleClient(t *testing.T) {
	s := NewServer(
		&config.WebUIConfig{Host: "127.0.0.1", Port: 8080},
		nil, nil, nil, nil, nil, nil, nil,
	)

	ts := httptest.NewServer(
		websocket.Handler(func(ws *websocket.Conn) {
			s.handleWebSocket(ws)
		}),
	)
	defer ts.Close()

	tsURL := strings.Replace(ts.URL, "http://", "ws://", 1)
	ws, err := websocket.Dial(tsURL, "", tsURL)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer ws.Close()

	discardFirstWSMessage(ws)

	s.BroadcastEvent("my_event", map[string]string{"k": "v"})

	var resp string
	if err := websocket.Message.Receive(ws, &resp); err != nil {
		t.Fatalf("receive: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(resp), &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded["event"] != "my_event" {
		t.Fatalf("event = %v", decoded["event"])
	}
}

// ─── NewServer edge cases ──────────────────────────────────────────

func TestNewServer_WithCustomHub(t *testing.T) {
	hub := NewWebSocketHub()
	s := NewServer(
		&config.WebUIConfig{Host: "0.0.0.0", Port: 7070},
		nil, nil, nil, nil, nil, nil, hub,
	)
	if s.hub != hub {
		t.Fatal("expected custom hub to be used")
	}
}

func TestNewServer_NilHubGetsDefault(t *testing.T) {
	s := NewServer(
		&config.WebUIConfig{Host: "0.0.0.0", Port: 6060},
		nil, nil, nil, nil, nil, nil, nil,
	)
	if s.hub == nil {
		t.Fatal("expected default hub")
	}
	if s.hub.clients == nil {
		t.Fatal("expected initialized clients map")
	}
}

func TestNewServer_StatsInitialized(t *testing.T) {
	s := NewServer(
		&config.WebUIConfig{Host: "127.0.0.1", Port: 8080},
		nil, nil, nil, nil, nil, nil, nil,
	)
	if s.stats.startedAt.IsZero() {
		t.Fatal("expected startedAt to be set")
	}
}

// ─── Health endpoint ───────────────────────────────────────────────

func TestHandleHealth_JSONResponse(t *testing.T) {
	s := NewServer(
		&config.WebUIConfig{Host: "127.0.0.1", Port: 8080},
		nil, nil, nil, nil, nil, nil, nil,
	)

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	w := httptest.NewRecorder()
	s.HandleHealth(w, req)

	var body map[string]string
	json.NewDecoder(w.Body).Decode(&body)
	if body["status"] != "ok" {
		t.Fatalf("status = %q", body["status"])
	}
}

func TestHandleHealth_ContentType(t *testing.T) {
	s := NewServer(
		&config.WebUIConfig{Host: "127.0.0.1", Port: 8080},
		nil, nil, nil, nil, nil, nil, nil,
	)

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	w := httptest.NewRecorder()
	s.HandleHealth(w, req)

	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q", ct)
	}
}

func TestHandleHealth_DoesNotIncrementCount(t *testing.T) {
	s := NewServer(
		&config.WebUIConfig{Host: "127.0.0.1", Port: 8080},
		nil, nil, nil, nil, nil, nil, nil,
	)

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
		w := httptest.NewRecorder()
		s.HandleHealth(w, req)
	}

	s.stats.mu.RLock()
	count := s.stats.requests
	s.stats.mu.RUnlock()
	if count != 0 {
		t.Fatalf("health should not increment request count, got %d", count)
	}
}

// ─── Config endpoint ────────────────────────────────────────────────

func TestConfigEndpoint_JSONFormat(t *testing.T) {
	cfg := &config.WebUIConfig{Host: "1.2.3.4", Port: 9999}
	s := NewServer(cfg, nil, nil, nil, nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	w := httptest.NewRecorder()
	s.HandleConfig(w, req)

	resp := w.Result()
	ct := resp.Header.Get("Content-Type")
	if ct != "application/json" {
		t.Fatalf("Content-Type = %q, want %q", ct, "application/json")
	}

	var returned config.WebUIConfig
	if err := json.NewDecoder(resp.Body).Decode(&returned); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if returned.Port != 9999 {
		t.Fatalf("port = %d, want 9999", returned.Port)
	}
}

func TestConfigEndpoint_EmptyHost(t *testing.T) {
	cfg := &config.WebUIConfig{Host: "", Port: 0}
	s := NewServer(cfg, nil, nil, nil, nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	w := httptest.NewRecorder()
	s.HandleConfig(w, req)

	var returned config.WebUIConfig
	if err := json.NewDecoder(w.Body).Decode(&returned); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if returned.Host != "" {
		t.Fatalf("host = %q, want empty", returned.Host)
	}
	if returned.Port != 0 {
		t.Fatalf("port = %d, want 0", returned.Port)
	}
}

// ─── Start ─────────────────────────────────────────────────────────

func TestStart_InvalidAddress(t *testing.T) {
	s := NewServer(
		&config.WebUIConfig{Host: "", Port: -1},
		nil, nil, nil, nil, nil, nil, nil,
	)
	err := s.Start()
	if err == nil {
		t.Fatal("expected error for invalid address")
	}
}

// ─── findWebDist ───────────────────────────────────────────────────

func TestFindWebDist_Found(t *testing.T) {
	result := findWebDist()
	if result == "" {
		t.Skip("web/dist not found — skipping in CI or when web app is not built")
	}
	if !strings.HasSuffix(result, "web/dist") {
		t.Fatalf("expected path ending in web/dist, got %q", result)
	}
}

func TestFindWebDist_FoundInCwd(t *testing.T) {
	origDir, _ := os.Getwd()
	tmpDir := t.TempDir()

	webDir := filepath.Join(tmpDir, "web", "dist")
	if err := os.MkdirAll(webDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	index := filepath.Join(webDir, "index.html")
	if err := os.WriteFile(index, []byte("<html></html>"), 0644); err != nil {
		t.Fatalf("write index.html: %v", err)
	}

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	result := findWebDist()
	if result == "" {
		t.Fatal("expected non-empty path")
	}
	if !strings.HasSuffix(result, "web/dist") {
		t.Fatalf("expected path ending in web/dist, got %q", result)
	}
}
