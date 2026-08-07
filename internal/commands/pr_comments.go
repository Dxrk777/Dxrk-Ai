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
)

func RegisterPRCommentsCommand() {
	cmd := &cobra.Command{
		Use:   "pr-comments [pr-number]",
		Short: "View and respond to PR comments",
		Long: `Fetch and display comments from a GitHub pull request.

Without arguments: shows comments for the current branch's PR.
With a PR number: shows comments for that specific PR.

Examples:
  dxrk pr-comments
  dxrk pr-comments 42
  dxrk pr-comments --resolve 42`,
		Args: cobra.MaximumNArgs(1),
		RunE: runPRComments,
	}

	cmd.Flags().Bool("resolve", false, "Mark comment thread as resolved")

	cli.AddCommand(cmd)
}

func runPRComments(cmd *cobra.Command, args []string) error {
	resolve, _ := cmd.Flags().GetBool("resolve")

	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	// Get PR number
	var prNum string
	if len(args) > 0 {
		prNum = args[0]
	} else {
		prNum, err = findCurrentBranchPR(wd)
		if err != nil {
			return fmt.Errorf("find PR for current branch: %w", err)
		}
	}

	if resolve {
		return resolvePRComments(wd, prNum)
	}

	return fetchAndDisplayPRComments(wd, prNum)
}

func findCurrentBranchPR(dir string) (string, error) {
	cmd := exec.CommandContext(context.Background(), "gh", "pr", "list", "--json", "number,headRefName", "--jq", ".[0].number") //nolint:gosec
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("no PR found for current branch: %w", err)
	}
	num := strings.TrimSpace(string(out))
	if num == "" || num == "null" {
		return "", fmt.Errorf("no PR found for current branch")
	}
	return num, nil
}

func fetchAndDisplayPRComments(dir, prNum string) error {
	// Get PR-level comments (issue comments)
	cmd := exec.CommandContext(context.Background(), "gh", "api", fmt.Sprintf("/repos/{owner}/{repo}/issues/%s/comments", prNum), "--jq", ".[].body") //nolint:gosec
	cmd.Dir = dir
	cmdOut, err := cmd.Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not fetch PR comments: %v\n", err)
	}

	// Get review comments (code review comments)
	cmd = exec.CommandContext(context.Background(), "gh", "api", fmt.Sprintf("/repos/{owner}/{repo}/pulls/%s/comments", prNum), "--jq", ".[] | \"@\\(.user.login) \\(.path)#\\(.line):\\n\\(.body)\\n---\"") //nolint:gosec
	cmd.Dir = dir
	reviewOut, err := cmd.Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not fetch review comments: %v\n", err)
	}

	fmt.Printf("PR #%s Comments:\n\n", prNum)

	if len(cmdOut) > 0 {
		fmt.Println("## General Comments")
		fmt.Println(strings.TrimSpace(string(cmdOut)))
		fmt.Println()
	}

	if len(reviewOut) > 0 {
		fmt.Println("## Code Review Comments")
		fmt.Println(strings.TrimSpace(string(reviewOut)))
		fmt.Println()
	}

	if len(cmdOut) == 0 && len(reviewOut) == 0 {
		fmt.Println("No comments found.")
	}

	return nil
}

func resolvePRComments(dir, prNum string) error {
	// Get unresolved comment threads
	cmd := exec.CommandContext(context.Background(), "gh", "api", fmt.Sprintf("/repos/{owner}/{repo}/pulls/%s/comments", prNum), "--jq", ".[].id") //nolint:gosec
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("fetch comments: %w", err)
	}

	ids := strings.Fields(strings.TrimSpace(string(out)))
	if len(ids) == 0 {
		fmt.Println("No comment threads to resolve.")
		return nil
	}

	for _, id := range ids {
		resolveCmd := exec.CommandContext(context.Background(), "gh", "api", fmt.Sprintf("/repos/{owner}/{repo}/pulls/comments/%s/threads/%s", id, id), "--method", "PATCH", "-f", "resolved=true") //nolint:gosec
		resolveCmd.Dir = dir
		if _, err := resolveCmd.Output(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not resolve thread %s: %v\n", id, err)
		}
	}

	fmt.Printf("resolved %d comment thread(s)\n", len(ids))
	return nil
}
