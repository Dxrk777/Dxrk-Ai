// SPDX-License-Identifier: MIT
package cli

import (
	"strings"
	"testing"
)

func TestRunCompletion_Bash(t *testing.T) {
	out, err := RunCompletion("bash")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "dxrk") {
		t.Errorf("bash completion should contain 'dxrk', got:\n%s", out)
	}
}

func TestRunCompletion_Zsh(t *testing.T) {
	out, err := RunCompletion("zsh")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "compdef") {
		t.Errorf("zsh completion should contain 'compdef', got:\n%s", out)
	}
}

func TestRunCompletion_Fish(t *testing.T) {
	out, err := RunCompletion("fish")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "complete -c dxrk") {
		t.Errorf("fish completion should contain 'complete -c dxrk', got:\n%s", out)
	}
}

func TestRunCompletion_Unsupported(t *testing.T) {
	_, err := RunCompletion("tcsh")
	if err == nil {
		t.Fatal("expected error for unsupported shell, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported") {
		t.Errorf("error should mention 'unsupported', got: %v", err)
	}
}
