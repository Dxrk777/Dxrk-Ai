package advanced

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/Dxrk777/Dxrk-Ai/internal/strconst"
)

var (
	ErrServerClosed    = errors.New("bridge server closed")
	ErrMaxConnections  = errors.New("max connections reached")
	ErrUnauthorized    = errors.New("unauthorized")
	ErrSessionNotFound = errors.New("session not found")
)

type ServerOption func(*Server)

type ServerHandler interface {
	ServeBridge(w http.ResponseWriter, r *http.Request, conn *BridgeConn)
}

type BridgeConn struct {
	mu        sync.RWMutex
	id        string
	remote    string
	token     string
	createdAt time.Time
	lastSeen  time.Time
	closed    bool
	closeCh   chan struct{}
	metadata  map[string]string
}

func NewBridgeConn(id, remote, token string) *BridgeConn {
	now := time.Now()
	return &BridgeConn{
		id:        id,
		remote:    remote,
		token:     token,
		createdAt: now,
		lastSeen:  now,
		closeCh:   make(chan struct{}),
		metadata:  make(map[string]string),
	}
}

func (c *BridgeConn) ID() string              { return c.id }
func (c *BridgeConn) RemoteAddr() string      { return c.remote }
func (c *BridgeConn) Token() string           { return c.token }
func (c *BridgeConn) CreatedAt() time.Time    { return c.createdAt }
func (c *BridgeConn) Done() <-chan struct{}   { return c.closeCh }
func (c *BridgeConn) SetMetadata(k, v string) { c.mu.Lock(); defer c.mu.Unlock(); c.metadata[k] = v }

func (c *BridgeConn) LastSeen() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastSeen
}

func (c *BridgeConn) Metadata(k string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.metadata[k]
}

func (c *BridgeConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	close(c.closeCh)
	return nil
}

type Server struct {
	mu           sync.RWMutex
	config       *Config
	httpServer   *http.Server
	listener     net.Listener
	connections  map[string]*BridgeConn
	connsByToken map[string]*BridgeConn
	handler      ServerHandler
	middleware   []Middleware
	authFunc     func(token string) bool
	quit         chan struct{}
	done         chan struct{}
}

func NewServer(config *Config, opts ...ServerOption) *Server {
	s := &Server{
		config:       config,
		connections:  make(map[string]*BridgeConn),
		connsByToken: make(map[string]*BridgeConn),
		quit:         make(chan struct{}),
		done:         make(chan struct{}),
		authFunc:     func(token string) bool { return token != "" },
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

func WithHandler(h ServerHandler) ServerOption {
	return func(s *Server) { s.handler = h }
}

func WithMiddleware(mw ...Middleware) ServerOption {
	return func(s *Server) { s.middleware = append(s.middleware, mw...) }
}

func WithAuthFunc(fn func(string) bool) ServerOption {
	return func(s *Server) { s.authFunc = fn }
}

func (s *Server) Start(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.config.Address())
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	s.listener = ln

	mux := http.NewServeMux()
	mux.HandleFunc("/bridge/connect", s.handleConnect)
	mux.HandleFunc("/bridge/disconnect", s.handleDisconnect)
	mux.HandleFunc("/bridge/ping", s.handlePing)
	mux.HandleFunc("/bridge/sessions", s.handleSessions)
	mux.HandleFunc("/bridge/sessions/", s.handleSessionByID)
	mux.HandleFunc("/", s.serveFallback)

	var handler http.Handler = mux
	for i := len(s.middleware) - 1; i >= 0; i-- {
		handler = s.middleware[i].Wrap(handler)
	}

	s.httpServer = &http.Server{
		Handler:           handler,
		ReadTimeout:       s.config.ReadTimeout,
		WriteTimeout:      s.config.WriteTimeout,
		IdleTimeout:       s.config.IdleTimeout,
		MaxHeaderBytes:    s.config.MaxHeaderBytes,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		defer close(s.done)
		if s.config.TLSEnabled {
			tlsCert, err := tls.LoadX509KeyPair(s.config.TLSCertFile, s.config.TLSKeyFile)
			if err != nil {
				return
			}
			s.httpServer.TLSConfig = &tls.Config{
				Certificates: []tls.Certificate{tlsCert},
				MinVersion:   tls.VersionTLS12,
			}
			if err := s.httpServer.ServeTLS(ln, "", ""); err != nil && !errors.Is(err, http.ErrServerClosed) {
				return
			}
		} else {
			if err := s.httpServer.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
				return
			}
		}
	}()

	go s.cleanupLoop()

	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	close(s.quit)

	for _, conn := range s.connections {
		_ = conn.Close()
	}

	if s.httpServer != nil {
		if err := s.httpServer.Shutdown(ctx); err != nil {
			return fmt.Errorf("shutdown: %w", err)
		}
	}
	if s.listener != nil {
		_ = s.listener.Close()
	}

	<-s.done
	return nil
}

func (s *Server) ConnectionCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.connections)
}

func (s *Server) GetConnection(id string) (*BridgeConn, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.connections[id]
	return c, ok
}

func (s *Server) GetConnectionByToken(token string) (*BridgeConn, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.connsByToken[token]
	return c, ok
}

func (s *Server) serveFallback(w http.ResponseWriter, r *http.Request) {
	if s.handler != nil {
		conn := s.connFromRequest(r)
		s.handler.ServeBridge(w, r, conn)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{strconst.StrStatus: "ok"})
}

func (s *Server) handleConnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	token := extractToken(r)
	if !s.authFunc(token) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.config.MaxConns > 0 && len(s.connections) >= s.config.MaxConns {
		http.Error(w, "max connections reached", http.StatusServiceUnavailable)
		return
	}

	id := generateBridgeID()
	remote := r.RemoteAddr
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		remote = fwd
	}

	conn := NewBridgeConn(id, remote, token)
	s.connections[id] = conn
	s.connsByToken[token] = conn

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"connection_id": id})
}

func (s *Server) handleDisconnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	token := extractToken(r)
	s.mu.Lock()
	defer s.mu.Unlock()

	if conn, ok := s.connsByToken[token]; ok {
		_ = conn.Close()
		delete(s.connections, conn.id)
		delete(s.connsByToken, token)
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handlePing(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	token := extractToken(r)
	s.mu.RLock()
	conn, ok := s.connsByToken[token]
	s.mu.RUnlock()

	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	conn.mu.Lock()
	conn.lastSeen = time.Now()
	conn.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{strconst.StrStatus: "pong"})
}

func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	type sessionInfo struct {
		ID        string            `json:"id"`
		Remote    string            `json:"remote"`
		CreatedAt time.Time         `json:"created_at"`
		LastSeen  time.Time         `json:"last_seen"`
		Metadata  map[string]string `json:"metadata"`
	}

	sessions := make([]sessionInfo, 0, len(s.connections))
	for _, c := range s.connections {
		c.mu.RLock()
		sessions = append(sessions, sessionInfo{
			ID:        c.id,
			Remote:    c.remote,
			CreatedAt: c.createdAt,
			LastSeen:  c.lastSeen,
			Metadata:  copyMap(c.metadata),
		})
		c.mu.RUnlock()
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(sessions)
}

func (s *Server) handleSessionByID(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Path[len("/bridge/sessions/"):]
	if id == "" {
		http.Error(w, "session id required", http.StatusBadRequest)
		return
	}

	s.mu.RLock()
	conn, ok := s.connections[id]
	s.mu.RUnlock()

	if !ok {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	conn.mu.RLock()
	info := map[string]interface{}{
		"id":                  conn.id,
		"remote":              conn.remote,
		strconst.StrCreatedAt: conn.createdAt,
		"last_seen":           conn.lastSeen,
		"metadata":            copyMap(conn.metadata),
	}
	conn.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(info)
}

func (s *Server) cleanupLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.quit:
			return
		case <-ticker.C:
			s.cleanupStale()
		}
	}
}

func (s *Server) cleanupStale() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for id, conn := range s.connections {
		conn.mu.RLock()
		stale := now.Sub(conn.lastSeen) > s.config.IdleTimeout
		conn.mu.RUnlock()

		if stale {
			_ = conn.Close()
			delete(s.connections, id)
			delete(s.connsByToken, conn.token)
		}
	}
}

func (s *Server) connFromRequest(r *http.Request) *BridgeConn {
	token := extractToken(r)
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.connsByToken[token]
}

func extractToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if len(auth) > 7 && auth[:7] == "Bearer " {
		return auth[7:]
	}
	return r.URL.Query().Get("token")
}

func generateBridgeID() string {
	b := make([]byte, 16)
	for i := range b {
		b[i] = byte(time.Now().UnixNano() & 0xff)
		time.Sleep(time.Nanosecond)
	}
	return fmt.Sprintf("%x", b)
}

func copyMap(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	cp := make(map[string]string, len(m))
	for k, v := range m {
		cp[k] = v
	}
	return cp
}
