// SPDX-License-Identifier: MIT
package remote

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"sync"
	"time"
)

// SessionOption is a functional option for RemoteSession.
type SessionOption func(*RemoteSession)

// OnStateChange is a callback for session state changes.
type OnStateChange func(old, new SessionState)

// OnMessage is a callback for incoming messages.
type OnMessage func(msg *RemoteMessage)

// WithOnStateChange registers a state change callback.
func WithOnStateChange(fn OnStateChange) SessionOption {
	return func(s *RemoteSession) { s.onStateChange = fn }
}

// WithOnMessage registers a message callback.
func WithOnMessage(fn OnMessage) SessionOption {
	return func(s *RemoteSession) { s.onMessage = fn }
}

// WithAutoReconnect enables automatic reconnection on disconnect.
func WithAutoReconnect(enabled bool) SessionOption {
	return func(s *RemoteSession) { s.autoReconnect = enabled }
}

// WithSessionID overrides the auto-generated session ID.
func WithSessionID(id string) SessionOption {
	return func(s *RemoteSession) { s.id = id }
}

// RemoteSession manages a single remote connection lifecycle.
type RemoteSession struct {
	mu            sync.RWMutex
	id            string
	config        *RemoteConfig
	state         SessionState
	conn          net.Conn
	lastActivity  time.Time
	retryCount    int
	autoReconnect bool
	onStateChange OnStateChange
	onMessage     OnMessage
	ctx           context.Context
	cancel        context.CancelFunc
	done          chan struct{}
	msgRouter     *MessageRouter
}

// NewRemoteSession creates a new session with the given config and options.
func NewRemoteSession(cfg *RemoteConfig, opts ...SessionOption) *RemoteSession {
	ctx, cancel := context.WithCancel(context.Background())
	s := &RemoteSession{
		config:        cfg,
		state:         StateDisconnected,
		autoReconnect: false,
		ctx:           ctx,
		cancel:        cancel,
		done:          make(chan struct{}),
		msgRouter:     NewMessageRouter(),
	}
	for _, opt := range opts {
		opt(s)
	}
	if s.id == "" {
		s.id = fmt.Sprintf("session-%d", time.Now().UnixNano())
	}
	return s
}

// ID returns the session identifier.
func (s *RemoteSession) ID() string {
	return s.id
}

// State returns the current session state.
func (s *RemoteSession) State() SessionState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}

// Config returns the session configuration.
func (s *RemoteSession) Config() *RemoteConfig {
	return s.config
}

// Router returns the message router for this session.
func (s *RemoteSession) Router() *MessageRouter {
	return s.msgRouter
}

// LastActivity returns the time of the last message activity.
func (s *RemoteSession) LastActivity() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastActivity
}

// Connect establishes a connection to the remote host.
func (s *RemoteSession) Connect(ctx context.Context) error {
	s.mu.Lock()
	if s.state == StateConnected {
		s.mu.Unlock()
		return fmt.Errorf("session %s already connected", s.id)
	}
	s.setState(StateConnecting)
	s.mu.Unlock()

	addr := s.config.Address()
	dialer := &net.Dialer{
		Timeout:   s.config.ConnectTimeoutDuration(),
		KeepAlive: s.config.KeepAliveDuration(),
	}

	var conn net.Conn
	var err error

	if s.config.TLS.Enabled {
		tlsCfg, tlsErr := s.config.BuildTLSConfig()
		if tlsErr != nil {
			s.setState(StateFailed)
			return fmt.Errorf("build TLS config: %w", tlsErr)
		}
		conn, err = tls.DialWithDialer(dialer, "tcp", addr, tlsCfg)
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", addr)
	}

	if err != nil {
		s.setState(StateFailed)
		return fmt.Errorf("connect to %s: %w", addr, err)
	}

	s.mu.Lock()
	s.conn = conn
	s.lastActivity = time.Now()
	s.retryCount = 0
	s.setState(StateConnected)
	s.mu.Unlock()

	go s.readLoop()

	return nil
}

// Disconnect closes the session connection.
func (s *RemoteSession) Disconnect() error {
	s.mu.Lock()
	if s.state == StateDisconnected {
		s.mu.Unlock()
		return nil
	}
	s.setState(StateDisconnected)
	conn := s.conn
	s.conn = nil
	s.mu.Unlock()

	if conn != nil {
		closeMsg := NewCloseMessage(s.id)
		data, err := closeMsg.Encode()
		if err == nil {
			_ = conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
			_, _ = conn.Write(append(data, '\n'))
		}
		return conn.Close()
	}
	return nil
}

// Reconnect disconnects and reconnects to the remote host.
func (s *RemoteSession) Reconnect(ctx context.Context) error {
	s.mu.Lock()
	s.setState(StateReconnecting)
	s.mu.Unlock()

	_ = s.Disconnect()

	return s.Connect(ctx)
}

// Send transmits a message to the remote host.
func (s *RemoteSession) Send(msg *RemoteMessage) error {
	s.mu.RLock()
	if s.state != StateConnected {
		s.mu.RUnlock()
		return fmt.Errorf("session not connected (state: %s)", s.state)
	}
	conn := s.conn
	s.mu.RUnlock()

	data, err := msg.Encode()
	if err != nil {
		return fmt.Errorf("encode message: %w", err)
	}

	_ = conn.SetWriteDeadline(time.Now().Add(s.config.WriteTimeoutDuration()))
	if _, err := conn.Write(append(data, '\n')); err != nil {
		s.handleError(err)
		return fmt.Errorf("send message: %w", err)
	}

	s.mu.Lock()
	s.lastActivity = time.Now()
	s.mu.Unlock()

	return nil
}

// SendRequest sends a request and waits for the response.
func (s *RemoteSession) SendRequest(ctx context.Context, method string, payload any) (*RemoteMessage, error) {
	msg, err := NewRequest(fmt.Sprintf("req-%d", time.Now().UnixNano()), method, payload)
	if err != nil {
		return nil, err
	}

	type result struct {
		msg *RemoteMessage
		err error
	}
	ch := make(chan result, 1)

	handler := func(m *RemoteMessage) (*RemoteMessage, error) {
		ch <- result{msg: m}
		return nil, nil
	}

	s.msgRouter.RegisterFunc(msg.ID, handler)
	defer s.msgRouter.RegisterFunc(msg.ID, nil)

	if err := s.Send(msg); err != nil {
		return nil, err
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case r := <-ch:
		return r.msg, r.err
	case <-time.After(s.config.ReadTimeoutDuration()):
		return nil, fmt.Errorf("request timed out")
	}
}

// readLoop continuously reads messages from the connection.
func (s *RemoteSession) readLoop() {
	defer close(s.done)

	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		s.mu.RLock()
		conn := s.conn
		s.mu.RUnlock()
		if conn == nil {
			return
		}

		_ = conn.SetReadDeadline(time.Now().Add(s.config.ReadTimeoutDuration()))

		buf := make([]byte, s.config.MaxMessageSize)
		n, err := conn.Read(buf)
		if err != nil {
			s.handleError(err)
			return
		}

		if n == 0 {
			continue
		}

		msg, err := DecodeMessage(buf[:n])
		if err != nil {
			continue
		}

		s.mu.Lock()
		s.lastActivity = time.Now()
		s.mu.Unlock()

		if msg.Type == MsgClose {
			s.mu.Lock()
			s.setState(StateDisconnected)
			s.mu.Unlock()
			return
		}

		if msg.Type == MsgHeartbeat {
			ack := NewHeartbeatAck(msg.ID)
			_ = s.Send(ack)
			continue
		}

		s.mu.RLock()
		handler := s.onMessage
		s.mu.RUnlock()
		if handler != nil {
			go handler(msg)
		}

		if resp, err := s.msgRouter.Route(msg); err == nil && resp != nil {
			_ = s.Send(resp)
		}
	}
}

// handleError processes connection errors and handles reconnection.
func (s *RemoteSession) handleError(_ error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.state = StateFailed

	if s.conn != nil {
		_ = s.conn.Close()
		s.conn = nil
	}

	if s.autoReconnect && s.retryCount < s.config.MaxRetries {
		s.retryCount++
		delay := s.config.RetryDelayDuration() * time.Duration(s.retryCount)
		go func() {
			time.Sleep(delay)
			s.mu.Unlock()
			_ = s.Connect(s.ctx)
			s.mu.Lock()
		}()
	}
}

// setState updates the session state and notifies listeners.
func (s *RemoteSession) setState(newState SessionState) {
	old := s.state
	s.state = newState
	if s.onStateChange != nil && old != newState {
		go s.onStateChange(old, newState)
	}
}

// Done returns a channel that is closed when the session ends.
func (s *RemoteSession) Done() <-chan struct{} {
	return s.done
}

// Context returns the session context.
func (s *RemoteSession) Context() context.Context {
	return s.ctx
}

// Close cancels the session context and disconnects.
func (s *RemoteSession) Close() error {
	s.cancel()
	return s.Disconnect()
}
