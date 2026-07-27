// SPDX-License-Identifier: MIT
package autonomy

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

type UpdateResult struct {
	Updated bool
	Before  string
	After   string
	Changes int
	Error   string
}

type Updater struct {
	projectRoot string
	mu          sync.Mutex
	lastCheck   time.Time
	interval    time.Duration
	perms       *PermissionStore
}

func NewUpdater(root string, intervalSec int, perms *PermissionStore) *Updater {
	interval := time.Duration(intervalSec) * time.Second
	if interval < 30*time.Second {
		interval = 30 * time.Second
	}
	return &Updater{
		projectRoot: root,
		interval:    interval,
		lastCheck:   time.Now().Add(-interval),
		perms:       perms,
	}
}

func (u *Updater) Check(force bool) *UpdateResult {
	u.mu.Lock()
	defer u.mu.Unlock()

	if !force && time.Since(u.lastCheck) < u.interval {
		return &UpdateResult{Updated: false}
	}
	u.lastCheck = time.Now()

	if err := u.perms.Check(CapGit, "self-update: git fetch + pull"); err != nil {
		return &UpdateResult{Updated: false, Error: err.Error()}
	}

	before := u.currentCommit()
	fetch := u.git("fetch", "--all")
	if fetch.err != nil {
		return &UpdateResult{Updated: false, Error: fmt.Sprintf("fetch: %s", fetch.out)}
	}

	logOut := u.git("log", "HEAD..origin/HEAD", "--oneline")
	if logOut.err != nil {
		return &UpdateResult{Updated: false, Error: fmt.Sprintf("log: %s", logOut.out)}
	}

	changes := bytes.Count(logOut.out, []byte{'\n'})
	if changes == 0 && !force {
		return &UpdateResult{Updated: false}
	}

	pull := u.git("pull", "--rebase")
	if pull.err != nil {
		return &UpdateResult{Updated: false, Error: fmt.Sprintf("pull: %s", pull.out)}
	}

	after := u.currentCommit()
	rebuild := u.runBuild()
	if rebuild.err != nil {
		return &UpdateResult{
			Updated: false,
			Before:  before,
			After:   after,
			Changes: changes,
			Error:   fmt.Sprintf("build failed: %s", rebuild.out),
		}
	}

	return &UpdateResult{
		Updated: true,
		Before:  before,
		After:   after,
		Changes: changes,
	}
}

func (u *Updater) currentCommit() string {
	out := u.git("rev-parse", "--short", "HEAD")
	if out.err != nil {
		return "unknown"
	}
	return string(bytes.TrimSpace(out.out))
}

func (u *Updater) git(args ...string) cmdResult {
	cmd := exec.Command("git", args...) //nolint:gosec
	cmd.Dir = u.projectRoot
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	out := stdout.Bytes()
	if stderr.Len() > 0 {
		out = append(out, '\n')
		out = append(out, stderr.Bytes()...)
	}
	return cmdResult{out: out, err: err}
}

func (u *Updater) runBuild() cmdResult {
	cmd := exec.Command("go", "build", "./...")
	cmd.Dir = u.projectRoot
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	out := stdout.Bytes()
	if stderr.Len() > 0 {
		out = append(out, '\n')
		out = append(out, stderr.Bytes()...)
	}
	return cmdResult{out: out, err: err}
}

func (u *Updater) LastCheck() time.Time {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.lastCheck
}

func (u *Updater) WriteFile(path string, content []byte) error {
	if err := u.perms.Check(CapFSWrite, fmt.Sprintf("write %s", path)); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	return os.WriteFile(path, content, 0o600)
}
