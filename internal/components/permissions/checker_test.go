// SPDX-License-Identifier: MIT
package permissions

import (
	"testing"
)

func TestChecker_DenyByDefault(t *testing.T) {
	c := NewChecker()
	result := c.Check(ActionRead, "/any/file")
	if result.Allowed {
		t.Fatal("expected deny by default")
	}
	if result.Reason != "denied by default (no matching rule)" {
		t.Fatalf("Reason = %q", result.Reason)
	}
}

func TestChecker_AllowByDefault(t *testing.T) {
	c := NewChecker(WithDenyByDefault(false))
	result := c.Check(ActionRead, "/any/file")
	if !result.Allowed {
		t.Fatal("expected allow by default")
	}
}

func TestChecker_ExplicitAllow(t *testing.T) {
	c := NewChecker(WithRules(AllowRule(ActionRead, "/safe/*")))
	result := c.Check(ActionRead, "/safe/file.txt")
	if !result.Allowed {
		t.Fatal("expected allow for matching rule")
	}
}

func TestChecker_ExplicitDeny(t *testing.T) {
	c := NewChecker(WithRules(
		DenyRule(ActionRead, "*.env"),
		AllowRule(ActionRead, "*"),
	))
	result := c.Check(ActionRead, ".env")
	if result.Allowed {
		t.Fatal("expected deny for .env")
	}
	result = c.Check(ActionRead, "main.go")
	if !result.Allowed {
		t.Fatal("expected allow for main.go")
	}
}

func TestChecker_RuleOrderFirstMatchWins(t *testing.T) {
	c := NewChecker(WithRules(
		DenyRule(ActionRead, "*"),
		AllowRule(ActionRead, "/safe/*"),
	))
	result := c.Check(ActionRead, "/safe/file.txt")
	if result.Allowed {
		t.Fatal("deny all should match first")
	}
}

func TestChecker_ActionSpecificity(t *testing.T) {
	c := NewChecker(WithRules(
		AllowRule(ActionRead, "*"),
		DenyRule(ActionExec, "git push *"),
	))
	readResult := c.Check(ActionRead, "git push origin main")
	if !readResult.Allowed {
		t.Fatal("read should be allowed")
	}
	execResult := c.Check(ActionExec, "git push origin main")
	if execResult.Allowed {
		t.Fatal("exec git push should be denied")
	}
}

func TestChecker_Presets(t *testing.T) {
	c := NewChecker(WithDenyByDefault(false), WithRules(
		DenyEnvFiles,
		DenySecretsDir,
	))
	tests := []struct {
		target  string
		allowed bool
	}{
		{".env", false},
		{".env.prod", true}, // *.env does not match .env.prod
		{"project/secrets/keys.json", false},
		{"/safe/file.txt", true},
	}
	for _, tt := range tests {
		result := c.Check(ActionRead, tt.target)
		if result.Allowed != tt.allowed {
			t.Errorf("Check(ActionRead, %q) = %v, want %v", tt.target, result.Allowed, tt.allowed)
		}
	}
}

func TestMatchRecursive(t *testing.T) {
	tests := []struct {
		pattern string
		target  string
		match   bool
	}{
		{"**/secrets/**", "project/secrets/keys.json", true},
		{"**/secrets/**", "project/src/file.go", false},
		{"**/secrets/**", "secrets/keys.json", true},
		{"/abs/**/secrets/**", "/abs/path/secrets/deep/file.txt", true},
	}
	for _, tt := range tests {
		got := matchRecursive(tt.pattern, tt.target)
		if got != tt.match {
			t.Errorf("matchRecursive(%q, %q) = %v, want %v", tt.pattern, tt.target, got, tt.match)
		}
	}
}

func TestMatchTarget_Wildcard(t *testing.T) {
	if !matchTarget("*", "anything") {
		t.Fatal("* should match anything")
	}
}

func TestMatchTarget_Glob(t *testing.T) {
	if !matchTarget("*.env", ".env") {
		t.Fatal("*.env should match .env")
	}
	if matchTarget("*.env", "file.txt") {
		t.Fatal("*.env should not match .txt")
	}
}
