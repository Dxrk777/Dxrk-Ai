// SPDX-License-Identifier: MIT
package commands

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Dxrk777/Dxrk-Ai/internal/cli"
	"github.com/Dxrk777/Dxrk-Ai/internal/git"
	"github.com/Dxrk777/Dxrk-Ai/internal/strconst"
)

func RegisterReviewCommand() {
	cmd := &cobra.Command{
		Use:   "review [pr-number]",
		Short: "Review code changes in a PR or local branch",
		Long: `Perform a code review of changes.

Without arguments: reviews local uncommitted changes.
With a PR number: reviews the specified GitHub PR.
With --local: reviews changes since the default branch.

Examples:
  dxrk review
  dxrk review 42
  dxrk review --local
  dxrk review --since main`,
		Args: cobra.MaximumNArgs(1),
		RunE: runReview,
	}

	cmd.Flags().Bool(strconst.StrLocal, false, "Review local changes since default branch")
	cmd.Flags().String("since", "", "Review changes since specified ref")

	cli.AddCommand(cmd)
}

func runReview(cmd *cobra.Command, args []string) error {
	local, _ := cmd.Flags().GetBool(strconst.StrLocal)
	since, _ := cmd.Flags().GetString("since")

	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	runner := git.NewRunner(wd)
	ctx := cmd.Context()

	// PR review mode
	if len(args) > 0 && !local {
		prNum := args[0]
		return reviewPR(ctx, wd, prNum)
	}

	// Local review mode
	if local || since != "" {
		ref := since
		if ref == "" {
			ref = detectDefaultBranch(wd)
		}
		return reviewLocal(ctx, runner, ref)
	}

	// Review uncommitted changes
	return reviewUncommitted(ctx, runner)
}

func reviewPR(ctx context.Context, dir, prNum string) error {
	// Get PR info
	cmd := exec.CommandContext(ctx, "gh", "pr", "view", prNum, "--json", "title,body,state,additions,deletions,changedFiles") //nolint:gosec
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("get PR info: %w", err)
	}
	fmt.Printf("PR #%s details:\n%s\n\n", prNum, string(out))

	// Get PR diff
	cmd = exec.CommandContext(ctx, "gh", "pr", "diff", prNum) //nolint:gosec
	cmd.Dir = dir
	diffOut, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("get PR diff: %w", err)
	}

	printReviewOutput(string(diffOut))
	return nil
}

func reviewLocal(ctx context.Context, _ *git.Runner, sinceRef string) error {
	wd, _ := os.Getwd()
	diffArgs := []string{"diff", sinceRef, "HEAD"}
	diff, err := gitDiffRef(ctx, wd, diffArgs)
	if err != nil {
		return fmt.Errorf("get diff: %w", err)
	}

	fmt.Printf("Reviewing changes since %s:\n", sinceRef)
	fmt.Printf("%d files changed, %d insertions(+), %d deletions(-)\n\n",
		diff.Stats.FilesChanged, diff.Stats.Additions, diff.Stats.Deletions)

	printReviewOutput(formatDiffResult(diff))
	return nil
}

func reviewUncommitted(ctx context.Context, runner *git.Runner) error {
	// Get staged diff
	stagedDiff, err := runner.Diff(ctx, true, "")
	if err != nil {
		return fmt.Errorf("get staged diff: %w", err)
	}

	// Get unstaged diff
	unstagedDiff, err := runner.Diff(ctx, false, "")
	if err != nil {
		return fmt.Errorf("get unstaged diff: %w", err)
	}

	totalAdditions := stagedDiff.Stats.Additions + unstagedDiff.Stats.Additions
	totalDeletions := stagedDiff.Stats.Deletions + unstagedDiff.Stats.Deletions
	totalFiles := stagedDiff.Stats.FilesChanged + unstagedDiff.Stats.FilesChanged

	fmt.Println("Reviewing uncommitted changes:")
	fmt.Printf("%d files changed, %d insertions(+), %d deletions(-)\n\n",
		totalFiles, totalAdditions, totalDeletions)

	if stagedDiff.Stats.FilesChanged > 0 {
		fmt.Println("=== STAGED ===")
		printReviewOutput(formatDiffResult(stagedDiff))
	}

	if unstagedDiff.Stats.FilesChanged > 0 {
		fmt.Println("=== UNSTAGED ===")
		printReviewOutput(formatDiffResult(unstagedDiff))
	}

	return nil
}

func formatDiffResult(diff *git.DiffResult) string {
	var sb strings.Builder
	for _, f := range diff.Files {
		fmt.Fprintf(&sb, "--- %s\n", f.Path)
		sb.WriteString(f.Content)
	}
	return sb.String()
}

func printReviewOutput(content string) {
	if content == "" {
		fmt.Println("No changes to review.")
		return
	}
	fmt.Println("=== REVIEW ===")
	fmt.Println(content)
	fmt.Println("=== SUMMARY ===")
	fmt.Println("Review the changes above for:")
	fmt.Println("  - Code correctness and logic")
	fmt.Println("  - Error handling")
	fmt.Println("  - Performance implications")
	fmt.Println("  - Test coverage")
	fmt.Println("  - Security considerations")
	fmt.Println("  - Following project conventions")
}
