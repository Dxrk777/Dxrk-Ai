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
)

func RegisterCommitCommand() {
	cmd := &cobra.Command{
		Use:   "commit [message]",
		Short: "Stage changes and create a conventional commit",
		Long: `Stage all changes and create a commit following conventional commit format.

If no message is provided, analyzes the diff to generate an appropriate
conventional commit message based on the nature of changes.

Examples:
  dxrk commit "feat: add user authentication"
  dxrk commit "fix: resolve null pointer in parser"
  dxrk commit --all`,
		Args: cobra.MaximumNArgs(1),
		RunE: runCommit,
	}

	cmd.Flags().BoolP("all", "a", false, "Stage all changes before committing")
	cmd.Flags().Bool("no-verify", false, "Skip pre-commit hooks")

	cli.AddCommand(cmd)
}

func runCommit(cmd *cobra.Command, args []string) error {
	all, _ := cmd.Flags().GetBool("all")
	noVerify, _ := cmd.Flags().GetBool("no-verify")

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

	// Build commit args
	commitArgs := []string{"commit", "-m", msg}
	if noVerify {
		commitArgs = append(commitArgs, "--no-verify")
	}

	// Execute commit directly via exec to stream output
	c := exec.CommandContext(ctx, "git", commitArgs...) //nolint:gosec
	c.Dir = wd
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr

	if err := c.Run(); err != nil {
		return fmt.Errorf("git commit: %w", err)
	}

	fmt.Printf("committed: %s\n", msg)
	return nil
}

func generateCommitMessage(ctx context.Context, runner *git.Runner) (string, error) {
	diff, err := runner.Diff(ctx, false, "")
	if err != nil {
		return "", fmt.Errorf("get diff: %w", err)
	}

	if diff.Stats.FilesChanged == 0 {
		return "", fmt.Errorf("no changes to commit")
	}

	// Determine commit type based on file changes
	commitType := "chore"
	switch {
	case hasNewFiles(diff):
		commitType = "feat"
	case hasOnlyTestFiles(diff):
		commitType = "test"
	case hasOnlyDocFiles(diff):
		commitType = "docs"
	}

	// Build scope from changed files
	scope := buildScope(diff)

	// Build description
	desc := fmt.Sprintf("%d files changed, %d insertions(+), %d deletions(-)",
		diff.Stats.FilesChanged, diff.Stats.Additions, diff.Stats.Deletions)

	if scope != "" {
		return fmt.Sprintf("%s(%s): %s", commitType, scope, desc), nil
	}
	return fmt.Sprintf("%s: %s", commitType, desc), nil
}

func hasNewFiles(diff *git.DiffResult) bool {
	for _, f := range diff.Files {
		if f.Status == "A" {
			return true
		}
	}
	return false
}

func hasOnlyTestFiles(diff *git.DiffResult) bool {
	for _, f := range diff.Files {
		if !strings.Contains(f.Path, "_test.go") &&
			!strings.Contains(f.Path, ".test.") &&
			!strings.Contains(f.Path, "test_") {
			return false
		}
	}
	return len(diff.Files) > 0
}

func hasOnlyDocFiles(diff *git.DiffResult) bool {
	docExts := []string{".md", ".txt", ".rst", ".adoc"}
	for _, f := range diff.Files {
		isDoc := false
		for _, ext := range docExts {
			if strings.HasSuffix(f.Path, ext) {
				isDoc = true
				break
			}
		}
		if !isDoc {
			return false
		}
	}
	return len(diff.Files) > 0
}

func buildScope(diff *git.DiffResult) string {
	if len(diff.Files) == 0 {
		return ""
	}
	if len(diff.Files) == 1 {
		parts := strings.Split(diff.Files[0].Path, "/")
		if len(parts) > 1 {
			return parts[0]
		}
		return ""
	}

	// Find common directory prefix
	parts := strings.Split(diff.Files[0].Path, "/")
	common := parts[:len(parts)-1]
	for _, f := range diff.Files[1:] {
		fp := strings.Split(f.Path, "/")
		newCommon := make([]string, 0, min(len(common), len(fp)))
		for i := 0; i < len(common) && i < len(fp); i++ {
			if common[i] == fp[i] {
				newCommon = append(newCommon, common[i])
			} else {
				break
			}
		}
		common = newCommon
	}
	if len(common) == 0 {
		return ""
	}
	return strings.Join(common, "/")
}
