package advanced

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/Dxrk777/Dxrk-Ai/internal/strconst"
)

var (
	ErrPermissionDenied    = errors.New("permission denied")
	ErrPermissionTimeout   = errors.New("permission request timed out")
	ErrPermissionCancelled = errors.New("permission request cancelled")
	ErrPermissionDuplicate = errors.New("duplicate permission request")
)

type PermissionBehavior string

const (
	BehaviorAllow PermissionBehavior = "allow"
	BehaviorDeny  PermissionBehavior = "deny"
)

type PermissionScope string

const (
	ScopeFileRead  PermissionScope = "file:read"
	ScopeFileWrite PermissionScope = "file:write"
	ScopeExec      PermissionScope = "exec"
	ScopeNetwork   PermissionScope = "network"
	ScopeSystem    PermissionScope = strconst.StrSystem
	ScopeSession   PermissionScope = "session"
)

type PermissionEntry struct {
	Scope     PermissionScope    `json:"scope"`
	Behavior  PermissionBehavior `json:"behavior"`
	Paths     []string           `json:"paths,omitempty"`
	Patterns  []string           `json:"patterns,omitempty"`
	ExpiresAt *time.Time         `json:"expires_at,omitempty"`
}

type PermissionPolicy struct {
	mu       sync.RWMutex
	entries  []PermissionEntry
	defaultB PermissionBehavior
}

func NewPermissionPolicy(defaultBehavior PermissionBehavior) *PermissionPolicy {
	return &PermissionPolicy{
		entries:  make([]PermissionEntry, 0),
		defaultB: defaultBehavior,
	}
}

func (p *PermissionPolicy) Allow(scope PermissionScope, paths ...string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.entries = append(p.entries, PermissionEntry{
		Scope:    scope,
		Behavior: BehaviorAllow,
		Paths:    paths,
	})
}

func (p *PermissionPolicy) Deny(scope PermissionScope, paths ...string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.entries = append(p.entries, PermissionEntry{
		Scope:    scope,
		Behavior: BehaviorDeny,
		Paths:    paths,
	})
}

func (p *PermissionPolicy) AllowPattern(scope PermissionScope, patterns ...string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.entries = append(p.entries, PermissionEntry{
		Scope:    scope,
		Behavior: BehaviorAllow,
		Patterns: patterns,
	})
}

func (p *PermissionPolicy) Check(scope PermissionScope, path string) PermissionBehavior {
	p.mu.RLock()
	defer p.mu.RUnlock()

	now := time.Now()
	for i := len(p.entries) - 1; i >= 0; i-- {
		e := p.entries[i]
		if e.Scope != scope {
			continue
		}
		if e.ExpiresAt != nil && now.After(*e.ExpiresAt) {
			continue
		}
		if path == "" && len(e.Paths) == 0 && len(e.Patterns) == 0 {
			return e.Behavior
		}
		for _, ep := range e.Paths {
			if ep == path {
				return e.Behavior
			}
		}
		for _, pattern := range e.Patterns {
			if matchGlob(pattern, path) {
				return e.Behavior
			}
		}
	}
	return p.defaultB
}

func (p *PermissionPolicy) Entries() []PermissionEntry {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]PermissionEntry, len(p.entries))
	copy(out, p.entries)
	return out
}

func (p *PermissionPolicy) SetDefault(b PermissionBehavior) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.defaultB = b
}

type PermissionDelegate struct {
	mu        sync.RWMutex
	pending   map[string]*pendingPermReq
	policy    *PermissionPolicy
	timeout   time.Duration
	onRequest func(*PermissionRequest)
}

type pendingPermReq struct {
	req      *PermissionRequest
	response chan *PermissionResponse
	timer    *time.Timer
}

func NewPermissionDelegate(policy *PermissionPolicy, timeout time.Duration) *PermissionDelegate {
	return &PermissionDelegate{
		pending: make(map[string]*pendingPermReq),
		policy:  policy,
		timeout: timeout,
	}
}

func (d *PermissionDelegate) SetOnRequest(fn func(*PermissionRequest)) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.onRequest = fn
}

func (d *PermissionDelegate) CheckLocal(scope PermissionScope, path string) PermissionBehavior {
	return d.policy.Check(scope, path)
}

func (d *PermissionDelegate) Request(ctx context.Context, req *PermissionRequest) (*PermissionResponse, error) {
	d.mu.Lock()
	if _, exists := d.pending[req.ID]; exists {
		d.mu.Unlock()
		return nil, ErrPermissionDuplicate
	}

	respCh := make(chan *PermissionResponse, 1)
	timer := time.NewTimer(d.timeout)
	d.pending[req.ID] = &pendingPermReq{
		req:      req,
		response: respCh,
		timer:    timer,
	}

	onReq := d.onRequest
	d.mu.Unlock()

	if onReq != nil {
		go onReq(req)
	}

	select {
	case resp := <-respCh:
		timer.Stop()
		return resp, nil
	case <-timer.C:
		d.mu.Lock()
		delete(d.pending, req.ID)
		d.mu.Unlock()
		return nil, ErrPermissionTimeout
	case <-ctx.Done():
		timer.Stop()
		d.mu.Lock()
		delete(d.pending, req.ID)
		d.mu.Unlock()
		return nil, ctx.Err()
	}
}

func (d *PermissionDelegate) Respond(requestID string, behavior PermissionBehavior, message string) error {
	d.mu.Lock()
	p, ok := d.pending[requestID]
	if !ok {
		d.mu.Unlock()
		return errors.New("unknown request")
	}
	delete(d.pending, requestID)
	d.mu.Unlock()

	p.timer.Stop()
	resp := &PermissionResponse{
		RequestID: requestID,
		Decision:  string(behavior),
		Message:   message,
	}
	p.response <- resp
	return nil
}

func (d *PermissionDelegate) Cancel(requestID string) error {
	d.mu.Lock()
	p, ok := d.pending[requestID]
	if !ok {
		d.mu.Unlock()
		return nil
	}
	delete(d.pending, requestID)
	d.mu.Unlock()

	p.timer.Stop()
	p.response <- &PermissionResponse{
		RequestID: requestID,
		Decision:  string(BehaviorDeny),
		Message:   strconst.StrCancelled,
	}
	return nil
}

func (d *PermissionDelegate) Pending() []*PermissionRequest {
	d.mu.RLock()
	defer d.mu.RUnlock()
	out := make([]*PermissionRequest, 0, len(d.pending))
	for _, p := range d.pending {
		out = append(out, p.req)
	}
	return out
}

func (d *PermissionDelegate) MarshalJSON() ([]byte, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	pending := make([]*PermissionRequest, 0, len(d.pending))
	for _, p := range d.pending {
		pending = append(pending, p.req)
	}
	return json.Marshal(struct {
		Pending []*PermissionRequest `json:"pending"`
		Default PermissionBehavior   `json:"default"`
	}{
		Pending: pending,
		Default: d.policy.defaultB,
	})
}

func matchGlob(pattern, path string) bool {
	if pattern == "" {
		return path == ""
	}
	if pattern == "*" {
		return true
	}
	if len(pattern) > 1 && pattern[0] == '*' && pattern[1] == '/' {
		suffix := pattern[2:]
		return len(path) >= len(suffix) && path[len(path)-len(suffix):] == suffix
	}
	if len(pattern) > 0 && pattern[len(pattern)-1] == '*' {
		prefix := pattern[:len(pattern)-1]
		return len(path) >= len(prefix) && path[:len(prefix)] == prefix
	}
	return pattern == path
}
