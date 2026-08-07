// SPDX-License-Identifier: MIT
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/Dxrk777/Dxrk/internal/log"
	"github.com/Dxrk777/Dxrk/internal/strconst"
	"github.com/Dxrk777/Dxrk/internal/task"
	"github.com/Dxrk777/Dxrk/internal/tools"
	"github.com/Dxrk777/Dxrk/internal/trace"
	"github.com/Dxrk777/Dxrk/internal/version"

	"go.opentelemetry.io/otel/attribute"
)

// RateLimiter implements a simple token-bucket rate limiter.
type RateLimiter struct {
	mu     sync.Mutex
	tokens float64
	max    float64
	refill float64 // tokens per second
	last   time.Time
}

// NewRateLimiter creates a rate limiter with given capacity and refill rate.
func NewRateLimiter(maxTokens, refillPerSec float64) *RateLimiter {
	return &RateLimiter{
		tokens: maxTokens,
		max:    maxTokens,
		refill: refillPerSec,
		last:   time.Now(),
	}
}

// Allow checks if an action is allowed, consuming one token.
func (rl *RateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(rl.last).Seconds()
	rl.tokens = min(rl.max, rl.tokens+elapsed*rl.refill)
	rl.last = now

	if rl.tokens >= 1 {
		rl.tokens--
		return true
	}
	return false
}

// Tokens returns the current number of available tokens.
func (rl *RateLimiter) Tokens() float64 {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	elapsed := now.Sub(rl.last).Seconds()
	rl.tokens = min(rl.max, rl.tokens+elapsed*rl.refill)
	rl.last = now
	return rl.tokens
}

// Server is an MCP server that delegates tool calls to a tools.Registry.
type Server struct {
	registry       *tools.Registry
	logger         log.Logger
	reader         io.Reader
	writer         io.Writer
	mu             sync.Mutex
	toolCtx        tools.Context
	rateLimiter    *RateLimiter
	workerPool     *task.WorkerPool
	watchdogCh     chan struct{}
	lastActivity   time.Time
	metrics        MetricsExporter
	metricsHandler func(http.ResponseWriter, *http.Request)
	tp             trace.Exporter
}

// ServerOption configures the MCP server.
type ServerOption func(*Server)

// ServerWithLogger sets the logger for the MCP server.
func ServerWithLogger(logger log.Logger) ServerOption {
	return func(s *Server) { s.logger = logger }
}

// ServerWithPermissionChecker sets the permission checker on the tool context.
func ServerWithPermissionChecker(checker tools.PermissionChecker, audit tools.PermissionAudit) ServerOption {
	return func(s *Server) {
		s.toolCtx.PermissionChecker = checker
		s.toolCtx.PermissionAudit = audit
	}
}

// ServerWithRateLimiter adds a rate limiter that limits tool calls per second.
func ServerWithRateLimiter(maxTokens, refillPerSec float64) ServerOption {
	return func(s *Server) {
		s.rateLimiter = NewRateLimiter(maxTokens, refillPerSec)
	}
}

// ServerWithMetrics sets the metrics exporter for the server.
func ServerWithMetrics(exporter MetricsExporter) ServerOption {
	return func(s *Server) { s.metrics = exporter }
}

// ServerHandleMetrics sets an external HTTP handler for serving metrics,
// allowing integration with existing HTTP servers.
func ServerHandleMetrics(handler func(http.ResponseWriter, *http.Request)) ServerOption {
	return func(s *Server) { s.metricsHandler = handler }
}

// ServerWithWorkerPool sets a worker pool for async tool execution.
func ServerWithWorkerPool(pool *task.WorkerPool) ServerOption {
	return func(s *Server) {
		s.workerPool = pool
	}
}

// ServerWithWatchdog enables a health-check watchdog that pings the server
// at the given interval and signals on the returned channel if the server stalls.
// The caller can select on the channel to trigger restart logic.
func ServerWithWatchdog(interval time.Duration) ServerOption {
	return func(s *Server) {
		s.watchdogCh = make(chan struct{}, 1)
		s.lastActivity = time.Now()
		go s.watchdogLoop(interval)
	}
}

// ServerWithTracer sets the OpenTelemetry exporter.
func ServerWithTracer(tp trace.Exporter) ServerOption {
	return func(s *Server) { s.tp = tp }
}

func (s *Server) watchdogLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		if time.Since(s.lastActivity) > interval*3 {
			select {
			case s.watchdogCh <- struct{}{}:
			default:
			}
		}
	}
}

// NewServer creates an MCP server backed by a tool registry.
func NewServer(reg *tools.Registry, reader io.Reader, writer io.Writer, opts ...ServerOption) *Server {
	s := &Server{
		registry: reg,
		logger:   log.NewSlog(slog.Default()),
		reader:   reader,
		writer:   writer,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// WatchdogCh returns the stall-detection channel, or nil if no watchdog is active.
func (s *Server) WatchdogCh() <-chan struct{} {
	if s.watchdogCh == nil {
		return nil
	}
	return s.watchdogCh
}

// Serve reads JSON-RPC requests from the reader and responds on the writer.
// Blocks until the reader is closed or an error occurs.
func (s *Server) Serve(ctx context.Context) error {
	scanner := bufio.NewScanner(s.reader)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	notifications := map[string]func(json.RawMessage) (any, error){
		"notifications/initialized": s.handleInitialized,
	}

	s.logger.Info("MCP server started")
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		s.lastActivity = time.Now()

		var req jsonRPCRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			s.sendError(nil, -32700, "Parse error: "+err.Error())
			continue
		}

		if _, isNotification := notifications[req.Method]; isNotification {
			s.logger.Debug("received notification", "method", req.Method)
			continue
		}

		var result any
		var err error
		switch req.Method {
		case "initialize":
			result = s.handleInitialize(req.Params)
		case "tools/list":
			result = s.handleListTools()
			err = nil
		case "tools/call":
			result, err = s.handleCallTool(req.Params)
		case "resources/list":
			result = map[string]any{"resources": []any{}}
		case "health/ping":
			result = map[string]string{strconst.StrStatus: "ok"}
		default:
			s.sendError(&req.ID, -32601, fmt.Sprintf("Method not found: %s", req.Method))
			continue
		}

		if err != nil {
			s.sendError(&req.ID, -32603, err.Error())
			continue
		}
		s.sendResult(&req.ID, result)
	}

	return scanner.Err()
}

// ServeTCP listens on a TCP address and serves MCP requests to each connection.
func (s *Server) ServeTCP(ctx context.Context, addr string) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("mcp tcp listen %s: %w", addr, err)
	}
	s.logger.Info("MCP TCP server listening", "addr", addr)

	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			s.logger.Error("mcp tcp accept", strconst.StrError, err)
			continue
		}
		go s.handleConnection(conn) //nolint:gosec
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	defer func() { _ = conn.Close() }()
	s.logger.Debug("MCP TCP client connected", "remote", conn.RemoteAddr())
	sub := &Server{
		registry:    s.registry,
		logger:      s.logger,
		reader:      conn,
		writer:      conn,
		toolCtx:     s.toolCtx,
		rateLimiter: s.rateLimiter,
		workerPool:  s.workerPool,
		metrics:     s.metrics,
	}
	_ = sub.Serve(context.Background())
}

func (s *Server) handleInitialize(_ json.RawMessage) any {
	s.logger.Info("MCP initialize handshake")
	return InitializeResult{
		ProtocolVersion: "2024-11-05",
		Capabilities: &Capabilities{
			Tools: &ToolsCapabilities{ListChanged: false},
		},
		ServerInfo: Implementation{
			Name:    "dxrk",
			Version: version.Version,
		},
	}
}

func (s *Server) handleInitialized(_ json.RawMessage) (any, error) {
	s.logger.Debug("client initialized")
	return nil, nil
}

type listToolsResult struct {
	Tools []ToolDefinition `json:"tools"`
}

func (s *Server) handleListTools() any {
	allTools := s.registry.List()
	defs := make([]ToolDefinition, 0, len(allTools))
	for _, t := range allTools {
		if !t.IsEnabled() {
			continue
		}
		schema := toolSchemaFromMap(t.InputSchema())
		defs = append(defs, ToolDefinition{
			Name:        t.Name(),
			Description: t.Description(),
			InputSchema: schema,
		})
	}
	return listToolsResult{Tools: defs}
}

type callToolParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

func (s *Server) handleCallTool(params json.RawMessage) (any, error) {
	var p callToolParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	tool, ok := s.registry.Get(p.Name)
	if !ok {
		return nil, fmt.Errorf("tool not found: %s", p.Name)
	}

	// Rate limiting
	if s.rateLimiter != nil && !s.rateLimiter.Allow() {
		if s.metrics != nil {
			s.metrics.IncRateLimitedCalls(p.Name)
		}
		return CallToolResult{
			Content: []ToolResultContent{
				{Type: "text", Text: "rate limit exceeded — try again later"},
			},
			IsError: true,
		}, nil
	}

	if s.metrics != nil && s.rateLimiter != nil {
		s.metrics.SetTokensRemaining(s.rateLimiter.Tokens())
	}

	tCtx := s.toolCtx
	tCtx.Context = context.Background()
	tCtx.Logger = s.logger

	if s.tp != nil {
		tCtx.Context, _ = trace.StartSpan(tCtx.Context, "tool."+p.Name,
			trace.WithAttributes(attribute.String("tool", p.Name)),
		)
	}

	result, execErr := tool.Execute(tCtx, p.Arguments)
	if execErr != nil {
		return CallToolResult{
			Content: []ToolResultContent{
				{Type: "text", Text: fmt.Sprintf("Error: %s", execErr.Error())},
			},
			IsError: true,
		}, nil
	}

	text, err := json.Marshal(result)
	if err != nil {
		text = []byte(fmt.Sprintf("%v", result))
	}

	return CallToolResult{
		Content: []ToolResultContent{
			{Type: "text", Text: string(text)},
		},
	}, nil
}

func (s *Server) sendResult(id *json.RawMessage, result any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	body, _ := json.Marshal(result)
	resp := jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      *id,
		Result:  body,
	}
	s.writeLine(resp)
}

func (s *Server) sendError(id *json.RawMessage, code int, msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	resp := jsonRPCResponse{
		JSONRPC: "2.0",
		Error: &jsonRPCError{
			Code:    code,
			Message: msg,
		},
	}
	if id != nil {
		resp.ID = *id
	}
	s.writeLine(resp)
}

func (s *Server) writeLine(v any) {
	data, err := json.Marshal(v)
	if err != nil {
		s.logger.Error("failed to marshal response", strconst.StrError, err)
		return
	}
	data = append(data, '\n')
	if _, err := s.writer.Write(data); err != nil {
		s.logger.Error("failed to write response", strconst.StrError, err)
	}
}

func toolSchemaFromMap(m map[string]any) ToolSchema {
	schema := ToolSchema{Type: strconst.StrObject}
	if m == nil {
		return schema
	}
	props, _ := m[strconst.StrProperties].(map[string]any)
	if props == nil {
		return schema
	}
	schema.Properties = make(map[string]ToolProperty)
	for key, val := range props {
		p, _ := val.(map[string]any)
		if p == nil {
			continue
		}
		prop := ToolProperty{
			Description: getString(p, strconst.StrDescription),
		}
		if t, ok := p["type"].(string); ok {
			prop.Type = t
		} else {
			prop.Type = strconst.StrString
		}
		schema.Properties[key] = prop
	}
	return schema
}

func getString(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}
