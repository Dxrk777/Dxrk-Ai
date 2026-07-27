// SPDX-License-Identifier: MIT
package a2a

import "encoding/json"

type ProtocolVersion string

const Version1 ProtocolVersion = "2.0"

type Method string

const (
	MethodHandoff       Method = "a2a.handoff"
	MethodQuery         Method = "a2a.query"
	MethodResponse      Method = "a2a.response"
	MethodBroadcast     Method = "a2a.broadcast"
	MethodConsensusReq  Method = "a2a.consensus.request"
	MethodConsensusVote Method = "a2a.consensus.vote"
	MethodConsensusRes  Method = "a2a.consensus.result"
	MethodHeartbeat     Method = "a2a.heartbeat"
	MethodCapabilities  Method = "a2a.capabilities"
	MethodShareContext  Method = "a2a.share_context"
)

type Message struct {
	JSONRPC ProtocolVersion `json:"jsonrpc"`
	ID      string          `json:"id"`
	Method  Method          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
}

type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type Capability struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Tools       []string `json:"tools,omitempty"`
	Models      []string `json:"models,omitempty"`
}

type HandoffParams struct {
	FromAgent string          `json:"from_agent"`
	ToAgent   string          `json:"to_agent"`
	Task      string          `json:"task"`
	Context   json.RawMessage `json:"context,omitempty"`
}

type HandoffResult struct {
	Accepted  bool   `json:"accepted"`
	Message   string `json:"message,omitempty"`
	SessionID string `json:"session_id,omitempty"`
}

type QueryParams struct {
	FromAgent string          `json:"from_agent"`
	Query     string          `json:"query"`
	Context   json.RawMessage `json:"context,omitempty"`
}

type QueryResult struct {
	Answer string          `json:"answer"`
	Data   json.RawMessage `json:"data,omitempty"`
}

type BroadcastParams struct {
	FromAgent string          `json:"from_agent"`
	Topic     string          `json:"topic"`
	Payload   json.RawMessage `json:"payload"`
}

type ShareContextParams struct {
	FromAgent string          `json:"from_agent"`
	Targets   []string        `json:"targets"`
	Context   json.RawMessage `json:"context"`
}

type ConsensusRequest struct {
	FromAgent  string          `json:"from_agent"`
	ProposalID string          `json:"proposal_id"`
	Proposal   string          `json:"proposal"`
	Options    []string        `json:"options"`
	Detail     json.RawMessage `json:"detail,omitempty"`
}

type ConsensusVote struct {
	AgentID    string `json:"agent_id"`
	ProposalID string `json:"proposal_id"`
	Vote       string `json:"vote"`
	Reason     string `json:"reason,omitempty"`
}

type ConsensusResult struct {
	ProposalID string          `json:"proposal_id"`
	Decided    bool            `json:"decided"`
	Outcome    string          `json:"outcome"`
	Votes      []ConsensusVote `json:"votes"`
	Summary    string          `json:"summary,omitempty"`
}
