// SPDX-License-Identifier: MIT
package git

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/Dxrk777/Dxrk-Ai/internal/strconst"
)

const (
	gitBranchCmd = "branch"
	gitStashCmd  = "stash"
)

type Runner struct {
	workDir string
}

func NewRunner(workDir string) *Runner {
	return &Runner{workDir: workDir}
}

func (r *Runner) run(ctx context.Context, args ...string) (string, string, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "git", args...) //nolint:gosec
	cmd.Dir = r.workDir
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return strings.TrimSpace(stdout.String()), strings.TrimSpace(stderr.String()), err
}

func (r *Runner) runWithInput(ctx context.Context, input string, args ...string) (string, string, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "git", args...) //nolint:gosec
	cmd.Dir = r.workDir
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Stdin = strings.NewReader(input)
	err := cmd.Run()
	return strings.TrimSpace(stdout.String()), strings.TrimSpace(stderr.String()), err
}

func (r *Runner) Status(ctx context.Context) (*StatusResult, error) {
	stdout, _, err := r.run(ctx, strconst.StrStatus, "--porcelain=v2", "--branch")
	if err != nil {
		return nil, fmt.Errorf("git status: %w", err)
	}
	return parseStatus(stdout), nil
}

func parseStatus(s string) *StatusResult {
	res := &StatusResult{}
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "# branch.head ") {
			res.Branch = strings.TrimPrefix(line, "# branch.head ")
			continue
		}
		if strings.HasPrefix(line, "# branch.ab +") {
			rest := strings.TrimPrefix(line, "# branch.ab +")
			parts := strings.SplitN(rest, " -", 2)
			if len(parts) == 2 {
				res.Ahead, _ = strconv.Atoi(parts[0])
				res.Behind, _ = strconv.Atoi(parts[1])
			}
			continue
		}
		if strings.HasPrefix(line, "1 ") || strings.HasPrefix(line, "2 ") {
			fields := strings.Fields(line)
			if len(fields) >= 3 {
				index := fields[1]
				work := fields[2]
				path := fields[len(fields)-1]
				entry := StatusEntry{Path: path, Index: index, Work: work}
				if index != "." || work != "." {
					res.Unstaged = append(res.Unstaged, entry)
				}
				if index != "." && index != " " {
					res.Staged = append(res.Staged, entry)
				}
			}
			continue
		}
		if strings.HasPrefix(line, "? ") {
			res.Untracked = append(res.Untracked, strings.TrimSpace(line[2:]))
			continue
		}
		if strings.HasPrefix(line, "u ") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				res.Conflict = append(res.Conflict, fields[len(fields)-1])
			}
		}
	}
	return res
}

func (r *Runner) Diff(ctx context.Context, staged bool, path string) (*DiffResult, error) {
	args := []string{"diff"}
	if staged {
		args = append(args, "--staged")
	}
	if path != "" {
		args = append(args, "--", path)
	}

	stdout, _, err := r.run(ctx, args...)
	if err != nil {
		return nil, fmt.Errorf("git diff: %w", err)
	}

	res := &DiffResult{}
	if stdout == "" {
		return res, nil
	}

	lines := strings.Split(stdout, "\n")
	var cur *DiffFile
	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git ") {
			if cur != nil {
				res.Files = append(res.Files, *cur)
			}
			cur = &DiffFile{}
			parts := strings.Split(line, " ")
			if len(parts) >= 4 {
				cur.Path = strings.TrimPrefix(parts[len(parts)-1], "b/")
			}
			continue
		}
		if strings.HasPrefix(line, "--- ") {
			continue
		}
		if strings.HasPrefix(line, "+++ ") {
			continue
		}
		if strings.HasPrefix(line, "@@") {
			if cur != nil {
				parts := strings.Split(line, " ")
				if len(parts) >= 3 {
					added, deleted := parseHunkStats(parts[2])
					cur.Additions += added
					cur.Deletions += deleted
				}
			}
		}
		if cur != nil {
			cur.Content += line + "\n"
			if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
				cur.Additions++
			} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
				cur.Deletions++
			}
		}
	}
	if cur != nil {
		res.Files = append(res.Files, *cur)
	}
	for _, f := range res.Files {
		res.Stats.FilesChanged++
		res.Stats.Additions += f.Additions
		res.Stats.Deletions += f.Deletions
	}
	return res, nil
}

func parseHunkStats(s string) (int, int) {
	// formats: "+1,2 -3,4" or "+1 -3"
	parts := strings.Split(s, " ")
	if len(parts) < 2 {
		return 0, 0
	}
	added := parseCount(strings.TrimPrefix(parts[0], "+"))
	deleted := parseCount(strings.TrimPrefix(parts[1], "-"))
	return added, deleted
}

func parseCount(s string) int {
	if idx := strings.IndexByte(s, ','); idx >= 0 {
		s = s[:idx]
	}
	n, _ := strconv.Atoi(s)
	return n
}

func (r *Runner) Log(ctx context.Context, opts LogOptions) ([]CommitInfo, error) {
	args := []string{"log", "--format=%H%n%h%n%an%n%ae%n%ct%n%s%n---"}
	if opts.Limit > 0 {
		args = append(args, fmt.Sprintf("-%d", opts.Limit))
	}
	if !opts.Since.IsZero() {
		args = append(args, "--since="+opts.Since.Format(time.RFC3339))
	}
	if opts.Author != "" {
		args = append(args, "--author="+opts.Author)
	}
	if opts.AllBranches {
		args = append(args, "--all")
	}
	if opts.Path != "" {
		args = append(args, "--", opts.Path)
	}

	stdout, _, err := r.run(ctx, args...)
	if err != nil {
		return nil, fmt.Errorf("git log: %w", err)
	}

	return parseCommits(stdout), nil
}

func parseCommits(s string) []CommitInfo {
	var commits []CommitInfo
	for _, block := range strings.Split(s, "---\n") {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}
		lines := strings.SplitN(block, "\n", 7)
		if len(lines) < 6 {
			continue
		}
		ts, _ := strconv.ParseInt(lines[4], 10, 64)
		commits = append(commits, CommitInfo{
			Hash:      lines[0],
			ShortHash: lines[1],
			Author:    lines[2],
			Email:     lines[3],
			Timestamp: time.Unix(ts, 0),
			Message:   lines[5],
		})
	}
	return commits
}

func (r *Runner) Add(ctx context.Context, files ...string) error {
	args := append([]string{"add"}, files...)
	_, stderr, err := r.run(ctx, args...)
	if err != nil {
		return fmt.Errorf("git add: %s: %w", stderr, err)
	}
	return nil
}

func (r *Runner) Commit(ctx context.Context, msg string, author *AuthorInfo) (*CommitInfo, error) {
	args := []string{"commit", "-m", msg}
	if author != nil {
		if author.Name != "" {
			args = append(args, "--author="+author.Name+" <"+author.Email+">")
		}
	}
	args = append(args, "--allow-empty")

	stdout, stderr, err := r.run(ctx, args...)
	if err != nil {
		return nil, fmt.Errorf("git commit: %s: %w", stderr, err)
	}

	hash := extractHash(stdout)
	info := &CommitInfo{Hash: hash, Message: msg}
	if len(hash) >= 7 {
		info.ShortHash = hash[:7]
	}
	info.Timestamp = time.Now()

	return info, nil
}

type AuthorInfo struct {
	Name  string
	Email string
}

func extractHash(s string) string {
	for _, line := range strings.Split(s, "\n") {
		if strings.HasPrefix(line, "[") {
			closeIdx := strings.IndexByte(line, ']')
			if closeIdx < 0 {
				continue
			}
			inner := line[1:closeIdx]
			parts := strings.Fields(inner)
			for _, p := range parts {
				if len(p) >= 7 && isHex(p) {
					return strings.TrimRight(p, ".")
				}
			}
		}
	}
	return ""
}

func isHex(s string) bool {
	for _, c := range s {
		if c == '.' {
			continue
		}
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return false
		}
	}
	return true
}

func (r *Runner) Branch(ctx context.Context, all bool) ([]BranchInfo, error) {
	args := []string{gitBranchCmd, "-vv"}
	if all {
		args = append(args, "-a")
	}

	stdout, _, err := r.run(ctx, args...)
	if err != nil {
		return nil, fmt.Errorf("git branch: %w", err)
	}

	return parseBranches(stdout), nil
}

func parseBranches(s string) []BranchInfo {
	var branches []BranchInfo
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		bi := BranchInfo{
			IsCurrent: strings.HasPrefix(line, "* "),
		}
		rest := strings.TrimLeft(line, "* ")
		parts := strings.Fields(rest)
		if len(parts) > 0 {
			bi.Name = parts[0]
		}
		if len(parts) > 1 {
			bi.Hash = parts[1]
		}
		bi.IsRemote = strings.HasPrefix(bi.Name, "remotes/")
		branches = append(branches, bi)
	}
	return branches
}

func (r *Runner) Checkout(ctx context.Context, branch string, create bool) error {
	args := []string{"checkout"}
	if create {
		args = append(args, "-b")
	}
	args = append(args, branch)
	_, stderr, err := r.run(ctx, args...)
	if err != nil {
		return fmt.Errorf("git checkout: %s: %w", stderr, err)
	}
	return nil
}

func (r *Runner) Push(ctx context.Context, remote, branch string, force bool) (*PushResult, error) {
	args := []string{"push"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, remote, branch)

	stdout, stderr, err := r.run(ctx, args...)
	if err != nil {
		return &PushResult{Success: false, Message: stderr}, fmt.Errorf("git push: %s: %w", stderr, err)
	}

	return &PushResult{
		Success: true,
		Message: stdout + "\n" + stderr,
	}, nil
}

func (r *Runner) Pull(ctx context.Context, remote, branch string, rebase bool) (*PullResult, error) {
	args := []string{"pull"}
	if rebase {
		args = append(args, "--rebase")
	}
	if remote != "" {
		args = append(args, remote)
	}
	if branch != "" {
		args = append(args, branch)
	}

	stdout, stderr, err := r.run(ctx, args...)
	if err != nil {
		return &PullResult{Success: false, Message: stderr}, fmt.Errorf("git pull: %s: %w", stderr, err)
	}

	return &PullResult{
		Success: true,
		Message: stdout + "\n" + stderr,
	}, nil
}

func (r *Runner) Stash(ctx context.Context, msg string) error {
	args := []string{gitStashCmd, "push"}
	if msg != "" {
		args = append(args, "-m", msg)
	}
	_, stderr, err := r.run(ctx, args...)
	if err != nil {
		return fmt.Errorf("git stash: %s: %w", stderr, err)
	}
	return nil
}

func (r *Runner) StashPop(ctx context.Context, index int) error {
	args := []string{"stash", "pop"}
	if index >= 0 {
		args = append(args, fmt.Sprintf("stash@{%d}", index))
	}
	_, stderr, err := r.run(ctx, args...)
	if err != nil {
		return fmt.Errorf("git stash pop: %s: %w", stderr, err)
	}
	return nil
}

func (r *Runner) StashList(ctx context.Context) ([]StashEntry, error) {
	args := []string{"stash", "list", "--format=%H%n%gd%n%an%n%ct%n%s%n---"}
	stdout, _, err := r.run(ctx, args...)
	if err != nil {
		return nil, fmt.Errorf("git stash list: %w", err)
	}

	return parseStashes(stdout), nil
}

func parseStashes(s string) []StashEntry {
	var stashes []StashEntry
	for _, block := range strings.Split(s, "---\n") {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}
		lines := strings.SplitN(block, "\n", 6)
		if len(lines) < 5 {
			continue
		}
		ts, _ := strconv.ParseInt(lines[3], 10, 64)
		var idx int
		_, _ = fmt.Sscanf(lines[1], "stash@{%d}", &idx)
		stashes = append(stashes, StashEntry{
			Index:     idx,
			Hash:      lines[0],
			Message:   lines[4],
			Timestamp: time.Unix(ts, 0),
		})
	}
	return stashes
}

func (r *Runner) Fetch(ctx context.Context, remote string, prune bool) (*FetchResult, error) {
	args := []string{"fetch"}
	if prune {
		args = append(args, "--prune")
	}
	if remote != "" {
		args = append(args, remote)
	}

	stdout, stderr, err := r.run(ctx, args...)
	if err != nil {
		return &FetchResult{Success: false, Message: stderr}, fmt.Errorf("git fetch: %s: %w", stderr, err)
	}

	pruned := 0
	for _, line := range strings.Split(stdout, "\n") {
		if strings.Contains(line, "[pruned]") {
			pruned++
		}
	}

	return &FetchResult{
		Success: true,
		Pruned:  pruned,
		Message: stdout + "\n" + stderr,
	}, nil
}

func (r *Runner) Merge(ctx context.Context, branch string, ffOnly bool) (*MergeResult, error) {
	args := []string{"merge"}
	if ffOnly {
		args = append(args, "--ff-only")
	}
	args = append(args, branch)

	stdout, stderr, err := r.run(ctx, args...)
	if err != nil {
		return &MergeResult{
			Success:   false,
			Conflicts: detectConflicts(stdout + "\n" + stderr),
			Message:   stderr,
		}, fmt.Errorf("git merge: %s: %w", stderr, err)
	}

	return &MergeResult{
		Success: true,
		Message: stdout + "\n" + stderr,
	}, nil
}

func (r *Runner) Remote(ctx context.Context) ([]RemoteInfo, error) {
	stdout, _, err := r.run(ctx, "remote", "-v")
	if err != nil {
		return nil, fmt.Errorf("git remote: %w", err)
	}

	seen := map[string]*RemoteInfo{}
	for _, line := range strings.Split(stdout, "\n") {
		parts := strings.Fields(line)
		if len(parts) < 3 {
			continue
		}
		name, url, ref := parts[0], parts[1], parts[2]
		if _, ok := seen[name]; !ok {
			seen[name] = &RemoteInfo{Name: name, URL: url}
		}
		ri := seen[name]
		switch ref {
		case "(fetch)":
			ri.Fetch = url
		case "(push)":
			ri.Push = url
		}
	}

	var result []RemoteInfo
	for _, ri := range seen {
		result = append(result, *ri)
	}
	return result, nil
}

func (r *Runner) WorktreeList(ctx context.Context) ([]WorktreeInfo, error) {
	stdout, _, err := r.run(ctx, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("git worktree list: %w", err)
	}

	return parseWorktrees(stdout), nil
}

func parseWorktrees(s string) []WorktreeInfo {
	var worktrees []WorktreeInfo
	var cur *WorktreeInfo
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			if cur != nil {
				worktrees = append(worktrees, *cur)
				cur = nil
			}
			continue
		}
		if strings.HasPrefix(line, "worktree ") {
			if cur != nil {
				worktrees = append(worktrees, *cur)
			}
			cur = &WorktreeInfo{Path: strings.TrimPrefix(line, "worktree ")}
			continue
		}
		if cur == nil {
			continue
		}
		if strings.HasPrefix(line, "HEAD ") {
			cur.Hash = strings.TrimPrefix(line, "HEAD ")
			continue
		}
		if strings.HasPrefix(line, "branch ") {
			cur.Branch = strings.TrimPrefix(line, "branch refs/heads/")
			continue
		}
		if line == "bare" {
			cur.IsBare = true
			continue
		}
		if line == "detached" {
			cur.IsDetached = true
			continue
		}
		if strings.HasPrefix(line, "locked") {
			cur.IsLocked = true
			continue
		}
	}
	if cur != nil {
		worktrees = append(worktrees, *cur)
	}
	return worktrees
}

func (r *Runner) WorktreeAdd(ctx context.Context, path, branch string) error {
	args := []string{"worktree", "add", path}
	if branch != "" {
		args = append(args, branch)
	}
	_, stderr, err := r.run(ctx, args...)
	if err != nil {
		return fmt.Errorf("git worktree add: %s: %w", stderr, err)
	}
	return nil
}

func (r *Runner) WorktreeRemove(ctx context.Context, path string, force bool) error {
	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, path)
	_, stderr, err := r.run(ctx, args...)
	if err != nil {
		return fmt.Errorf("git worktree remove: %s: %w", stderr, err)
	}
	return nil
}

func (r *Runner) CreatePR(ctx context.Context, cfg PRConfig) (*PRInfo, error) {
	args := []string{"pr", "create", "--title", cfg.Title, "--body", cfg.Body}
	if cfg.BaseBranch != "" {
		args = append(args, "--base", cfg.BaseBranch)
	}
	if cfg.HeadBranch != "" {
		args = append(args, "--head", cfg.HeadBranch)
	}
	if cfg.Draft {
		args = append(args, "--draft")
	}
	for _, label := range cfg.Labels {
		args = append(args, "--label", label)
	}
	for _, reviewer := range cfg.Reviewers {
		args = append(args, "--reviewer", reviewer)
	}
	for _, assignee := range cfg.Assignees {
		args = append(args, "--assignee", assignee)
	}

	stdout, stderr, err := r.runWithInput(ctx, cfg.Body, args...)
	if err != nil {
		return nil, fmt.Errorf("gh pr create: %s: %w", stderr, err)
	}

	return &PRInfo{
		URL:  strings.TrimSpace(stdout),
		Body: cfg.Body,
	}, nil
}

func detectConflicts(s string) []string {
	var conflicts []string
	for _, line := range strings.Split(s, "\n") {
		if strings.HasPrefix(line, "CONFLICT") {
			if idx := strings.Index(line, ": "); idx >= 0 {
				desc := strings.TrimRight(line[idx+2:], ".")
				conflicts = append(conflicts, desc)
			} else {
				conflicts = append(conflicts, strings.TrimRight(line, "."))
			}
		}
	}
	return conflicts
}

func (r *Runner) Root(ctx context.Context) (string, error) {
	stdout, _, err := r.run(ctx, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("git rev-parse: %w", err)
	}
	return stdout, nil
}

func (r *Runner) CurrentBranch(ctx context.Context) (string, error) {
	stdout, _, err := r.run(ctx, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", fmt.Errorf("git rev-parse: %w", err)
	}
	return stdout, nil
}
