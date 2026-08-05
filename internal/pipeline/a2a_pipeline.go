// SPDX-License-Identifier: MIT
package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/Dxrk777/Dxrk-Ai/internal/a2a"
	"github.com/Dxrk777/Dxrk-Ai/internal/query"
	"github.com/Dxrk777/Dxrk-Ai/internal/router"
	"github.com/Dxrk777/Dxrk-Ai/internal/strconst"
)

const PipelineVersion = "1.0"

type PipelineRole string

const (
	RoleCoder    PipelineRole = "coder"
	RoleTester   PipelineRole = "tester"
	RoleReviewer PipelineRole = "reviewer"
	RoleDocs     PipelineRole = "docs"
)

var pipelineRoles = []PipelineRole{RoleCoder, RoleTester, RoleReviewer, RoleDocs}

type PipelineConfig struct {
	Router        *router.Router
	MaxIterations int
	Logger        func(format string, args ...any)
	CodePrompt    string
	TestPrompt    string
	ReviewPrompt  string
	DocsPrompt    string
	Timeouts      time.Duration
}

func DefaultPipelineConfig(rtr *router.Router) PipelineConfig {
	return PipelineConfig{
		Router:        rtr,
		MaxIterations: 3,
		Logger:        func(string, ...any) {},
		CodePrompt:    "You are a senior software engineer. Write clean, idiomatic, production-ready code.",
		TestPrompt:    "You are a QA engineer. Write thorough tests covering edge cases and error paths.",
		ReviewPrompt:  "You are a code reviewer. Check for bugs, security issues, performance, style, and correctness.",
		DocsPrompt:    "You are a technical writer. Write clear, comprehensive documentation.",
		Timeouts:      60 * time.Second,
	}
}

type PipelineTask struct {
	ID          string          `json:"id"`
	Description string          `json:"description"`
	Language    string          `json:"language"`
	Spec        json.RawMessage `json:"spec,omitempty"`
}

type PipelineResult struct {
	TaskID     string        `json:"task_id"`
	Code       string        `json:"code"`
	Tests      string        `json:"tests"`
	ReviewLog  string        `json:"review_log"`
	Docs       string        `json:"docs"`
	Iterations int           `json:"iterations"`
	Success    bool          `json:"success"`
	DurationMs int64         `json:"duration_ms"`
	Stages     []StageReport `json:"stages"`
}

type StageReport struct {
	Role       PipelineRole `json:"role"`
	Status     string       `json:"status"`
	Output     string       `json:"output,omitempty"`
	Error      string       `json:"error,omitempty"`
	DurationMs int64        `json:"duration_ms"`
}

type workUnit struct {
	task      PipelineTask
	code      string
	tests     string
	reviewLog string
	docs      string
	iter      int
	mu        sync.Mutex
	reports   []StageReport
}

type Pipeline struct {
	cfg   PipelineConfig
	nodes map[PipelineRole]*a2a.AgentNode
}

func NewPipeline(cfg PipelineConfig) *Pipeline {
	p := &Pipeline{
		cfg:   cfg,
		nodes: make(map[PipelineRole]*a2a.AgentNode),
	}

	opts := []a2a.AgentOption{a2a.WithAgentLogger(cfg.Logger)}

	coderNode := a2a.NewAgentNode(string(RoleCoder),
		[]a2a.Capability{{Name: "coding", Description: cfg.CodePrompt}},
		p.makeHandler(RoleCoder, cfg.CodePrompt), opts...)

	testerNode := a2a.NewAgentNode(string(RoleTester),
		[]a2a.Capability{{Name: "testing", Description: cfg.TestPrompt}},
		p.makeHandler(RoleTester, cfg.TestPrompt), opts...)

	reviewerNode := a2a.NewAgentNode(string(RoleReviewer),
		[]a2a.Capability{{Name: "reviewing", Description: cfg.ReviewPrompt}},
		p.makeHandler(RoleReviewer, cfg.ReviewPrompt), opts...)

	docsNode := a2a.NewAgentNode(string(RoleDocs),
		[]a2a.Capability{{Name: "docs", Description: cfg.DocsPrompt}},
		p.makeHandler(RoleDocs, cfg.DocsPrompt), opts...)

	for _, n := range []*a2a.AgentNode{coderNode, testerNode, reviewerNode, docsNode} {
		p.nodes[PipelineRole(n.Name)] = n
	}

	for _, a := range p.nodes {
		for _, b := range p.nodes {
			if a.Name != b.Name {
				a.AddPeer(b)
			}
		}
	}

	return p
}

func (p *Pipeline) Stop() {
	for _, n := range p.nodes {
		n.Stop()
	}
}

func (p *Pipeline) Execute(ctx context.Context, task PipelineTask) PipelineResult {
	start := time.Now()
	wu := &workUnit{task: task}

	for wu.iter = 1; wu.iter <= p.cfg.MaxIterations; wu.iter++ {
		codeRes := p.runNode(ctx, wu, RoleCoder, func() (string, error) {
			return p.llmGenerate(ctx, RoleCoder, buildCodePrompt(task, wu.code, wu.tests, wu.reviewLog))
		})
		if codeRes.Error != "" {
			p.cfg.Logger("[pipeline] iter %d coder failed: %s", wu.iter, codeRes.Error)
			continue
		}
		wu.mu.Lock()
		wu.code = codeRes.Output
		wu.reports = append(wu.reports, codeRes)
		wu.mu.Unlock()

		testRes := p.runNode(ctx, wu, RoleTester, func() (string, error) {
			return p.llmGenerate(ctx, RoleTester, buildTestPrompt(task, wu.code))
		})
		if testRes.Error != "" {
			p.cfg.Logger("[pipeline] iter %d tests failed: %s", wu.iter, testRes.Error)
			wu.mu.Lock()
			wu.reports = append(wu.reports, testRes)
			wu.mu.Unlock()
			continue
		}
		wu.mu.Lock()
		wu.tests = testRes.Output
		wu.reports = append(wu.reports, testRes)
		wu.mu.Unlock()

		reviewRes := p.runNode(ctx, wu, RoleReviewer, func() (string, error) {
			return p.llmGenerate(ctx, RoleReviewer, buildReviewPrompt(task, wu.code, wu.tests))
		})
		if reviewRes.Error != "" {
			wu.mu.Lock()
			wu.reports = append(wu.reports, reviewRes)
			wu.mu.Unlock()
			continue
		}
		wu.mu.Lock()
		wu.reviewLog = reviewRes.Output
		wu.reports = append(wu.reports, reviewRes)
		wu.mu.Unlock()

		if p.hasIssues(wu.reviewLog) {
			p.cfg.Logger("[pipeline] review found issues at iter %d, retrying", wu.iter)
			continue
		}

		docsRes := p.runNode(ctx, wu, RoleDocs, func() (string, error) {
			return p.llmGenerate(ctx, RoleDocs, buildDocsPrompt(task, wu.code, wu.tests))
		})
		wu.mu.Lock()
		wu.docs = docsRes.Output
		wu.reports = append(wu.reports, docsRes)
		wu.mu.Unlock()

		elapsed := time.Since(start)
		return PipelineResult{
			TaskID:     task.ID,
			Code:       wu.code,
			Tests:      wu.tests,
			ReviewLog:  wu.reviewLog,
			Docs:       wu.docs,
			Iterations: wu.iter,
			Success:    true,
			DurationMs: elapsed.Milliseconds(),
			Stages:     wu.reports,
		}
	}

	return PipelineResult{
		TaskID:     task.ID,
		Code:       wu.code,
		Tests:      wu.tests,
		ReviewLog:  wu.reviewLog,
		Docs:       wu.docs,
		Iterations: wu.iter - 1,
		Success:    false,
		DurationMs: time.Since(start).Milliseconds(),
		Stages:     wu.reports,
	}
}

func (p *Pipeline) runNode(_ context.Context, _ *workUnit, role PipelineRole, fn func() (string, error)) StageReport {
	start := time.Now()
	output, err := fn()
	duration := time.Since(start)

	report := StageReport{
		Role:       role,
		DurationMs: duration.Milliseconds(),
	}
	if err != nil {
		report.Status = strconst.StrFailed
		report.Error = err.Error()
	} else {
		report.Status = strconst.StrCompleted
		report.Output = output
	}
	return report
}

func (p *Pipeline) llmGenerate(ctx context.Context, role PipelineRole, prompt string) (string, error) {
	if p.cfg.Router == nil {
		return "", fmt.Errorf("no LLM router configured for pipeline")
	}

	messages := []query.Message{
		{Role: strconst.StrSystem, Content: p.systemPromptForRole(role)},
		{Role: "user", Content: prompt},
	}

	resp, err := p.cfg.Router.Generate(ctx, messages, nil)
	if err != nil {
		return "", fmt.Errorf("llm generate: %w", err)
	}

	return resp.Text, nil
}

func (p *Pipeline) systemPromptForRole(role PipelineRole) string {
	switch role {
	case RoleCoder:
		return p.cfg.CodePrompt
	case RoleTester:
		return p.cfg.TestPrompt
	case RoleReviewer:
		return p.cfg.ReviewPrompt
	case RoleDocs:
		return p.cfg.DocsPrompt
	default:
		return "You are an AI assistant."
	}
}

func (p *Pipeline) makeHandler(role PipelineRole, systemPrompt string) a2a.MessageHandler {
	return func(ctx context.Context, msg a2a.Message) (any, error) {
		switch msg.Method {
		case a2a.MethodHandoff:
			var params a2a.HandoffParams
			if err := json.Unmarshal(msg.Params, &params); err != nil {
				return nil, err
			}
			if p.cfg.Logger != nil {
				p.cfg.Logger("[pipeline] %s handoff: task=%q from=%s", role, params.Task, params.FromAgent)
			}
			return a2a.HandoffResult{Accepted: true, SessionID: fmt.Sprintf("%s-%s", role, params.FromAgent)}, nil

		case a2a.MethodQuery:
			var params a2a.QueryParams
			if err := json.Unmarshal(msg.Params, &params); err != nil {
				return nil, err
			}
			messages := []query.Message{
				{Role: strconst.StrSystem, Content: systemPrompt},
				{Role: "user", Content: params.Query},
			}
			resp, err := p.cfg.Router.Generate(ctx, messages, nil)
			if err != nil {
				return a2a.QueryResult{Answer: fmt.Sprintf("error: %v", err)}, nil
			}
			return a2a.QueryResult{Answer: resp.Text}, nil

		case a2a.MethodShareContext:
			var params a2a.ShareContextParams
			if err := json.Unmarshal(msg.Params, &params); err != nil {
				return nil, fmt.Errorf("bad share_context params: %w", err)
			}
			p.cfg.Logger("[pipeline] %s received context from %s", role, params.FromAgent)
			return map[string]string{strconst.StrStatus: "received"}, nil

		default:
			return nil, fmt.Errorf("pipeline agent %s: unhandled method %s", role, msg.Method)
		}
	}
}

func (p *Pipeline) hasIssues(reviewLog string) bool {
	return len(reviewLog) > 0
}

func buildCodePrompt(task PipelineTask, prevCode, prevTests, reviewLog string) string {
	prompt := fmt.Sprintf("Implement the following in %s:\n\n%s\n\n", task.Language, task.Description)
	if task.Spec != nil {
		prompt += fmt.Sprintf("Spec:\n%s\n\n", string(task.Spec))
	}
	if prevCode != "" {
		prompt += fmt.Sprintf("Previous code (needs improvement):\n%s\n\n", prevCode)
	}
	if prevTests != "" {
		prompt += fmt.Sprintf("Tests:\n%s\n\n", prevTests)
	}
	if reviewLog != "" {
		prompt += fmt.Sprintf("Previous review comments to address:\n%s\n\n", reviewLog)
	}
	prompt += "Return ONLY the code wrapped in triple backticks. No explanation."
	return prompt
}

func buildTestPrompt(task PipelineTask, code string) string {
	return fmt.Sprintf("Write tests for the following %s code:\n\n%s\n\nReturn ONLY the test code wrapped in triple backticks.",
		task.Language, code)
}

func buildReviewPrompt(task PipelineTask, code, tests string) string {
	return fmt.Sprintf("Review the following %s code and tests. List bugs, security issues, style problems, and suggestions:\n\nCODE:\n%s\n\nTESTS:\n%s",
		task.Language, code, tests)
}

func buildDocsPrompt(task PipelineTask, code, tests string) string {
	return fmt.Sprintf("Write documentation for the following %s code and tests:\n\nCODE:\n%s\n\nTESTS:\n%s\n\nReturn documentation in markdown format.",
		task.Language, code, tests)
}
