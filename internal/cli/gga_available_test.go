// SPDX-License-Identifier: MIT
package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Dxrk777/Dxrk-Ai/internal/system"
)

// TestGGAAvailableDetectsViaLookPath verifies that dxrkGuardianAvailable returns true
// when dxrk-guardian is found on PATH via cmdLookPath.
func TestGGAAvailableDetectsViaLookPath(t *testing.T) {
	origLookPath := cmdLookPath
	cmdLookPath = func(file string) (string, error) {
		if file == "dxrk-guardian" {
			return "/usr/local/bin/dxrk-guardian", nil
		}
		return "", os.ErrNotExist
	}
	t.Cleanup(func() { cmdLookPath = origLookPath })

	if !dxrkGuardianAvailable(system.PlatformProfile{OS: "darwin", PackageManager: "brew"}) {
		t.Fatal("dxrkGuardianAvailable() = false, want true when dxrk-guardian is on PATH")
	}
}

// TestGGAAvailableDetectsViaLocalBin verifies that dxrkGuardianAvailable returns true
// when dxrk-guardian exists at ~/.local/bin/dxrk-guardian (default for install.sh on Linux/macOS).
func TestGGAAvailableDetectsViaLocalBin(t *testing.T) {
	tmpHome := t.TempDir()
	localBin := filepath.Join(tmpHome, ".local", "bin")
	if err := os.MkdirAll(localBin, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(localBin, "dxrk-guardian"), []byte("fake"), 0o600); err != nil {
		t.Fatal(err)
	}

	origLookPath := cmdLookPath
	origHomeDir := osUserHomeDir
	origStat := osStat
	cmdLookPath = func(file string) (string, error) { return "", os.ErrNotExist }
	osUserHomeDir = func() (string, error) { return tmpHome, nil }
	osStat = os.Stat
	t.Cleanup(func() {
		cmdLookPath = origLookPath
		osUserHomeDir = origHomeDir
		osStat = origStat
	})

	if !dxrkGuardianAvailable(system.PlatformProfile{OS: "linux", PackageManager: "apt"}) {
		t.Fatal("dxrkGuardianAvailable() = false, want true when dxrk-guardian is at ~/.local/bin/dxrk-guardian")
	}
}

// TestGGAAvailableDetectsViaHomebrewOptPrefix verifies that dxrkGuardianAvailable returns
// true when dxrk-guardian exists at /opt/homebrew/bin/dxrk-guardian (Apple Silicon Homebrew default).
func TestGGAAvailableDetectsViaHomebrewOptPrefix(t *testing.T) {
	tmpDir := t.TempDir()
	fakeOptHomebrew := filepath.Join(tmpDir, "opt", "homebrew", "bin", "dxrk-guardian")
	if err := os.MkdirAll(filepath.Dir(fakeOptHomebrew), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fakeOptHomebrew, []byte("fake"), 0o600); err != nil {
		t.Fatal(err)
	}

	origLookPath := cmdLookPath
	origHomeDir := osUserHomeDir
	origStat := osStat
	cmdLookPath = func(file string) (string, error) { return "", os.ErrNotExist }
	osUserHomeDir = func() (string, error) { return tmpDir, nil }
	// Override osStat to redirect well-known brew paths to our temp dir.
	osStat = func(name string) (os.FileInfo, error) {
		switch name {
		case "/opt/homebrew/bin/dxrk-guardian":
			return os.Stat(fakeOptHomebrew)
		case "/usr/local/bin/dxrk-guardian":
			return nil, os.ErrNotExist
		default:
			return os.Stat(name)
		}
	}
	t.Cleanup(func() {
		cmdLookPath = origLookPath
		osUserHomeDir = origHomeDir
		osStat = origStat
	})

	if !dxrkGuardianAvailable(system.PlatformProfile{OS: "darwin", PackageManager: "brew"}) {
		t.Fatal("dxrkGuardianAvailable() = false, want true when dxrk-guardian is at /opt/homebrew/bin/dxrk-guardian")
	}
}

// TestGGAAvailableDetectsViaHomebrewUsrLocalPrefix verifies that dxrkGuardianAvailable
// returns true when dxrk-guardian exists at /usr/local/bin/dxrk-guardian (Intel Mac Homebrew default).
func TestGGAAvailableDetectsViaHomebrewUsrLocalPrefix(t *testing.T) {
	origLookPath := cmdLookPath
	origHomeDir := osUserHomeDir
	origStat := osStat
	cmdLookPath = func(file string) (string, error) { return "", os.ErrNotExist }
	osUserHomeDir = func() (string, error) { return t.TempDir(), nil }
	osStat = func(name string) (os.FileInfo, error) {
		switch name {
		case "/opt/homebrew/bin/dxrk-guardian":
			return nil, os.ErrNotExist
		case "/usr/local/bin/dxrk-guardian":
			// Simulate dxrk-guardian present here.
			return os.Stat(os.DevNull)
		default:
			return nil, os.ErrNotExist
		}
	}
	t.Cleanup(func() {
		cmdLookPath = origLookPath
		osUserHomeDir = origHomeDir
		osStat = origStat
	})

	if !dxrkGuardianAvailable(system.PlatformProfile{OS: "darwin", PackageManager: "brew"}) {
		t.Fatal("dxrkGuardianAvailable() = false, want true when dxrk-guardian is at /usr/local/bin/dxrk-guardian")
	}
}

// TestGGAAvailableReturnsFalseWhenNotFound verifies that dxrkGuardianAvailable returns
// false when dxrk-guardian is not found via any detection path.
func TestGGAAvailableReturnsFalseWhenNotFound(t *testing.T) {
	origLookPath := cmdLookPath
	origHomeDir := osUserHomeDir
	origStat := osStat
	cmdLookPath = func(file string) (string, error) { return "", os.ErrNotExist }
	osUserHomeDir = func() (string, error) { return t.TempDir(), nil }
	osStat = func(name string) (os.FileInfo, error) { return nil, os.ErrNotExist }
	t.Cleanup(func() {
		cmdLookPath = origLookPath
		osUserHomeDir = origHomeDir
		osStat = origStat
	})

	if dxrkGuardianAvailable(system.PlatformProfile{OS: "darwin", PackageManager: "brew"}) {
		t.Fatal("dxrkGuardianAvailable() = true, want false when dxrk-guardian is not installed anywhere")
	}
}

// TestGGAAvailableBrewPathsSkippedOnLinux verifies that the Homebrew-specific
// paths (/opt/homebrew/bin/dxrk-guardian, /usr/local/bin/dxrk-guardian) are NOT checked on Linux
// even if those paths happen to exist (they never exist there in practice, but
// the guard ensures no cross-platform false positives).
func TestGGAAvailableBrewPathsSkippedOnLinux(t *testing.T) {
	origLookPath := cmdLookPath
	origHomeDir := osUserHomeDir
	origStat := osStat
	cmdLookPath = func(file string) (string, error) { return "", os.ErrNotExist }
	osUserHomeDir = func() (string, error) { return t.TempDir(), nil }

	statCallCount := 0
	osStat = func(name string) (os.FileInfo, error) {
		if name == "/opt/homebrew/bin/dxrk-guardian" || name == "/usr/local/bin/dxrk-guardian" {
			statCallCount++
		}
		return nil, os.ErrNotExist
	}
	t.Cleanup(func() {
		cmdLookPath = origLookPath
		osUserHomeDir = origHomeDir
		osStat = origStat
	})

	dxrkGuardianAvailable(system.PlatformProfile{OS: "linux", PackageManager: "apt"})
	if statCallCount > 0 {
		t.Fatalf("dxrkGuardianAvailable() checked Homebrew paths on Linux (%d calls), expected 0", statCallCount)
	}
}
