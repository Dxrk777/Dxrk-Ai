// SPDX-License-Identifier: MIT
package mcp

import (
	"fmt"
	"io"
	"net/http"
	"sync"
	"testing"

	"github.com/Dxrk777/Dxrk/internal/tools"
)

func TestMetrics_IncRateLimitedCalls(t *testing.T) {
	pm := NewPrometheusMetrics()
	pm.IncRateLimitedCalls("tool_a")
	pm.IncRateLimitedCalls("tool_b")
	pm.IncRateLimitedCalls("tool_a")

	snap := pm.Snapshot()
	total := snap["rate_limited_total"].(int64)
	if total != 3 {
		t.Fatalf("total = %d, want 3", total)
	}
	byTool := snap["rate_limited_by_tool"].(map[string]int64)
	if byTool["tool_a"] != 2 {
		t.Fatalf("tool_a count = %d, want 2", byTool["tool_a"])
	}
	if byTool["tool_b"] != 1 {
		t.Fatalf("tool_b count = %d, want 1", byTool["tool_b"])
	}
}

func TestMetrics_SetTokensRemaining(t *testing.T) {
	pm := NewPrometheusMetrics()
	pm.SetTokensRemaining(5)

	snap := pm.Snapshot()
	tokens := snap["rate_limiter_tokens_remaining"].(int64)
	if tokens != 5 {
		t.Fatalf("tokens = %d, want 5", tokens)
	}
}

func TestMetrics_Snapshot(t *testing.T) {
	pm := NewPrometheusMetrics()
	pm.IncRateLimitedCalls("tool_x")
	pm.SetTokensRemaining(3)

	snap := pm.Snapshot()
	if _, ok := snap["rate_limited_total"]; !ok {
		t.Fatal("snapshot missing rate_limited_total")
	}
	if _, ok := snap["rate_limiter_tokens_remaining"]; !ok {
		t.Fatal("snapshot missing rate_limiter_tokens_remaining")
	}
	if _, ok := snap["rate_limited_by_tool"]; !ok {
		t.Fatal("snapshot missing rate_limited_by_tool")
	}
}

func TestServerWithMetrics(t *testing.T) {
	reg := tools.New()
	pm := NewPrometheusMetrics()
	s := NewServer(reg, nil, io.Discard, ServerWithMetrics(pm))
	if s.metrics != pm {
		t.Fatal("ServerWithMetrics did not set metrics exporter")
	}
}

func TestMetrics_Concurrent(t *testing.T) {
	pm := NewPrometheusMetrics()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			toolName := fmt.Sprintf("tool_%d", n%10)
			pm.IncRateLimitedCalls(toolName)
		}(i)
	}
	wg.Wait()

	snap := pm.Snapshot()
	total := snap["rate_limited_total"].(int64)
	if total != 100 {
		t.Fatalf("total = %d, want 100", total)
	}
	byTool := snap["rate_limited_by_tool"].(map[string]int64)
	sum := int64(0)
	for _, v := range byTool {
		sum += v
	}
	if sum != 100 {
		t.Fatalf("sum of tool counts = %d, want 100", sum)
	}
}

func TestServerHandleMetrics_Option(t *testing.T) {
	reg := tools.New()
	handler := func(w http.ResponseWriter, r *http.Request) {}
	s := NewServer(reg, nil, io.Discard, ServerHandleMetrics(handler))
	if s.metricsHandler == nil {
		t.Fatal("ServerHandleMetrics did not set metricsHandler")
	}
}

func TestMetrics_SnapshotDeterministic(t *testing.T) {
	pm := NewPrometheusMetrics()
	pm.IncRateLimitedCalls("z_tool")
	pm.IncRateLimitedCalls("a_tool")
	pm.SetTokensRemaining(1)

	snap1 := pm.Snapshot()
	snap2 := pm.Snapshot()

	total1 := snap1["rate_limited_total"].(int64)
	total2 := snap2["rate_limited_total"].(int64)
	if total1 != total2 {
		t.Fatalf("non-deterministic total: %d vs %d", total1, total2)
	}
}

func TestRateLimiter_Tokens(t *testing.T) {
	rl := NewRateLimiter(10, 100)
	tokens := rl.Tokens()
	if tokens < 9.9 || tokens > 10.1 {
		t.Fatalf("Tokens() = %f, want ~10", tokens)
	}
	rl.Allow()
	rl.Allow()
	tokens = rl.Tokens()
	if tokens < 7.9 || tokens > 8.1 {
		t.Fatalf("Tokens() = %f, want ~8", tokens)
	}
}
