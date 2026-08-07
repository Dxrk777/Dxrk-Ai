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

func RegisterBranchCommand() {
	cmd := &cobra.Command{
		Use:   "branch [name]",
		Short: "Branch operations (create, switch, list, delete)",
		Long: `Manage git branches.

Without arguments: lists local branches.
With a name: creates and switches to a new branch.
With --delete: deletes the specified branch.
With --switch: switches to an existing branch.
With --list: lists all branches (local and remote).

Examples:
  dxrk branch
  dxrk branch feature/auth
  dxrk branch --switch main
  dxrk branch --delete old-feature
  dxrk branch --list`,
		Args: cobra.MaximumNArgs(1),
		RunE: runBranch,
	}

	cmd.Flags().BoolP("delete", "d", false, "Delete the specified branch")
	cmd.Flags().BoolP("switch", "s", false, "Switch to an existing branch")
	cmd.Flags().BoolP("list", "l", false, "List all branches (local and remote)")
	cmd.Flags().Bool("force", false, "Force delete or create")

	cli.AddCommand(cmd)
}

func runBranch(cmd *cobra.Command, args []string) error {
	deleteBranch, _ := cmd.Flags().GetBool("delete")
	switchBranch, _ := cmd.Flags().GetBool("switch")
	listAll, _ := cmd.Flags().GetBool("list")
	force, _ := cmd.Flags().GetBool("force")

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

	// List branches
	if listAll || len(args) == 0 && !deleteBranch && !switchBranch {
		branches, err := runner.Branch(ctx, listAll)
		if err != nil {
			return fmt.Errorf("list branches: %w", err)
		}

		for _, b := range branches {
			prefix := "  "
			if b.IsCurrent {
				prefix = "* "
			}
			fmt.Printf("%s%s\n", prefix, b.Name)
		}
		return nil
	}

	// Delete branch
	if deleteBranch {
		if len(args) == 0 {
			return fmt.Errorf("branch name required for delete")
		}
		branchName := args[0]
		if err := deleteGitBranch(ctx, wd, branchName, force); err != nil {
			return fmt.Errorf("delete branch: %w", err)
		}
		fmt.Printf("deleted branch: %s\n", branchName)
		return nil
	}

	// Switch branch
	if switchBranch {
		if len(args) == 0 {
			return fmt.Errorf("branch name required for switch")
		}
		branchName := args[0]
		if err := runner.Checkout(ctx, branchName, false); err != nil {
			return fmt.Errorf("switch branch: %w", err)
		}
		fmt.Printf("switched to branch: %s\n", branchName)
		return nil
	}

	// Create new branch
	if len(args) > 0 {
		branchName := args[0]
		if err := runner.Checkout(ctx, branchName, true); err != nil {
			return fmt.Errorf("create branch: %w", err)
		}
		fmt.Printf("created and switched to branch: %s\n", branchName)
		return nil
	}

	return nil
}

func deleteGitBranch(ctx context.Context, dir, branch string, force bool) error {
	args := []string{"branch"}
	if force {
		args = append(args, "-D")
	} else {
		args = append(args, "-d")
	}
	args = append(args, branch)

	cmd := exec.CommandContext(ctx, "git", args...) //nolint:gosec
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}
