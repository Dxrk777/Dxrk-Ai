//go:build docker

// SPDX-License-Identifier: MIT
package sandbox

import (
	"testing"
)

func BenchmarkImageForLanguage(b *testing.B) {
	langs := []Language{LanguageGo, LanguagePython, LanguageNode, LanguageBash, LanguageRust, LanguageTypeScript}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ImageForLanguage(langs[i%len(langs)])
	}
}

func BenchmarkCommandForLanguage(b *testing.B) {
	langs := []Language{LanguageGo, LanguagePython, LanguageNode, LanguageBash, LanguageRust, LanguageTypeScript}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CommandForLanguage(langs[i%len(langs)], "main.go")
	}
}

func BenchmarkDefaultPoolConfig(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		DefaultPoolConfig()
	}
}

func BenchmarkNewPool(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := NewPool(PoolConfig{
			MaxContainers: 5,
			IdleTimeout:   300000000000,
			DockerCmd:     "docker",
			DefaultImage:  "alpine:latest",
		})
		if err != nil {
			b.Skip("docker not available")
		}
	}
}

func BenchmarkPoolStats(b *testing.B) {
	p, err := NewPool(DefaultPoolConfig())
	if err != nil {
		b.Skip("docker not available")
	}
	defer func() { _ = p.Close() }()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Stats()
	}
}

func BenchmarkScriptFileName(b *testing.B) {
	langs := []Language{LanguageGo, LanguagePython, LanguageNode, LanguageBash, LanguageRust, LanguageTypeScript}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		scriptFileName(langs[i%len(langs)])
	}
}
