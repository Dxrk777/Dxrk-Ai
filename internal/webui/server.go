// SPDX-License-Identifier: MIT
package webui

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Dxrk777/Dxrk/internal/autonomy"
	"github.com/Dxrk777/Dxrk/internal/config"
	"github.com/Dxrk777/Dxrk/internal/pipeline"
	"github.com/Dxrk777/Dxrk/internal/rag"
	"github.com/Dxrk777/Dxrk/internal/router"
	"github.com/Dxrk777/Dxrk/internal/strconst"
	"github.com/Dxrk777/Dxrk/internal/tools"
	"github.com/Dxrk777/Dxrk/internal/vault"
	"github.com/Dxrk777/Dxrk/internal/version"

	"golang.org/x/net/websocket"
)

const statusRunning = strconst.StrRunning

// serverPipeline is the minimal pipeline surface consumed by the webui.
type serverPipeline interface {
	LastResult() pipeline.PipelineResult
}

type Server struct {
	config   *config.WebUIConfig
	router   *router.Router
	autonomy *autonomy.Autonomy
	rag      *rag.RAG
	vault    *vault.Vault
	cache    *router.SemanticCache
	pipeline serverPipeline

	tools   *tools.Registry
	checker tools.PermissionChecker
	audit   tools.PermissionAudit

	hub *WebSocketHub

	stats struct {
		mu        sync.RWMutex
		startedAt time.Time
		requests  int
	}
}

type WebSocketHub struct {
	mu      sync.RWMutex
	clients map[*websocket.Conn]bool
}

func NewWebSocketHub() *WebSocketHub {
	return &WebSocketHub{clients: make(map[*websocket.Conn]bool)}
}

func (h *WebSocketHub) Add(ws *websocket.Conn) {
	h.mu.Lock()
	h.clients[ws] = true
	h.mu.Unlock()
}

func (h *WebSocketHub) Remove(ws *websocket.Conn) {
	h.mu.Lock()
	delete(h.clients, ws)
	h.mu.Unlock()
}

func (h *WebSocketHub) Broadcast(msg any) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	for ws := range h.clients {
		_ = websocket.Message.Send(ws, string(data))
	}
}

type StatusResponse struct {
	Version   string           `json:"version"`
	Uptime    string           `json:"uptime"`
	Status    string           `json:"status"`
	Providers []providerStatus `json:"providers"`
	Autonomy  *autonomyStatus  `json:"autonomy,omitempty"`
	RAG       *ragStatus       `json:"rag,omitempty"`
	Vault     *vaultStatus     `json:"vault,omitempty"`
	Cache     *cacheStatus     `json:"cache,omitempty"`
	Pipeline  *pipelineStatus  `json:"pipeline,omitempty"`
	Requests  int              `json:"requests"`
}

type providerStatus struct {
	Name    string  `json:"name"`
	Model   string  `json:"model"`
	Healthy bool    `json:"healthy"`
	Cost    float64 `json:"cost"`
}

type autonomyStatus struct {
	Enabled     bool    `json:"enabled"`
	IQ          float64 `json:"iq"`
	Patterns    int     `json:"patterns"`
	Permissions int     `json:"permissions"`
	Updating    bool    `json:"updating"`
	Verifying   bool    `json:"verifying"`
	Learning    bool    `json:"learning"`
	Evolving    bool    `json:"evolving"`
}

type ragStatus struct {
	Enabled      bool   `json:"enabled"`
	Vectors      int    `json:"vectors"`
	Dimensions   int    `json:"dimensions"`
	FilesIndexed int    `json:"files_indexed"`
	LastIndex    string `json:"last_index,omitempty"`
}

type vaultStatus struct {
	Enabled     bool     `json:"enabled"`
	Secrets     int      `json:"secrets"`
	SecretNames []string `json:"secret_names,omitempty"`
}

type cacheStatus struct {
	Enabled bool   `json:"enabled"`
	Size    int    `json:"size"`
	MaxSize int    `json:"max_size"`
	Hits    int    `json:"hits"`
	TTL     string `json:"ttl"`
}

type pipelineStatus struct {
	Running    bool   `json:"running"`
	LastRun    string `json:"last_run,omitempty"`
	Iterations int    `json:"iterations"`
	Success    bool   `json:"success"`
}

func NewServer(
	cfg *config.WebUIConfig,
	rtr *router.Router,
	auto *autonomy.Autonomy,
	ragInstance *rag.RAG,
	vaultInstance *vault.Vault,
	cacheInstance *router.SemanticCache,
	pipelineInstance serverPipeline,
	hub *WebSocketHub,
) *Server {
	if hub == nil {
		hub = NewWebSocketHub()
	}
	s := &Server{
		config:   cfg,
		router:   rtr,
		autonomy: auto,
		rag:      ragInstance,
		vault:    vaultInstance,
		cache:    cacheInstance,
		pipeline: pipelineInstance,
		hub:      hub,
	}
	s.stats.startedAt = time.Now()
	return s
}

// WithTools attaches a tools.Registry so the /api/tools endpoint can be served.
func (s *Server) WithTools(reg *tools.Registry) *Server {
	s.tools = reg
	return s
}

// WithPermissions attaches a permission checker and optional audit logger.
// When set, config/settings endpoints enforce read/write grants.
func (s *Server) WithPermissions(checker tools.PermissionChecker, audit tools.PermissionAudit) *Server {
	s.checker = checker
	s.audit = audit
	return s
}

func (s *Server) Start() error {
	addr := s.config.Host + ":" + itoa(s.config.Port)

	mux := http.NewServeMux()

	mux.HandleFunc("/api/status", s.HandleStatus)
	mux.HandleFunc("/api/health", s.HandleHealth)
	mux.HandleFunc("/api/config", s.HandleConfig)
	mux.HandleFunc("/api/providers", s.HandleProviders)
	mux.HandleFunc("/api/settings", s.HandleSettings)
	mux.HandleFunc("/api/tools", s.HandleTools)
	mux.Handle("/ws", websocket.Handler(s.handleWebSocket))

	webDist := findWebDist()
	if webDist != "" {
		fs := http.FileServer(http.Dir(webDist))
		mux.Handle("/", fs)
	} else {
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{
				"name":              "Dxrk.ai API",
				strconst.StrVersion: version.Version,
				strconst.StrStatus:  statusRunning, //nolint:goconst
			})
		})
	}

	log.Printf("[webui] listening on http://%s", addr)
	return http.ListenAndServe(addr, withMiddleware(mux)) //nolint:gosec
}

func (s *Server) HandleStatus(w http.ResponseWriter, r *http.Request) {
	s.stats.mu.Lock()
	s.stats.requests++
	s.stats.mu.Unlock()

	s.stats.mu.RLock()
	uptime := time.Since(s.stats.startedAt).String()
	requests := s.stats.requests
	s.stats.mu.RUnlock()

	resp := StatusResponse{
		Version:  version.Version,
		Uptime:   uptime,
		Status:   "ok",
		Requests: requests,
	}

	if s.router != nil {
		ct := s.router.CostTracker()
		costs := ct.ByModel()
		for _, p := range s.router.Providers() {
			providerCost := costs[p.Model]
			resp.Providers = append(resp.Providers, providerStatus{
				Name:    p.Name,
				Model:   p.Model,
				Healthy: true,
				Cost:    providerCost,
			})
		}
	}

	if s.autonomy != nil {
		iq := s.autonomy.CurrentIQ()
		perms := s.autonomy.Permissions.AllGranted()
		resp.Autonomy = &autonomyStatus{
			Enabled:     s.autonomy.Config.Enabled,
			IQ:          iq.OverallIQ,
			Patterns:    0,
			Permissions: len(perms),
			Updating:    false,
			Verifying:   false,
			Learning:    false,
			Evolving:    s.autonomy.Evolution != nil && s.autonomy.Config.Evolution,
		}
	}

	if s.rag != nil {
		count, dims := s.rag.Store.Stats()
		resp.RAG = &ragStatus{
			Enabled:    s.rag.IsEnabled(),
			Vectors:    count,
			Dimensions: dims,
		}
	}

	if s.vault != nil {
		names := s.vault.List()
		resp.Vault = &vaultStatus{
			Enabled:     true,
			Secrets:     len(names),
			SecretNames: names,
		}
	}

	if s.cache != nil {
		st := s.cache.Stats()
		resp.Cache = &cacheStatus{
			Enabled: true,
			Size:    st.Size,
			MaxSize: st.MaxSize,
			Hits:    st.Hits,
			TTL:     st.TTL.String(),
		}
	}

	if s.pipeline != nil {
		res := s.pipeline.LastResult()
		resp.Pipeline = &pipelineStatus{
			Running:    false,
			LastRun:    res.TaskID,
			Iterations: res.Iterations,
			Success:    res.Success,
		}
	}

	writeJSON(w, resp)
}

func (s *Server) HandleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]string{strconst.StrStatus: "ok"}) //nolint:goconst
}

func (s *Server) HandleConfig(w http.ResponseWriter, r *http.Request) {
	if !s.authorize(w, r, "config.read") {
		return
	}
	writeJSON(w, s.config)
}

func (s *Server) HandleProviders(w http.ResponseWriter, r *http.Request) {
	if s.router == nil {
		writeJSON(w, []providerStatus{})
		return
	}
	ct := s.router.CostTracker()
	costs := ct.ByModel()
	var providers []providerStatus
	for _, p := range s.router.Providers() {
		providers = append(providers, providerStatus{
			Name:    p.Name,
			Model:   p.Model,
			Healthy: true,
			Cost:    costs[p.Model],
		})
	}
	writeJSON(w, providers)
}

type toolInfo struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Enabled     bool           `json:"enabled"`
	ReadOnly    bool           `json:"read_only"`
	InputSchema map[string]any `json:"input_schema,omitempty"`
}

// HandleTools exposes the enabled tools registered in the attached registry.
func (s *Server) HandleTools(w http.ResponseWriter, r *http.Request) {
	if !s.authorize(w, r, "tools.list") {
		return
	}
	if s.tools == nil {
		writeJSON(w, []toolInfo{})
		return
	}
	list := s.tools.ListEnabled()
	infos := make([]toolInfo, 0, len(list))
	for _, t := range list {
		infos = append(infos, toolInfo{
			Name:        t.Name(),
			Description: t.Description(),
			Enabled:     t.IsEnabled(),
			ReadOnly:    t.IsReadOnly(),
			InputSchema: t.InputSchema(),
		})
	}
	writeJSON(w, infos)
}

// authorize enforces the attached permission checker and records the decision.
// With no checker attached the request is always allowed (backwards compatible).
func (s *Server) authorize(w http.ResponseWriter, r *http.Request, action string) bool {
	if s.checker == nil {
		return true
	}
	target := r.URL.Path
	allowed, reason := s.checker.Check(action, target)
	if s.audit != nil {
		s.audit.Record(action, target, allowed, reason)
	}
	if !allowed {
		w.WriteHeader(http.StatusForbidden)
		writeJSON(w, map[string]string{"error": reason})
		return false
	}
	return true
}

func (s *Server) HandleSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		if !s.authorize(w, r, "config.read") {
			return
		}
		writeJSON(w, s.config)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	if !s.authorize(w, r, "config.write") {
		return
	}

	var updates map[string]any
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
		return
	}

	if v, ok := updates["theme"].(string); ok {
		s.config.Theme = v
	}
	if v, ok := updates["log_level"].(string); ok {
		s.config.LogLevel = v
	}
	if v, ok := updates["auto_update"].(bool); ok {
		s.config.AutoUpdate = v
	}

	writeJSON(w, map[string]string{strconst.StrStatus: "ok"})
}

func (s *Server) handleWebSocket(ws *websocket.Conn) {
	s.hub.Add(ws)
	defer s.hub.Remove(ws)

	s.hub.Broadcast(map[string]string{"type": "connected", "message": "client joined"})

	for {
		var msg string
		if err := websocket.Message.Receive(ws, &msg); err != nil {
			break
		}

		var req map[string]any
		if err := json.Unmarshal([]byte(msg), &req); err != nil {
			continue
		}

		method, _ := req["method"].(string)
		switch method {
		case "ping":
			_ = websocket.Message.Send(ws, `{"type":"pong"}`)
		case "get_status":
			s.sendStatusViaWS(ws)
		case "subscribe_logs":
			s.streamLogs(ws)
		}
	}
}

func (s *Server) sendStatusViaWS(ws *websocket.Conn) {
	resp := map[string]any{
		"type":   strconst.StrStatus, //nolint:goconst
		"uptime": time.Since(s.stats.startedAt).String(),
	}
	if s.autonomy != nil {
		resp["autonomy"] = map[string]any{
			strconst.StrEnabled: s.autonomy.Config.Enabled,
			"iq":                s.autonomy.CurrentIQ().OverallIQ,
		}
	}
	if s.router != nil {
		resp["cost"] = s.router.CostTracker().Total()
	}
	data, _ := json.Marshal(resp)
	_ = websocket.Message.Send(ws, string(data))
}

func (s *Server) streamLogs(ws *websocket.Conn) {
	entries := generateMockLogs()
	for _, entry := range entries {
		data, _ := json.Marshal(entry)
		_ = websocket.Message.Send(ws, string(data))
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func itoa(n int) string {
	if n == 0 {
		return "8080"
	}
	return fmt.Sprintf("%d", n)
}

func withMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, PUT")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("X-Dxrk-Version", version.Version)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func findWebDist() string {
	candidates := []string{
		"web/dist",
		"../web/dist",
		filepath.Join(os.Getenv("HOME"), "Proyectos", "Dxrk", "web", "dist"),
	}
	for _, c := range candidates {
		abs, err := filepath.Abs(c)
		if err != nil {
			continue
		}
		if info, err := os.Stat(abs); err == nil && info.IsDir() {
			if _, err := os.Stat(filepath.Join(abs, "index.html")); err == nil {
				return abs
			}
		}
	}
	return ""
}

type logEntry struct {
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	Source    string `json:"source"`
	Message   string `json:"message"`
}

func generateMockLogs() []logEntry {
	now := time.Now()
	return []logEntry{
		{Timestamp: now.Format(time.RFC3339), Level: "INFO", Source: strconst.StrSystem, Message: "Web UI server started"},
		{Timestamp: now.Add(-1 * time.Second).Format(time.RFC3339), Level: "INFO", Source: "router", Message: "Provider claude/gpt-4o initialized"},
		{Timestamp: now.Add(-2 * time.Second).Format(time.RFC3339), Level: "WARN", Source: "sandbox", Message: "Container pool at 80% capacity"},
		{Timestamp: now.Add(-3 * time.Second).Format(time.RFC3339), Level: "INFO", Source: "autonomy", Message: "IQ report: score 85.3, 12 patterns learned"},
		{Timestamp: now.Add(-4 * time.Second).Format(time.RFC3339), Level: "ERROR", Source: "router", Message: "Provider gemini/gemini-2.0-flash timeout, falling back"},
		{Timestamp: now.Add(-5 * time.Second).Format(time.RFC3339), Level: "INFO", Source: "rag", Message: "Indexed 47 files, 312 chunks created"},
	}
}

func (s *Server) BroadcastEvent(event string, data any) {
	msg := map[string]any{"event": event, "data": data}
	s.hub.Broadcast(msg)
}
