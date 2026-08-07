package advanced

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/Dxrk777/Dxrk/internal/strconst"
)

var (
	ErrSessionClosed      = errors.New("session closed")
	ErrSessionTimeout     = errors.New("session timed out")
	ErrSessionDuplicate   = errors.New("session already exists")
	ErrSessionMaxExceeded = errors.New("maximum sessions exceeded")
)

type SessionState int

const (
	SessionPending SessionState = iota
	SessionActive
	SessionPaused
	SessionCompleted
	SessionFailed
	SessionKilled
)

func (s SessionState) String() string {
	switch s {
	case SessionPending:
		return strconst.StrPending
	case SessionActive:
		return strconst.StrActive
	case SessionPaused:
		return "paused"
	case SessionCompleted:
		return strconst.StrCompleted
	case SessionFailed:
		return strconst.StrFailed
	case SessionKilled:
		return "killed"
	default:
		return strconst.StrUnknown
	}
}

type ActivityType string

const (
	ActivityToolStart ActivityType = "tool_start"
	ActivityText      ActivityType = "text"
	ActivityResult    ActivityType = strconst.StrResult
	ActivityError     ActivityType = strconst.StrError
)

type SessionActivity struct {
	Type      ActivityType `json:"type"`
	Summary   string       `json:"summary"`
	Timestamp int64        `json:"timestamp"`
}

type PermissionRequest struct {
	ID       string                 `json:"id"`
	ToolName string                 `json:"tool_name"`
	Input    map[string]interface{} `json:"input"`
	Action   string                 `json:"action"`
	Message  string                 `json:"message"`
}

type PermissionResponse struct {
	RequestID string `json:"request_id"`
	Decision  string `json:"decision"`
	Message   string `json:"message,omitempty"`
}

type Session struct {
	mu           sync.RWMutex
	id           string
	remoteID     string
	state        SessionState
	createdAt    time.Time
	startedAt    *time.Time
	completedAt  *time.Time
	accessToken  string
	activities   []SessionActivity
	permRequests map[string]*PermissionRequest
	stderr       []string
	error        string
	metadata     map[string]string
	timeout      time.Duration
	ctx          context.Context
	cancel       context.CancelFunc
	doneCh       chan SessionState
	updateCh     chan SessionEvent
}

type SessionEvent struct {
	Type    string
	Payload interface{}
}

type SessionOption func(*Session)

func WithSessionTimeout(d time.Duration) SessionOption {
	return func(s *Session) { s.timeout = d }
}

func WithSessionMetadata(k, v string) SessionOption {
	return func(s *Session) {
		if s.metadata == nil {
			s.metadata = make(map[string]string)
		}
		s.metadata[k] = v
	}
}

func WithRemoteID(id string) SessionOption {
	return func(s *Session) { s.remoteID = id }
}

func NewSession(id, accessToken string, opts ...SessionOption) *Session {
	ctx, cancel := context.WithCancel(context.Background())
	now := time.Now()
	s := &Session{
		id:           id,
		state:        SessionPending,
		createdAt:    now,
		accessToken:  accessToken,
		activities:   make([]SessionActivity, 0, 16),
		permRequests: make(map[string]*PermissionRequest),
		stderr:       make([]string, 0, 8),
		metadata:     make(map[string]string),
		timeout:      24 * time.Hour,
		ctx:          ctx,
		cancel:       cancel,
		doneCh:       make(chan SessionState, 1),
		updateCh:     make(chan SessionEvent, 64),
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

func (s *Session) ID() string                   { return s.id }
func (s *Session) RemoteID() string             { s.mu.RLock(); defer s.mu.RUnlock(); return s.remoteID }
func (s *Session) State() SessionState          { s.mu.RLock(); defer s.mu.RUnlock(); return s.state }
func (s *Session) AccessToken() string          { s.mu.RLock(); defer s.mu.RUnlock(); return s.accessToken }
func (s *Session) CreatedAt() time.Time         { return s.createdAt }
func (s *Session) ErrorMessage() string         { s.mu.RLock(); defer s.mu.RUnlock(); return s.error }
func (s *Session) Done() <-chan SessionState    { return s.doneCh }
func (s *Session) Updates() <-chan SessionEvent { return s.updateCh }
func (s *Session) Context() context.Context     { return s.ctx }

func (s *Session) StartedAt() *time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.startedAt
}

func (s *Session) CompletedAt() *time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.completedAt
}

func (s *Session) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state != SessionPending {
		return fmt.Errorf("cannot start session in state %s", s.state)
	}

	now := time.Now()
	s.state = SessionActive
	s.startedAt = &now

	if s.timeout > 0 {
		go s.timeoutWatcher()
	}

	return nil
}

func (s *Session) Complete(errMsg string) {
	s.mu.Lock()
	if s.state == SessionCompleted || s.state == SessionKilled {
		s.mu.Unlock()
		return
	}

	now := time.Now()
	s.completedAt = &now
	if errMsg != "" {
		s.state = SessionFailed
		s.error = errMsg
	} else {
		s.state = SessionCompleted
	}
	s.mu.Unlock()

	s.cancel()

	select {
	case s.doneCh <- s.state:
	default:
	}

	s.emit(SessionEvent{Type: strconst.StrCompleted, Payload: s.state})
}

func (s *Session) Kill() {
	s.mu.Lock()
	if s.state == SessionCompleted || s.state == SessionKilled {
		s.mu.Unlock()
		return
	}

	now := time.Now()
	s.completedAt = &now
	s.state = SessionKilled
	s.mu.Unlock()

	s.cancel()

	select {
	case s.doneCh <- s.state:
	default:
	}

	s.emit(SessionEvent{Type: "killed", Payload: s.state})
}

func (s *Session) Pause() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state != SessionActive {
		return fmt.Errorf("cannot pause session in state %s", s.state)
	}
	s.state = SessionPaused
	s.emit(SessionEvent{Type: "paused"})
	return nil
}

func (s *Session) Resume() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state != SessionPaused {
		return fmt.Errorf("cannot resume session in state %s", s.state)
	}
	s.state = SessionActive
	s.emit(SessionEvent{Type: "resumed"})
	return nil
}

func (s *Session) AddActivity(actType ActivityType, summary string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	activity := SessionActivity{
		Type:      actType,
		Summary:   summary,
		Timestamp: time.Now().UnixMilli(),
	}
	s.activities = append(s.activities, activity)
	if len(s.activities) > 20 {
		s.activities = s.activities[len(s.activities)-20:]
	}

	s.emit(SessionEvent{Type: "activity", Payload: activity})
}

func (s *Session) Activities() []SessionActivity {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]SessionActivity, len(s.activities))
	copy(out, s.activities)
	return out
}

func (s *Session) AddPermissionRequest(req *PermissionRequest) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.permRequests[req.ID] = req
	s.emit(SessionEvent{Type: "permission_request", Payload: req})
}

func (s *Session) ResolvePermission(requestID, decision, message string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.permRequests[requestID]; ok {
		delete(s.permRequests, requestID)
		resp := &PermissionResponse{
			RequestID: requestID,
			Decision:  decision,
			Message:   message,
		}
		s.emit(SessionEvent{Type: "permission_response", Payload: resp})
	}
}

func (s *Session) PendingPermissions() []*PermissionRequest {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*PermissionRequest, 0, len(s.permRequests))
	for _, req := range s.permRequests {
		out = append(out, req)
	}
	return out
}

func (s *Session) AppendStderr(line string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stderr = append(s.stderr, line)
	if len(s.stderr) > 50 {
		s.stderr = s.stderr[len(s.stderr)-50:]
	}
}

func (s *Session) Stderr() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]string, len(s.stderr))
	copy(out, s.stderr)
	return out
}

func (s *Session) SetMetadata(k, v string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.metadata[k] = v
}

func (s *Session) Metadata(k string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.metadata[k]
}

func (s *Session) Duration() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.completedAt != nil {
		return s.completedAt.Sub(s.createdAt)
	}
	if s.startedAt != nil {
		return time.Since(*s.startedAt)
	}
	return 0
}

func (s *Session) timeoutWatcher() {
	timer := time.NewTimer(s.timeout)
	defer timer.Stop()

	select {
	case <-timer.C:
		s.mu.Lock()
		if s.state == SessionActive {
			s.state = SessionKilled
			s.error = "session timed out"
			now := time.Now()
			s.completedAt = &now
			s.mu.Unlock()
			s.cancel()
			s.emit(SessionEvent{Type: strconst.StrTimeout})
		} else {
			s.mu.Unlock()
		}
	case <-s.ctx.Done():
		return
	}
}

func (s *Session) emit(evt SessionEvent) {
	select {
	case s.updateCh <- evt:
	default:
	}
}

func (s *Session) MarshalJSON() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return json.Marshal(struct {
		ID          string            `json:"id"`
		RemoteID    string            `json:"remote_id"`
		State       SessionState      `json:"state"`
		CreatedAt   time.Time         `json:"created_at"`
		StartedAt   *time.Time        `json:"started_at,omitempty"`
		CompletedAt *time.Time        `json:"completed_at,omitempty"`
		Error       string            `json:"error,omitempty"`
		Metadata    map[string]string `json:"metadata"`
	}{
		ID:          s.id,
		RemoteID:    s.remoteID,
		State:       s.state,
		CreatedAt:   s.createdAt,
		StartedAt:   s.startedAt,
		CompletedAt: s.completedAt,
		Error:       s.error,
		Metadata:    s.metadata,
	})
}

type SessionManager struct {
	mu          sync.RWMutex
	sessions    map[string]*Session
	maxSessions int
}

func NewSessionManager(maxSessions int) *SessionManager {
	if maxSessions <= 0 {
		maxSessions = 10
	}
	return &SessionManager{
		sessions:    make(map[string]*Session),
		maxSessions: maxSessions,
	}
}

func (m *SessionManager) Create(id, token string, opts ...SessionOption) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.sessions[id]; exists {
		return nil, ErrSessionDuplicate
	}

	if len(m.sessions) >= m.maxSessions {
		return nil, ErrSessionMaxExceeded
	}

	sess := NewSession(id, token, opts...)
	m.sessions[id] = sess
	return sess, nil
}

func (m *SessionManager) Get(id string) (*Session, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[id]
	return s, ok
}

func (m *SessionManager) Remove(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.sessions[id]; ok {
		s.Kill()
		delete(m.sessions, id)
	}
}

func (m *SessionManager) Active() []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []*Session
	for _, s := range m.sessions {
		st := s.State()
		if st == SessionActive || st == SessionPending {
			out = append(out, s)
		}
	}
	return out
}

func (m *SessionManager) All() []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		out = append(out, s)
	}
	return out
}

func (m *SessionManager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.sessions)
}

func (m *SessionManager) Cleanup() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := 0
	for id, s := range m.sessions {
		st := s.State()
		if st == SessionCompleted || st == SessionFailed || st == SessionKilled {
			delete(m.sessions, id)
			count++
		}
	}
	return count
}
