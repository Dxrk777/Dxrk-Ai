// SPDX-License-Identifier: MIT
package git

import (
	"time"
)

// Repo represents a git repository.
type Repo struct {
	Path          string
	Worktree      string // For worktrees, path to linked worktree
	RemoteURL     string
	CurrentBranch string
	IsWorktree    bool
}

// CommitInfo holds commit metadata.
type CommitInfo struct {
	Hash      string
	ShortHash string
	Author    string
	Email     string
	Message   string
	Timestamp time.Time
	Files     []string
}

// DiffResult represents a git diff.
type DiffResult struct {
	Files []DiffFile
	Stats DiffStats
}

type DiffFile struct {
	Path      string
	OldPath   string // For renames
	Status    string // A, M, D, R, C, U
	Additions int
	Deletions int
	IsBinary  bool
	Content   string // Unified diff content
}

type DiffStats struct {
	FilesChanged int
	Additions    int
	Deletions    int
}

// StatusResult represents git status.
type StatusResult struct {
	Branch    string
	Ahead     int
	Behind    int
	Staged    []StatusEntry
	Unstaged  []StatusEntry
	Untracked []string
	Conflict  []string
}

type StatusEntry struct {
	Path  string
	Index string // Status in index
	Work  string // Status in worktree
}

// LogOptions configures git log.
type LogOptions struct {
	Limit       int
	Since       time.Time
	Until       time.Time
	Author      string
	Path        string
	AllBranches bool
	Oneline     bool
}

// BranchInfo represents a branch.
type BranchInfo struct {
	Name       string
	Hash       string
	IsCurrent  bool
	IsRemote   bool
	Upstream   string
	Ahead      int
	Behind     int
	LastCommit CommitInfo
}

// RemoteInfo represents a git remote.
type RemoteInfo struct {
	Name  string
	URL   string
	Fetch string
	Push  string
}

// TagInfo represents a git tag.
type TagInfo struct {
	Name        string
	Hash        string
	Message     string
	Tagger      string
	Timestamp   time.Time
	IsAnnotated bool
}

// StashEntry represents a stash.
type StashEntry struct {
	Index     int
	Hash      string
	Branch    string
	Message   string
	Timestamp time.Time
}

// WorktreeInfo represents a git worktree.
type WorktreeInfo struct {
	Path       string
	Branch     string
	Hash       string
	IsBare     bool
	IsDetached bool
	IsLocked   bool
}

// PRConfig holds PR creation configuration.
type PRConfig struct {
	Title      string
	Body       string
	BaseBranch string
	HeadBranch string
	Draft      bool
	Reviewers  []string
	Labels     []string
	Assignees  []string
}

// PRInfo represents a pull request.
type PRInfo struct {
	Number     int
	Title      string
	Body       string
	State      string // open, closed, merged
	BaseBranch string
	HeadBranch string
	URL        string
	Author     string
	CreatedAt  time.Time
	UpdatedAt  time.Time
	MergedAt   *time.Time
	Labels     []string
	Reviewers  []string
	Commits    int
	Changes    int
}

// MergeResult represents a merge operation result.
type MergeResult struct {
	Success    bool
	CommitHash string
	Conflicts  []string
	Message    string
}

// RebaseResult represents a rebase operation result.
type RebaseResult struct {
	Success    bool
	CommitHash string
	Conflicts  []string
	Steps      int
	Message    string
}

// PushResult represents a push operation result.
type PushResult struct {
	Success     bool
	RemoteRefs  []string
	Message     string
	ForcePushed bool
}

// FetchResult represents a fetch operation result.
type FetchResult struct {
	Success bool
	Refs    []string
	Pruned  int
	Message string
}

// PullResult represents a pull operation result.
type PullResult struct {
	Success     bool
	MergeCommit string
	FastForward bool
	Conflicts   []string
	Message     string
}

// ConfigOptions for git config.
type ConfigOptions struct {
	Global   bool
	Local    bool
	System   bool
	FilePath string
}
