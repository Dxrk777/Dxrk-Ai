// SPDX-License-Identifier: MIT
package a2a

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestAgentHandoff(t *testing.T) {
	a1 := NewAgentNode("agent-a", nil, func(ctx context.Context, msg Message) (any, error) {
		return nil, nil
	})
	defer a1.Stop()

	a2 := NewAgentNode("agent-b", nil, func(ctx context.Context, msg Message) (any, error) {
		var p HandoffParams
		_ = json.Unmarshal(msg.Params, &p)
		if p.Task == "delegate" {
			return HandoffResult{Accepted: true, SessionID: "sess-1"}, nil
		}
		return HandoffResult{Accepted: false, Message: "unknown task"}, nil
	})
	defer a2.Stop()

	a1.AddPeer(a2)
	a2.AddPeer(a1)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := a1.Handoff(ctx, "agent-b", "delegate", nil)
	if err != nil {
		t.Fatalf("handoff failed: %v", err)
	}
	if !result.Accepted {
		t.Fatal("expected handoff to be accepted")
	}
	if result.SessionID != "sess-1" {
		t.Fatalf("expected session_id 'sess-1', got %q", result.SessionID)
	}
}

func TestAgentHandoff_Rejected(t *testing.T) {
	a1 := NewAgentNode("agent-a", nil, func(ctx context.Context, msg Message) (any, error) {
		return nil, nil
	})
	defer a1.Stop()

	a2 := NewAgentNode("agent-b", nil, func(ctx context.Context, msg Message) (any, error) {
		return HandoffResult{Accepted: false, Message: "busy"}, nil
	})
	defer a2.Stop()

	a1.AddPeer(a2)
	a2.AddPeer(a1)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := a1.Handoff(ctx, "agent-b", "task", nil)
	if err != nil {
		t.Fatalf("handoff failed: %v", err)
	}
	if result.Accepted {
		t.Fatal("expected handoff to be rejected")
	}
}

func TestAgentQuery(t *testing.T) {
	a1 := NewAgentNode("agent-a", nil, func(ctx context.Context, msg Message) (any, error) {
		return nil, nil
	})
	defer a1.Stop()

	a2 := NewAgentNode("agent-b", nil, func(ctx context.Context, msg Message) (any, error) {
		var p QueryParams
		_ = json.Unmarshal(msg.Params, &p)
		return QueryResult{Answer: "answer: " + p.Query}, nil
	})
	defer a2.Stop()

	a1.AddPeer(a2)
	a2.AddPeer(a1)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := a1.Query(ctx, "agent-b", "what is 2+2?", nil)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if result.Answer != "answer: what is 2+2?" {
		t.Fatalf("unexpected answer: %q", result.Answer)
	}
}

func TestAgentConsensus(t *testing.T) {
	state := NewConsensusState()
	a1 := NewAgentNode("agent-a", nil, func(ctx context.Context, msg Message) (any, error) {
		switch msg.Method {
		case MethodConsensusReq:
			return handleConsensusRequest(ctx, msg, state, "agent-a")
		default:
			return nil, nil
		}
	})
	defer a1.Stop()

	a2 := NewAgentNode("agent-b", nil, func(ctx context.Context, msg Message) (any, error) {
		switch msg.Method {
		case MethodConsensusReq:
			return handleConsensusRequest(ctx, msg, state, "agent-b")
		default:
			return nil, nil
		}
	})
	defer a2.Stop()

	a3 := NewAgentNode("agent-c", nil, func(ctx context.Context, msg Message) (any, error) {
		switch msg.Method {
		case MethodConsensusReq:
			return handleConsensusRequest(ctx, msg, state, "agent-c")
		default:
			return nil, nil
		}
	})
	defer a3.Stop()

	a1.AddPeer(a2)
	a1.AddPeer(a3)
	a2.AddPeer(a1)
	a3.AddPeer(a1)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := a1.ProposeConsensus(ctx, []string{"agent-b", "agent-c"}, "prop-1",
		"Which model to use?", []string{"gpt-4o", "claude-sonnet-4"}, nil)
	if err != nil {
		t.Fatalf("consensus failed: %v", err)
	}
	if !result.Decided {
		t.Fatal("expected consensus to be decided")
	}
	if len(result.Votes) != 2 {
		t.Fatalf("expected 2 votes, got %d", len(result.Votes))
	}
}

func TestAgentBroadcast(t *testing.T) {
	received := make(chan string, 2)
	a1 := NewAgentNode("agent-a", nil, func(ctx context.Context, msg Message) (any, error) {
		received <- "a1"
		return nil, nil
	})
	defer a1.Stop()

	a2 := NewAgentNode("agent-b", nil, func(ctx context.Context, msg Message) (any, error) {
		received <- "a2"
		return nil, nil
	})
	defer a2.Stop()

	a1.AddPeer(a2)
	a2.AddPeer(a1)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	errs := a1.Broadcast(ctx, "announcement", "hello everyone")
	if len(errs) > 0 {
		t.Fatalf("broadcast errors: %v", errs)
	}

	select {
	case <-received:
	case <-ctx.Done():
		t.Fatal("timeout waiting for broadcast delivery")
	}
}

func TestAgentShareContext(t *testing.T) {
	received := make(chan string, 1)
	a1 := NewAgentNode("agent-a", nil, func(ctx context.Context, msg Message) (any, error) {
		return nil, nil
	})
	defer a1.Stop()

	a2 := NewAgentNode("agent-b", nil, func(ctx context.Context, msg Message) (any, error) {
		received <- "shared"
		return nil, nil
	})
	defer a2.Stop()

	a1.AddPeer(a2)
	a2.AddPeer(a1)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := a1.ShareContext(ctx, []string{"agent-b"}, map[string]string{"key": "value"})
	if err != nil {
		t.Fatalf("share context failed: %v", err)
	}

	select {
	case <-received:
	case <-ctx.Done():
		t.Fatal("timeout waiting for context share")
	}
}

func TestAgentCapabilities(t *testing.T) {
	caps := []Capability{
		{Name: "coding", Description: "write code", Tools: []string{"sandbox_run_code"}},
		{Name: "research", Description: "search web", Tools: []string{"web_search"}},
	}
	a := NewAgentNode("agent-a", caps, nil)
	defer a.Stop()

	if len(a.Capabilities) != 2 {
		t.Fatalf("expected 2 capabilities, got %d", len(a.Capabilities))
	}
	if a.Capabilities[0].Name != "coding" {
		t.Fatalf("expected 'coding', got %q", a.Capabilities[0].Name)
	}
}
