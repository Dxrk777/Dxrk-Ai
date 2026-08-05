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

func RegisterDiffCommand() {
	cmd := &cobra.Command{
		Use:   "diff [ref1] [ref2]",
		Short: "Show diff of changes",
		Long: `Display differences between git states.

Without arguments: shows unstaged changes.
With --staged: shows staged changes.
With one ref: shows changes since that ref.
With two refs: shows changes between those refs.
With --path: filters to a specific file path.

Examples:
  dxrk diff
  dxrk diff --staged
  dxrk diff HEAD
  dxrk diff main..feature
  dxrk diff --path internal/server.go`,
		Args: cobra.MaximumNArgs(2),
		RunE: runDiff,
	}

	cmd.Flags().BoolP("staged", "s", false, "Show staged changes")
	cmd.Flags().String("path", "", "Filter diff to a specific file path")
	cmd.Flags().BoolP("stat", "t", false, "Show only diffstat (summary)")

	cli.AddCommand(cmd)
}

func runDiff(cmd *cobra.Command, args []string) error {
	staged, _ := cmd.Flags().GetBool("staged")
	path, _ := cmd.Flags().GetString("path")
	statOnly, _ := cmd.Flags().GetBool("stat")

	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	runner := git.NewRunner(wd)
	ctx := cmd.Context()

	if len(args) == 0 {
		// Show unstaged or staged changes
		diff, err := runner.Diff(ctx, staged, path)
		if err != nil {
			return fmt.Errorf("get diff: %w", err)
		}
		printDiff(diff, statOnly)
		return nil
	}

	if len(args) == 1 {
		// Diff since ref
		diffArgs := []string{"diff", args[0], "HEAD"}
		if path != "" {
			diffArgs = append(diffArgs, "--", path)
		}
		diff, err := gitDiffRef(ctx, wd, diffArgs)
		if err != nil {
			return fmt.Errorf("get diff: %w", err)
		}
		printDiff(diff, statOnly)
		return nil
	}

	// Diff between two refs
	diffArgs := []string{"diff", args[0], args[1]}
	if path != "" {
		diffArgs = append(diffArgs, "--", path)
	}
	diff, err := gitDiffRef(ctx, wd, diffArgs)
	if err != nil {
		return fmt.Errorf("get diff: %w", err)
	}
	printDiff(diff, statOnly)
	return nil
}

func printDiff(diff *git.DiffResult, statOnly bool) {
	if diff.Stats.FilesChanged == 0 {
		fmt.Println("no changes")
		return
	}

	if statOnly {
		fmt.Printf("%d files changed, %d insertions(+), %d deletions(-)\n",
			diff.Stats.FilesChanged, diff.Stats.Additions, diff.Stats.Deletions)
		for _, f := range diff.Files {
			fmt.Printf("  %s (+%d, -%d)\n", f.Path, f.Additions, f.Deletions)
		}
		return
	}

	for _, f := range diff.Files {
		fmt.Printf("--- %s\n", f.Path)
		fmt.Print(f.Content)
	}
}

func gitDiffRef(ctx context.Context, dir string, args []string) (*git.DiffResult, error) {
	cmd := exec.CommandContext(ctx, "git", args...) //nolint:gosec
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git diff: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return parseDiffOutput(string(out)), nil
}

func parseDiffOutput(out string) *git.DiffResult {
	res := &git.DiffResult{}
	if out == "" {
		return res
	}
	lines := strings.Split(out, "\n")
	var cur *git.DiffFile
	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git ") {
			if cur != nil {
				res.Files = append(res.Files, *cur)
			}
			cur = &git.DiffFile{}
			parts := strings.Split(line, " ")
			if len(parts) >= 4 {
				cur.Path = strings.TrimPrefix(parts[len(parts)-1], "b/")
			}
			continue
		}
		if strings.HasPrefix(line, "--- ") || strings.HasPrefix(line, "+++") {
			continue
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
	return res
}
