// SPDX-License-Identifier: MIT
package a2a

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/Dxrk777/Dxrk/internal/strconst"
	"github.com/google/uuid"
)

type MessageHandler func(ctx context.Context, msg Message) (any, error)

type AgentNode struct {
	Name         string
	Capabilities []Capability
	handler      MessageHandler
	peers        map[string]*AgentNode
	mu           sync.RWMutex
	messages     chan Message
	ctx          context.Context
	cancel       context.CancelFunc
	logger       func(format string, args ...any)
}

type AgentOption func(*AgentNode)

func WithAgentLogger(logFn func(format string, args ...any)) AgentOption {
	return func(a *AgentNode) { a.logger = logFn }
}

func NewAgentNode(name string, caps []Capability, handler MessageHandler, opts ...AgentOption) *AgentNode {
	ctx, cancel := context.WithCancel(context.Background())
	a := &AgentNode{
		Name:         name,
		Capabilities: caps,
		handler:      handler,
		peers:        make(map[string]*AgentNode),
		messages:     make(chan Message, 100),
		ctx:          ctx,
		cancel:       cancel,
		logger:       func(string, ...any) {},
	}
	for _, opt := range opts {
		opt(a)
	}
	go a.messageLoop()
	return a
}

func (a *AgentNode) AddPeer(peer *AgentNode) {
	a.mu.Lock()
	a.peers[peer.Name] = peer
	a.mu.Unlock()
	a.logger("[a2a] %s added peer %s", a.Name, peer.Name)
}

func (a *AgentNode) RemovePeer(name string) {
	a.mu.Lock()
	delete(a.peers, name)
	a.mu.Unlock()
}

func (a *AgentNode) Stop() {
	a.cancel()
}

func (a *AgentNode) Send(ctx context.Context, target string, msg Message) (Message, error) {
	a.mu.RLock()
	peer, ok := a.peers[target]
	a.mu.RUnlock()
	if !ok {
		return Message{}, fmt.Errorf("peer %q not found", target)
	}

	ch := make(chan Message, 1)
	pendingMu.Lock()
	pending[msg.ID] = ch
	pendingMu.Unlock()

	defer func() {
		pendingMu.Lock()
		delete(pending, msg.ID)
		pendingMu.Unlock()
	}()

	select {
	case peer.messages <- msg:
	default:
		return Message{}, fmt.Errorf("peer %q message buffer full", target)
	}

	select {
	case resp := <-ch:
		return resp, nil
	case <-time.After(30 * time.Second):
		return Message{}, fmt.Errorf("timeout waiting for response to %s", msg.ID)
	case <-ctx.Done():
		return Message{}, ctx.Err()
	}
}

func (a *AgentNode) Handoff(ctx context.Context, target, task string, contextData any) (*HandoffResult, error) {
	var ctxRaw json.RawMessage
	if contextData != nil {
		raw, err := json.Marshal(contextData)
		if err != nil {
			return nil, fmt.Errorf("marshal context: %w", err)
		}
		ctxRaw = raw
	}

	params := HandoffParams{
		FromAgent: a.Name,
		ToAgent:   target,
		Task:      task,
		Context:   ctxRaw,
	}
	body, _ := json.Marshal(params)

	msg := Message{
		JSONRPC: Version1,
		ID:      uuid.New().String(),
		Method:  MethodHandoff,
		Params:  body,
	}

	resp, err := a.Send(ctx, target, msg)
	if err != nil {
		return nil, err
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("handoff error: %s", resp.Error.Message)
	}

	var result HandoffResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("parse handoff result: %w", err)
	}
	return &result, nil
}

func (a *AgentNode) Query(ctx context.Context, target, query string, contextData any) (*QueryResult, error) {
	var ctxRaw json.RawMessage
	if contextData != nil {
		raw, err := json.Marshal(contextData)
		if err != nil {
			return nil, fmt.Errorf("marshal context: %w", err)
		}
		ctxRaw = raw
	}

	params := QueryParams{
		FromAgent: a.Name,
		Query:     query,
		Context:   ctxRaw,
	}
	body, _ := json.Marshal(params)

	msg := Message{
		JSONRPC: Version1,
		ID:      uuid.New().String(),
		Method:  MethodQuery,
		Params:  body,
	}

	resp, err := a.Send(ctx, target, msg)
	if err != nil {
		return nil, err
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("query error: %s", resp.Error.Message)
	}

	var result QueryResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("parse query result: %w", err)
	}
	return &result, nil
}

func (a *AgentNode) Broadcast(ctx context.Context, topic string, payload any) []error {
	body, _ := json.Marshal(BroadcastParams{
		FromAgent: a.Name,
		Topic:     topic,
		Payload:   bodyMessage(payload),
	})
	msg := Message{
		JSONRPC: Version1,
		ID:      uuid.New().String(),
		Method:  MethodBroadcast,
		Params:  body,
	}

	a.mu.RLock()
	peers := make([]*AgentNode, 0, len(a.peers))
	for _, p := range a.peers {
		peers = append(peers, p)
	}
	a.mu.RUnlock()

	var errs []error
	for _, peer := range peers {
		select {
		case peer.messages <- msg:
		default:
			errs = append(errs, fmt.Errorf("peer %q buffer full", peer.Name))
		}
	}
	return errs
}

func (a *AgentNode) ShareContext(ctx context.Context, targets []string, contextData any) error {
	body, _ := json.Marshal(ShareContextParams{
		FromAgent: a.Name,
		Targets:   targets,
		Context:   bodyMessage(contextData),
	})
	msg := Message{
		JSONRPC: Version1,
		ID:      uuid.New().String(),
		Method:  MethodShareContext,
		Params:  body,
	}

	a.mu.RLock()
	defer a.mu.RUnlock()

	for _, target := range targets {
		peer, ok := a.peers[target]
		if !ok {
			continue
		}
		select {
		case peer.messages <- msg:
		default:
			return fmt.Errorf("peer %q buffer full", target)
		}
	}
	return nil
}

var (
	pendingMu sync.Mutex
	pending   = make(map[string]chan Message)
)

func (a *AgentNode) messageLoop() {
	for {
		select {
		case <-a.ctx.Done():
			return
		case msg := <-a.messages:
			a.handleMessage(msg)
		}
	}
}

func (a *AgentNode) handleMessage(msg Message) {
	a.logger("[a2a] %s received %s from %s", a.Name, msg.Method, extractFromAgent(msg))

	switch msg.Method {
	case MethodHandoff, MethodQuery, MethodBroadcast, MethodShareContext, MethodConsensusReq:
		result, err := a.handler(a.ctx, msg)
		resp := Message{
			JSONRPC: Version1,
			ID:      msg.ID,
			Method:  MethodResponse,
		}
		if err != nil {
			resp.Error = &Error{Code: -1, Message: err.Error()}
		} else if result != nil {
			resp.Result, _ = json.Marshal(result)
		}

		pendingMu.Lock()
		if ch, ok := pending[msg.ID]; ok {
			ch <- resp
		}
		pendingMu.Unlock()

	case MethodResponse, MethodConsensusVote, MethodConsensusRes:
		pendingMu.Lock()
		if ch, ok := pending[msg.ID]; ok {
			ch <- msg
		}
		pendingMu.Unlock()

	default:
		a.logger("[a2a] %s unknown method: %s", a.Name, msg.Method)
	}
}

func extractFromAgent(msg Message) string {
	switch msg.Method {
	case MethodHandoff:
		var p HandoffParams
		_ = json.Unmarshal(msg.Params, &p)
		return p.FromAgent
	case MethodQuery:
		var p QueryParams
		_ = json.Unmarshal(msg.Params, &p)
		return p.FromAgent
	case MethodBroadcast:
		var p BroadcastParams
		_ = json.Unmarshal(msg.Params, &p)
		return p.FromAgent
	case MethodShareContext:
		var p ShareContextParams
		_ = json.Unmarshal(msg.Params, &p)
		return p.FromAgent
	case MethodConsensusReq:
		var p ConsensusRequest
		_ = json.Unmarshal(msg.Params, &p)
		return p.FromAgent
	default:
		return strconst.StrUnknown
	}
}

func bodyMessage(v any) json.RawMessage {
	if v == nil {
		return nil
	}
	b, _ := json.Marshal(v)
	return b
}
