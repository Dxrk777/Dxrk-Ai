// SPDX-License-Identifier: MIT
package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Dxrk777/Dxrk-Ai/internal/model"
	"github.com/Dxrk777/Dxrk-Ai/internal/planner"
	"github.com/Dxrk777/Dxrk-Ai/internal/verify"
)

func TestEngramPathGuidanceFish(t *testing.T) {
	msg := dxrkMemoryPathGuidance("/usr/bin/fish")
	if want := "fish_user_paths"; !strings.Contains(msg, want) {
		t.Fatalf("dxrkMemoryPathGuidance(fish) missing %q: %s", want, msg)
	}
}

func TestEngramPathGuidanceZsh(t *testing.T) {
	msg := dxrkMemoryPathGuidance("/bin/zsh")
	if want := ".zshrc"; !strings.Contains(msg, want) {
		t.Fatalf("dxrkMemoryPathGuidance(zsh) missing %q: %s", want, msg)
	}
}

func TestEngramPathGuidanceDefault(t *testing.T) {
	msg := dxrkMemoryPathGuidance("")
	if want := "go/bin"; !strings.Contains(msg, want) {
		t.Fatalf("dxrkMemoryPathGuidance(default) missing %q: %s", want, msg)
	}
}

func TestGoInstallBinDirDefaultsToHomeGoBin(t *testing.T) {
	t.Setenv("GOBIN", "")
	t.Setenv("GOPATH", "")

	dir := goInstallBinDir()
	home, _ := os.UserHomeDir()
	want := filepath.Join(home, "go", "bin")
	if dir != want {
		t.Fatalf("goInstallBinDir() = %q, want %q", dir, want)
	}
}

func TestGoInstallBinDirRespectsGOBIN(t *testing.T) {
	t.Setenv("GOBIN", "/custom/gobin")
	dir := goInstallBinDir()
	if dir != "/custom/gobin" {
		t.Fatalf("goInstallBinDir() = %q, want /custom/gobin", dir)
	}
}

func TestGoInstallBinDirRespectsGOPATH(t *testing.T) {
	t.Setenv("GOBIN", "")
	t.Setenv("GOPATH", "/custom/gopath")
	dir := goInstallBinDir()
	want := filepath.Join("/custom/gopath", "bin")
	if dir != want {
		t.Fatalf("goInstallBinDir() = %q, want %q", dir, want)
	}
}

func TestIsInPATH(t *testing.T) {
	t.Setenv("PATH", "/usr/bin"+string(os.PathListSeparator)+"/home/user/go/bin")
	if !isInPATH("/home/user/go/bin") {
		t.Fatal("isInPATH should return true for entry in PATH")
	}
	if isInPATH("/not/in/path") {
		t.Fatal("isInPATH should return false for entry not in PATH")
	}
}

func TestWithGoInstallPathNoteAddsNoteWhenNotInPATH(t *testing.T) {
	t.Setenv("GOBIN", "")
	t.Setenv("GOPATH", "")
	// Set PATH to something that does NOT contain ~/go/bin.
	t.Setenv("PATH", "/usr/bin:/usr/local/bin")

	report := verify.Report{Ready: true, FinalNote: "You're ready."}
	resolved := planner.ResolvedPlan{
		OrderedComponents: []model.ComponentID{model.ComponentDxrkMemory},
		PlatformDecision:  planner.PlatformDecision{PackageManager: "apt"},
	}

	updated := withGoInstallPathNote(report, resolved)
	if !strings.Contains(updated.FinalNote, "go install") {
		t.Fatalf("FinalNote should contain go install guidance, got: %q", updated.FinalNote)
	}
	if !strings.Contains(updated.FinalNote, "go/bin") {
		t.Fatalf("FinalNote should reference go/bin dir, got: %q", updated.FinalNote)
	}
}

func TestWithGoInstallPathNoteSkipsWhenBrew(t *testing.T) {
	report := verify.Report{Ready: true, FinalNote: "You're ready."}
	resolved := planner.ResolvedPlan{
		OrderedComponents: []model.ComponentID{model.ComponentDxrkMemory},
		PlatformDecision:  planner.PlatformDecision{PackageManager: "brew"},
	}

	updated := withGoInstallPathNote(report, resolved)
	if updated.FinalNote != report.FinalNote {
		t.Fatalf("FinalNote should be unchanged for brew, got: %q", updated.FinalNote)
	}
}

func TestWithGoInstallPathNoteSkipsWhenInPATH(t *testing.T) {
	t.Setenv("GOBIN", "")
	t.Setenv("GOPATH", "")
	home, _ := os.UserHomeDir()
	goBin := filepath.Join(home, "go", "bin")
	t.Setenv("PATH", "/usr/bin"+string(os.PathListSeparator)+goBin)

	report := verify.Report{Ready: true, FinalNote: "You're ready."}
	resolved := planner.ResolvedPlan{
		OrderedComponents: []model.ComponentID{model.ComponentDxrkMemory},
		PlatformDecision:  planner.PlatformDecision{PackageManager: "apt"},
	}

	updated := withGoInstallPathNote(report, resolved)
	if updated.FinalNote != report.FinalNote {
		t.Fatalf("FinalNote should be unchanged when go/bin is in PATH, got: %q", updated.FinalNote)
	}
}

func TestWithGoInstallPathNoteSkipsWithoutEngram(t *testing.T) {
	report := verify.Report{Ready: true, FinalNote: "You're ready."}
	resolved := planner.ResolvedPlan{
		OrderedComponents: []model.ComponentID{model.ComponentDxrkGuardian},
		PlatformDecision:  planner.PlatformDecision{PackageManager: "apt"},
	}

	updated := withGoInstallPathNote(report, resolved)
	if updated.FinalNote != report.FinalNote {
		t.Fatalf("FinalNote should be unchanged without dxrk-memory, got: %q", updated.FinalNote)
	}
}
