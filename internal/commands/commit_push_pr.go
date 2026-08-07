// SPDX-License-Identifier: MIT
package commands

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Dxrk777/Dxrk/internal/cli"
	"github.com/Dxrk777/Dxrk/internal/git"
)

func RegisterCommitPushPRCommand() {
	cmd := &cobra.Command{
		Use:   "cpr [title]",
		Short: "Commit, push, and create a PR in one step",
		Long: `Stage all changes, create a conventional commit, push to origin,
and open a pull request. If a PR already exists for the branch, updates it.

Examples:
  dxrk cpr "feat: add user auth"
  dxrk cpr --draft "WIP: new dashboard"
  dxrk cpr --base develop "fix: parser error"`,
		Args: cobra.MaximumNArgs(1),
		RunE: runCommitPushPR,
	}

	cmd.Flags().BoolP("all", "a", false, "Stage all changes before committing")
	cmd.Flags().String("base", "", "Base branch for the PR (default: auto-detect)")
	cmd.Flags().Bool("draft", false, "Create PR as draft")
	cmd.Flags().StringSlice("label", nil, "Labels to add to the PR")
	cmd.Flags().StringSlice("reviewer", nil, "Reviewers to add to the PR")

	cli.AddCommand(cmd)
}

func runCommitPushPR(cmd *cobra.Command, args []string) error {
	all, _ := cmd.Flags().GetBool("all")
	base, _ := cmd.Flags().GetString("base")
	draft, _ := cmd.Flags().GetBool("draft")
	labels, _ := cmd.Flags().GetStringSlice("label")
	reviewers, _ := cmd.Flags().GetStringSlice("reviewer")

	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	runner := git.NewRunner(wd)
	ctx := cmd.Context()

	// Check if in a git repo
	if _, err := runner.Root(ctx); err != nil {
		return fmt.Errorf("not a git repository: %w", err)
	}

	// Stage files if --all
	if all {
		if err := runner.Add(ctx, "-A"); err != nil {
			return fmt.Errorf("stage files: %w", err)
		}
	}

	// Check for staged changes
	status, err := runner.Status(ctx)
	if err != nil {
		return fmt.Errorf("get status: %w", err)
	}

	if len(status.Staged) == 0 && len(status.Untracked) == 0 {
		fmt.Fprintln(os.Stderr, "nothing to commit")
		return nil
	}

	// Get commit message
	var msg string
	if len(args) > 0 {
		msg = args[0]
	} else {
		msg, err = generateCommitMessage(ctx, runner)
		if err != nil {
			return fmt.Errorf("generate commit message: %w", err)
		}
	}

	// Get current branch
	branch, err := runner.CurrentBranch(ctx)
	if err != nil {
		return fmt.Errorf("get current branch: %w", err)
	}

	// Commit
	ci, err := runner.Commit(ctx, msg, nil)
	if err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	fmt.Printf("committed: %s (%s)\n", ci.ShortHash, ci.Message)

	// Detect default branch for base
	if base == "" {
		base = detectDefaultBranch(wd)
	}

	// Push
	pushResult, err := runner.Push(ctx, "origin", branch, false)
	if err != nil {
		return fmt.Errorf("push: %w", err)
	}
	fmt.Printf("pushed: %s\n", pushResult.Message)

	// Check if PR exists
	existingPR := findExistingPR(wd, branch)

	if existingPR != "" {
		// Update existing PR
		if err := updatePR(wd, existingPR, msg, labels); err != nil {
			return fmt.Errorf("update PR: %w", err)
		}
		fmt.Printf("updated PR: %s\n", existingPR)
	} else {
		// Create new PR
		prURL, err := createPR(wd, branch, base, msg, draft, labels, reviewers)
		if err != nil {
			return fmt.Errorf("create PR: %w", err)
		}
		fmt.Printf("created PR: %s\n", prURL)
	}

	return nil
}

func detectDefaultBranch(dir string) string {
	cmd := exec.CommandContext(context.Background(), "git", "symbolic-ref", "refs/remotes/origin/HEAD", "--short") //nolint:gosec
	cmd.Dir = dir
	out, err := cmd.Output()
	if err == nil {
		branch := strings.TrimSpace(string(out))
		branch = strings.TrimPrefix(branch, "origin/")
		if branch != "" {
			return branch
		}
	}
	return "main"
}

func findExistingPR(dir, branch string) string {
	cmd := exec.CommandContext(context.Background(), "gh", "pr", "list", "--head", branch, "--json", "number", "--jq", ".[0].number") //nolint:gosec
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	num := strings.TrimSpace(string(out))
	if num == "" || num == "null" {
		return ""
	}
	return num
}

func createPR(dir, head, base, title string, draft bool, labels, reviewers []string) (string, error) {
	args := []string{"pr", "create", "--title", title, "--body", fmt.Sprintf("## Summary\n\nAutomated PR via dxrk cpr.\n\nBranch: %s → %s", head, base)}
	if base != "" {
		args = append(args, "--base", base)
	}
	if draft {
		args = append(args, "--draft")
	}
	for _, l := range labels {
		args = append(args, "--label", l)
	}
	for _, r := range reviewers {
		args = append(args, "--reviewer", r)
	}

	cmd := exec.CommandContext(context.Background(), "gh", args...) //nolint:gosec
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("gh pr create: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func updatePR(dir, prNumber, title string, labels []string) error {
	args := make([]string, 0, 5+2*len(labels))
	args = append(args, "pr", "edit", prNumber, "--title", title)
	for _, l := range labels {
		args = append(args, "--add-label", l)
	}

	cmd := exec.CommandContext(context.Background(), "gh", args...) //nolint:gosec
	cmd.Dir = dir
	_, err := cmd.Output()
	return err
}
