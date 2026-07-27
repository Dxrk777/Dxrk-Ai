// SPDX-License-Identifier: MIT
package a2a

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

type ConsensusProposal struct {
	ID       string
	Proposal string
	Options  []string
	Detail   json.RawMessage
	Deadline time.Time
}

type ConsensusState struct {
	mu        sync.Mutex
	proposals map[string]*activeProposal
}

type activeProposal struct {
	proposal ConsensusProposal
	votes    []ConsensusVote
	from     string
	decided  bool
}

func NewConsensusState() *ConsensusState {
	return &ConsensusState{
		proposals: make(map[string]*activeProposal),
	}
}

func (a *AgentNode) ProposeConsensus(ctx context.Context, targets []string, proposalID, proposal string, options []string, detail any) (*ConsensusResult, error) {
	var detailRaw json.RawMessage
	if detail != nil {
		detailRaw, _ = json.Marshal(detail)
	}

	params := ConsensusRequest{
		FromAgent:  a.Name,
		ProposalID: proposalID,
		Proposal:   proposal,
		Options:    options,
		Detail:     detailRaw,
	}
	body, _ := json.Marshal(params)

	msg := Message{
		JSONRPC: Version1,
		ID:      uuid.New().String(),
		Method:  MethodConsensusReq,
		Params:  body,
	}

	a.mu.RLock()
	peers := make([]*AgentNode, 0, len(targets))
	for _, t := range targets {
		if p, ok := a.peers[t]; ok {
			peers = append(peers, p)
		}
	}
	a.mu.RUnlock()

	if len(peers) == 0 {
		return nil, fmt.Errorf("no target peers found")
	}

	votes := make([]ConsensusVote, 0, len(peers))
	for _, peer := range peers {
		resp, err := a.Send(ctx, peer.Name, msg)
		if err != nil {
			continue
		}
		if resp.Error != nil {
			continue
		}
		var vote ConsensusVote
		if err := json.Unmarshal(resp.Result, &vote); err != nil {
			continue
		}
		votes = append(votes, vote)
	}

	outcome := a.resolveConsensus(proposal, options, votes)
	return &ConsensusResult{
		ProposalID: proposalID,
		Decided:    outcome != "",
		Outcome:    outcome,
		Votes:      votes,
		Summary:    fmt.Sprintf("Consensus on %q: %s (%d/%d votes)", proposal, outcome, len(votes), len(peers)),
	}, nil
}

func (a *AgentNode) resolveConsensus(_ string, options []string, votes []ConsensusVote) string {
	if len(votes) == 0 {
		return ""
	}

	counts := make(map[string]int)
	for _, v := range votes {
		counts[v.Vote]++
	}

	majority := len(votes)/2 + 1
	for _, opt := range options {
		if counts[opt] >= majority {
			return opt
		}
	}
	return ""
}

func handleConsensusRequest(_ context.Context, msg Message, state *ConsensusState, agentName string) (any, error) {
	var req ConsensusRequest
	if err := json.Unmarshal(msg.Params, &req); err != nil {
		return nil, fmt.Errorf("parse consensus request: %w", err)
	}

	state.mu.Lock()
	state.proposals[req.ProposalID] = &activeProposal{
		proposal: ConsensusProposal{
			ID:       req.ProposalID,
			Proposal: req.Proposal,
			Options:  req.Options,
			Detail:   req.Detail,
			Deadline: time.Now().Add(30 * time.Second),
		},
		from:    req.FromAgent,
		votes:   nil,
		decided: false,
	}
	state.mu.Unlock()

	vote := ConsensusVote{
		AgentID:    agentName,
		ProposalID: req.ProposalID,
		Vote:       req.Options[0],
		Reason:     "auto-accepted",
	}
	return vote, nil
}
