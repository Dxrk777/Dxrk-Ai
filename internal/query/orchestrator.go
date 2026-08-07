// SPDX-License-Identifier: MIT
package query

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/Dxrk777/Dxrk/internal/log"
	"github.com/Dxrk777/Dxrk/internal/tools"
)

// Orchestrator executes tool_use blocks from the LLM,
// running concurrent-safe tools in parallel and serial tools sequentially.
type Orchestrator struct {
	registry *tools.Registry
	toolCtx  tools.Context
}

// NewOrchestrator creates a tool orchestrator.
func NewOrchestrator(registry *tools.Registry, toolCtx ...tools.Context) *Orchestrator {
	o := &Orchestrator{registry: registry}
	if len(toolCtx) > 0 {
		o.toolCtx = toolCtx[0]
	}
	return o
}

// Execute runs all tool_use blocks, returning result blocks.
// Concurrent-safe tools (read-only) run in parallel;
// non-concurrent-safe tools run sequentially in order.
func (o *Orchestrator) Execute(ctx context.Context, blocks []ToolUseBlock) ([]ToolResultBlock, error) {
	if len(blocks) == 0 {
		return nil, nil
	}

	results := make([]ToolResultBlock, 0, len(blocks))

	// Partition: concurrent-safe (read-only) vs serial (mutating)
	var concurrent, serial []ToolUseBlock
	for _, block := range blocks {
		tool, ok := o.registry.Get(block.Name)
		if !ok {
			results = append(results, ToolResultBlock{
				ToolUseID: block.ID,
				Name:      block.Name,
				Content:   fmt.Sprintf("unknown tool: %q", block.Name),
				IsError:   true,
			})
			continue
		}
		if tool.IsConcurrentSafe() {
			concurrent = append(concurrent, block)
		} else {
			serial = append(serial, block)
		}
	}

	// Run concurrent-safe tools in parallel
	if len(concurrent) > 0 {
		cr := o.executeConcurrent(ctx, concurrent)
		results = append(results, cr...)
	}

	// Run serial tools in order
	for _, block := range serial {
		r := o.executeOne(ctx, block)
		results = append(results, r)
	}

	return results, nil
}

func (o *Orchestrator) executeConcurrent(ctx context.Context, blocks []ToolUseBlock) []ToolResultBlock {
	var wg sync.WaitGroup
	results := make([]ToolResultBlock, len(blocks))

	for i, block := range blocks {
		wg.Add(1)
		go func(i int, block ToolUseBlock) {
			defer wg.Done()
			results[i] = o.executeOne(ctx, block)
		}(i, block)
	}

	wg.Wait()
	return results
}

func (o *Orchestrator) executeOne(ctx context.Context, block ToolUseBlock) ToolResultBlock {
	tool, ok := o.registry.Get(block.Name)
	if !ok {
		return ToolResultBlock{
			ToolUseID: block.ID,
			Name:      block.Name,
			Content:   fmt.Sprintf("unknown tool: %q", block.Name),
			IsError:   true,
		}
	}

	tCtx := o.toolCtx
	tCtx.Context = ctx
	if tCtx.Logger == nil {
		tCtx.Logger = log.NewNop()
	}

	result, err := tool.Execute(tCtx, block.Input)
	if err != nil {
		return ToolResultBlock{
			ToolUseID: block.ID,
			Name:      block.Name,
			Content:   fmt.Sprintf("error: %v", err),
			IsError:   true,
		}
	}

	content, err := marshalResult(result)
	if err != nil {
		return ToolResultBlock{
			ToolUseID: block.ID,
			Name:      block.Name,
			Content:   fmt.Sprintf("error marshaling result: %v", err),
			IsError:   true,
		}
	}

	return ToolResultBlock{
		ToolUseID: block.ID,
		Name:      block.Name,
		Content:   content,
		IsError:   false,
	}
}

func marshalResult(v any) (string, error) {
	switch val := v.(type) {
	case string:
		return val, nil
	case []byte:
		return string(val), nil
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
}
