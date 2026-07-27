// SPDX-License-Identifier: MIT
package compress

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func BenchmarkSnip(b *testing.B) {
	b.ReportAllocs()
	c := New(WithStrategy(StrategySnip), WithMaxTokens(50000), WithCompressionPct(50))
	contents := genContents(1000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Compress(contents)
	}
}

func BenchmarkTrimHead(b *testing.B) {
	b.ReportAllocs()
	c := New(WithStrategy(StrategyTrimHead), WithMaxTokens(50000), WithCompressionPct(50))
	contents := genContents(1000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Compress(contents)
	}
}

func BenchmarkSummarize(b *testing.B) {
	b.ReportAllocs()
	c := New(WithStrategy(StrategySummary), WithMaxTokens(50000), WithCompressionPct(50))
	contents := genContents(1000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Compress(contents)
	}
}

func BenchmarkBudgetAdd(b *testing.B) {
	b.ReportAllocs()
	budget := NewBudget(100000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < 10000; j++ {
			budget.Add(j)
		}
	}
}

func BenchmarkSnapshotMarshal(b *testing.B) {
	b.ReportAllocs()
	ss := NewSnapshotter(time.Hour, 100)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		contents := make([]Content, 100)
		for j := 0; j < 100; j++ {
			contents[j] = Content{
				ID:   fmt.Sprintf("msg-%d", j),
				Role: "user",
				Text: "This is a sample message for the snapshot benchmark " + fmt.Sprintf("%d", j),
				Size: 60,
			}
		}
		ss.Record(fmt.Sprintf("snap-%d", i), contents)
	}
}

func genContents(n int) []Content {
	contents := make([]Content, n)
	for i := 0; i < n; i++ {
		contents[i] = Content{
			ID:   fmt.Sprintf("msg-%d", i),
			Role: []string{"user", "assistant", "tool", "system"}[i%4],
			Text: strings.Repeat("a", 100+i),
			Size: 100 + i,
		}
	}
	return contents
}
