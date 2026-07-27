// SPDX-License-Identifier: MIT
package tools

import (
	"context"
	"fmt"
	"testing"
)

func BenchmarkBuild(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		for j := 0; j < 100; j++ {
			_, _ = Build(ToolDef{
				Name: fmt.Sprintf("tool-%d", j),
				Execute: func(ctx Context, input map[string]any) (any, error) {
					return nil, nil
				},
			})
		}
	}
}

func BenchmarkExecute(b *testing.B) {
	b.ReportAllocs()
	tool, err := Build(ToolDef{
		Name: "echo",
		Execute: func(ctx Context, input map[string]any) (any, error) {
			return input["msg"], nil
		},
	})
	if err != nil {
		b.Fatal(err)
	}
	ctx := Context{Context: context.Background()}
	input := map[string]any{"msg": "hello"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < 1000; j++ {
			_, _ = tool.Execute(ctx, input)
		}
	}
}

func BenchmarkRegistryRegister(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		r := New()
		for j := 0; j < 100; j++ {
			tool, err := Build(ToolDef{
				Name: fmt.Sprintf("tool-%d", j),
				Execute: func(ctx Context, input map[string]any) (any, error) {
					return nil, nil
				},
			})
			if err != nil {
				b.Fatal(err)
			}
			_ = r.Register(tool)
		}
	}
}

func BenchmarkRegistryGet(b *testing.B) {
	b.ReportAllocs()
	r := New()
	for j := 0; j < 100; j++ {
		tool, err := Build(ToolDef{
			Name: fmt.Sprintf("tool-%d", j),
			Execute: func(ctx Context, input map[string]any) (any, error) {
				return nil, nil
			},
		})
		if err != nil {
			b.Fatal(err)
		}
		_ = r.Register(tool)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < 1000; j++ {
			r.Get(fmt.Sprintf("tool-%d", j%100))
		}
	}
}
