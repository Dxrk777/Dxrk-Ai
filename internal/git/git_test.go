// SPDX-License-Identifier: MIT
package git

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestParseStatus(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  func(*testing.T, *StatusResult)
	}{
		{
			name: "clean",
			input: `# branch.head main
# branch.ab +0 -0
`,
			want: func(t *testing.T, s *StatusResult) {
				if s.Branch != "main" {
					t.Errorf("branch = %q, want main", s.Branch)
				}
				if len(s.Staged) != 0 {
					t.Errorf("staged = %v, want empty", s.Staged)
				}
			},
		},
		{
			name: "modified file",
			input: `# branch.head feature
# branch.ab +3 -1
1 .M N... 100644 100644 00000000000 00000000000 file.go
`,
			want: func(t *testing.T, s *StatusResult) {
				if s.Branch != "feature" {
					t.Errorf("branch = %q", s.Branch)
				}
				if s.Ahead != 3 || s.Behind != 1 {
					t.Errorf("ahead/behind = %d/%d, want 3/1", s.Ahead, s.Behind)
				}
				if len(s.Unstaged) != 1 {
					t.Fatalf("unstaged = %v, want 1 entry", s.Unstaged)
				}
				if s.Unstaged[0].Path != "file.go" {
					t.Errorf("path = %q, want file.go", s.Unstaged[0].Path)
				}
			},
		},
		{
			name: "untracked",
			input: `# branch.head main
# branch.ab +0 -0
? new_file.go
`,
			want: func(t *testing.T, s *StatusResult) {
				if len(s.Untracked) != 1 || s.Untracked[0] != "new_file.go" {
					t.Errorf("untracked = %v, want [new_file.go]", s.Untracked)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseStatus(tt.input)
			tt.want(t, result)
		})
	}
}

func TestParseCommits(t *testing.T) {
	input := `abc123def456
abc1234
John Doe
john@example.com
1700000000
Initial commit
---
def789abc012
def7890
Jane Doe
jane@example.com
1700000001
Second commit
---
`
	commits := parseCommits(input)
	if len(commits) != 2 {
		t.Fatalf("got %d commits, want 2", len(commits))
	}

	if commits[0].Hash != "abc123def456" {
		t.Errorf("hash = %q, want abc123def456", commits[0].Hash)
	}
	if commits[0].ShortHash != "abc1234" {
		t.Errorf("short hash = %q, want abc1234", commits[0].ShortHash)
	}
	if commits[0].Author != "John Doe" {
		t.Errorf("author = %q", commits[0].Author)
	}
	if commits[0].Email != "john@example.com" {
		t.Errorf("email = %q", commits[0].Email)
	}
	if !commits[0].Timestamp.Equal(time.Unix(1700000000, 0)) {
		t.Errorf("timestamp = %v", commits[0].Timestamp)
	}
	if commits[0].Message != "Initial commit" {
		t.Errorf("message = %q", commits[0].Message)
	}

	if commits[1].Hash != "def789abc012" {
		t.Errorf("hash = %q", commits[1].Hash)
	}
}

func TestParseBranches(t *testing.T) {
	input := `* main                 abc1234 [origin/main] Latest changes
  feature-x            def5678 Some feature
  remotes/origin/main  abc1234 Latest changes
`
	branches := parseBranches(input)
	if len(branches) != 3 {
		t.Fatalf("got %d branches, want 3", len(branches))
	}

	if branches[0].Name != "main" || !branches[0].IsCurrent {
		t.Errorf("first branch = %+v, want main current", branches[0])
	}
	if branches[1].Name != "feature-x" || branches[1].IsCurrent {
		t.Errorf("second branch = %+v", branches[1])
	}
	if branches[2].Name != "remotes/origin/main" || !branches[2].IsRemote {
		t.Errorf("third branch = %+v", branches[2])
	}
}

func TestParseStashes(t *testing.T) {
	input := `abc123def456
stash@{0}
John Doe
1700000000
WIP on feature
---
`
	stashes := parseStashes(input)
	if len(stashes) != 1 {
		t.Fatalf("got %d stashes, want 1", len(stashes))
	}
	if stashes[0].Hash != "abc123def456" {
		t.Errorf("hash = %q", stashes[0].Hash)
	}
	if stashes[0].Message != "WIP on feature" {
		t.Errorf("message = %q", stashes[0].Message)
	}
}

func TestParseWorktrees(t *testing.T) {
	input := `worktree /home/user/project
HEAD abc1234
branch refs/heads/main

worktree /home/user/project-feature
HEAD def5678
branch refs/heads/feature
locked
`
	worktrees := parseWorktrees(input)
	if len(worktrees) != 2 {
		t.Fatalf("got %d worktrees, want 2", len(worktrees))
	}
	if worktrees[0].Path != "/home/user/project" {
		t.Errorf("path = %q", worktrees[0].Path)
	}
	if worktrees[0].Branch != "main" {
		t.Errorf("branch = %q", worktrees[0].Branch)
	}
	if worktrees[1].Branch != "feature" || !worktrees[1].IsLocked {
		t.Errorf("second worktree = %+v", worktrees[1])
	}
}

func TestParseHunkStats(t *testing.T) {
	tests := []struct {
		input       string
		wantAdded   int
		wantDeleted int
	}{
		{"+1,2 -3,4", 1, 3},
		{"+1 -3", 1, 3},
		{"+0,0 -0,0", 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			added, deleted := parseHunkStats(tt.input)
			if added != tt.wantAdded || deleted != tt.wantDeleted {
				t.Errorf("got %d,+%d, want %d,+%d", added, deleted, tt.wantAdded, tt.wantDeleted)
			}
		})
	}
}

func TestDetectConflicts(t *testing.T) {
	input := `Auto-merging file.go
CONFLICT (content): Merge conflict in file.go
CONFLICT (modify/delete): file2.go deleted in HEAD
`
	conflicts := detectConflicts(input)
	if len(conflicts) != 2 {
		t.Fatalf("got %d conflicts, want 2", len(conflicts))
	}
	if conflicts[0] != "Merge conflict in file.go" {
		t.Errorf("conflict[0] = %q", conflicts[0])
	}
}

func TestParseCount(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"1", 1},
		{"0", 0},
		{"", 0},
		{"5,2", 5},
		{"abc", 0},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseCount(tt.input)
			if got != tt.want {
				t.Errorf("parseCount(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsHex(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"abc123", true},
		{"ABC123", true},
		{"deadbeef", true},
		{"", true},
		{"xyz", false},
		{"abc123z", false},
		{"123", true},
		{"abc.123", true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := isHex(tt.input)
			if got != tt.want {
				t.Errorf("isHex(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func createTestRepo(t *testing.T, cmds ...string) *Runner {
	t.Helper()
	dir := t.TempDir()

	mustRun(t, dir, "git", "init", "--initial-branch=master")
	mustRun(t, dir, "git", "config", "user.name", "Test User")
	mustRun(t, dir, "git", "config", "user.email", "test@example.com")

	for _, cmd := range cmds {
		parts := strings.Fields(cmd)
		mustRun(t, dir, parts[0], parts[1:]...)
	}

	return NewRunner(dir)
}

func mustRun(t *testing.T, dir, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, out)
	}
	return string(out)
}

func TestRunnerCurrentBranch(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping git-dependent test in short mode")
	}
	r := createTestRepo(t, "git commit --allow-empty -m init")
	branch, err := r.CurrentBranch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if branch != "master" {
		t.Errorf("CurrentBranch = %q, want master", branch)
	}

	mustRun(t, r.workDir, "git", "checkout", "-b", "test-branch")
	branch, err = r.CurrentBranch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if branch != "test-branch" {
		t.Errorf("CurrentBranch = %q, want test-branch", branch)
	}
}

func TestRunnerRoot(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping git-dependent test in short mode")
	}
	r := createTestRepo(t)
	root, err := r.Root(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(root, r.workDir) {
		t.Errorf("Root = %q, want suffix %q", root, r.workDir)
	}
}

func TestRunnerStatus(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping git-dependent test in short mode")
	}
	r := createTestRepo(t, "git commit --allow-empty -m init")

	// Clean
	s, err := r.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if s.Branch != "master" {
		t.Errorf("branch = %q, want master", s.Branch)
	}
	if len(s.Unstaged) != 0 || len(s.Staged) != 0 || len(s.Untracked) != 0 {
		t.Errorf("expected clean status, got unstaged=%d staged=%d untracked=%d", len(s.Unstaged), len(s.Staged), len(s.Untracked))
	}

	// Modified
	mustRun(t, r.workDir, "sh", "-c", "echo change > file.txt")
	s, err = r.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Untracked) == 0 {
		t.Fatal("expected untracked file")
	}
	if s.Untracked[0] != "file.txt" {
		t.Errorf("untracked = %q, want file.txt", s.Untracked[0])
	}

	// Staged
	mustRun(t, r.workDir, "git", "add", "file.txt")
	s, err = r.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Staged) == 0 {
		t.Fatal("expected staged file")
	}
}

func TestRunnerLog(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping git-dependent test in short mode")
	}
	r := createTestRepo(t,
		"git commit --allow-empty -m first",
		"git commit --allow-empty -m second",
		"git commit --allow-empty -m third",
	)

	commits, err := r.Log(context.Background(), LogOptions{Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) != 2 {
		t.Fatalf("got %d commits, want 2", len(commits))
	}
	if commits[0].Message != "third" {
		t.Errorf("first commit message = %q, want third", commits[0].Message)
	}
	if commits[1].Message != "second" {
		t.Errorf("second commit message = %q, want second", commits[1].Message)
	}
	if commits[0].Hash == "" || commits[0].ShortHash == "" {
		t.Error("commit hash/short hash should not be empty")
	}
	if commits[0].Author != "Test User" {
		t.Errorf("author = %q, want Test User", commits[0].Author)
	}
}

func TestRunnerAddCommit(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping git-dependent test in short mode")
	}
	r := createTestRepo(t, "git commit --allow-empty -m init")

	mustRun(t, r.workDir, "sh", "-c", "echo content > test.txt")

	if err := r.Add(context.Background(), "test.txt"); err != nil {
		t.Fatal(err)
	}

	author := &AuthorInfo{Name: "Test User", Email: "test@example.com"}
	ci, err := r.Commit(context.Background(), "add test.txt", author)
	if err != nil {
		t.Fatal(err)
	}
	if ci.Hash == "" {
		t.Error("commit hash should not be empty")
	}
	if ci.ShortHash == "" {
		t.Error("short hash should not be empty")
	}
	if ci.Message != "add test.txt" {
		t.Errorf("message = %q, want add test.txt", ci.Message)
	}

	// Verify with log
	commits, err := r.Log(context.Background(), LogOptions{Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) != 1 {
		t.Fatalf("got %d commits, want 1", len(commits))
	}
	if commits[0].Message != "add test.txt" {
		t.Errorf("message = %q, want add test.txt", commits[0].Message)
	}
}

func TestRunnerBranch(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping git-dependent test in short mode")
	}
	r := createTestRepo(t, "git commit --allow-empty -m init")

	mustRun(t, r.workDir, "git", "checkout", "-b", "feature-a")
	mustRun(t, r.workDir, "git", "commit", "--allow-empty", "-m", "feature a work")
	mustRun(t, r.workDir, "git", "checkout", "-b", "feature-b")

	branches, err := r.Branch(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if len(branches) < 2 {
		t.Fatalf("got %d branches, want >= 2", len(branches))
	}

	gotCurrent := 0
	gotFeature := false
	for _, b := range branches {
		if b.IsCurrent {
			gotCurrent++
			if b.Name != "feature-b" {
				t.Errorf("current branch = %q, want feature-b", b.Name)
			}
		}
		if b.Name == "feature-a" {
			gotFeature = true
		}
	}
	if gotCurrent != 1 {
		t.Errorf("got %d current branches, want 1", gotCurrent)
	}
	if !gotFeature {
		t.Error("feature-a branch not found")
	}
}

func TestRunnerDiff(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping git-dependent test in short mode")
	}
	r := createTestRepo(t, "git commit --allow-empty -m init")

	// Create a tracked file first, then modify it
	mustRun(t, r.workDir, "sh", "-c", "echo line1 > diff.txt")
	mustRun(t, r.workDir, "git", "add", "diff.txt")
	mustRun(t, r.workDir, "git", "commit", "-m", "add diff.txt")
	mustRun(t, r.workDir, "sh", "-c", "echo line2 >> diff.txt")

	// Unstaged diff
	diff, err := r.Diff(context.Background(), false, "")
	if err != nil {
		t.Fatal(err)
	}
	if diff.Stats.FilesChanged == 0 {
		t.Fatal("expected diff to show changes")
	}

	// Staged diff (should be empty since nothing is staged)
	diffStaged, err := r.Diff(context.Background(), true, "")
	if err != nil {
		t.Fatal(err)
	}
	if diffStaged.Stats.FilesChanged != 0 {
		t.Errorf("expected empty staged diff, got %d files", diffStaged.Stats.FilesChanged)
	}
}

func TestRunnerStash(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping git-dependent test in short mode")
	}
	r := createTestRepo(t, "git commit --allow-empty -m init")

	mustRun(t, r.workDir, "sh", "-c", "echo stash-content > stash.txt")
	mustRun(t, r.workDir, "git", "add", "stash.txt")

	// Stash
	if err := r.Stash(context.Background(), "test stash"); err != nil {
		t.Fatal(err)
	}

	// StashList
	stashes, err := r.StashList(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(stashes) != 1 {
		t.Fatalf("got %d stashes, want 1", len(stashes))
	}
	if stashes[0].Message != "On master: test stash" {
		t.Errorf("stash message = %q, want On master: test stash", stashes[0].Message)
	}
	if stashes[0].Index != 0 {
		t.Errorf("stash index = %d, want 0", stashes[0].Index)
	}
	if stashes[0].Hash == "" {
		t.Error("stash hash should not be empty")
	}

	// StashPop
	if err := r.StashPop(context.Background(), 0); err != nil {
		t.Fatal(err)
	}

	stashes, err = r.StashList(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(stashes) != 0 {
		t.Errorf("expected empty stash list after pop, got %d", len(stashes))
	}
}

func TestExtractHash(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"[main abc1234] Initial commit", "abc1234"},
		{"[main abc1234.] Initial commit", "abc1234"},
		{"[feature/x  def5678] Feature", "def5678"},
		{"nothing to commit", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			hash := extractHash(tt.input)
			if hash != tt.want {
				t.Errorf("got %q, want %q", hash, tt.want)
			}
		})
	}
}
