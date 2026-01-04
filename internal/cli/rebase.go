package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/stefanaki/stk/internal/stack"
	"github.com/stefanaki/stk/internal/ui"
)

var rebaseCmd = &cobra.Command{
	Use:   "rebase",
	Short: "Rebase the entire stack",
	Long: `Rebase all branches in the stack onto their parents.

This operation is atomic by default - if any rebase fails, all branches
are rolled back to their original positions.

The rebase proceeds from the first branch to the last:
  1. Rebase first branch onto base
  2. Rebase second branch onto first
  3. And so on...

Examples:
  stk rebase                    # Rebase entire stack
  stk rebase --from feature-api # Start from a specific branch
  stk rebase --to feature-api   # Stop at a specific branch
  stk rebase --no-atomic        # Don't rollback on failure`,
	RunE: runRebase,
}

var (
	rebaseFrom     string
	rebaseTo       string
	rebaseNoAtomic bool
)

func init() {
	rebaseCmd.Flags().StringVar(&rebaseFrom, "from", "", "start rebase from this branch")
	rebaseCmd.Flags().StringVar(&rebaseTo, "to", "", "stop rebase at this branch")
	rebaseCmd.Flags().BoolVar(&rebaseNoAtomic, "no-atomic", false, "don't rollback on failure")
	rootCmd.AddCommand(rebaseCmd)
}

func runRebase(cmd *cobra.Command, args []string) error {
	stack := RequireStack()
	RequireCleanTree()

	if len(stack.Branches) == 0 {
		ui.Info("Stack has no branches to rebase")
		return nil
	}

	// Save original branch to return to
	originalBranch, _ := Git().CurrentBranch()

	// Determine start and end indices
	startIdx := 0
	endIdx := len(stack.Branches) - 1

	if rebaseFrom != "" {
		startIdx = stack.FindBranch(rebaseFrom)
		if startIdx < 0 {
			return fmt.Errorf("branch %q not found in stack", rebaseFrom)
		}
	}

	if rebaseTo != "" {
		endIdx = stack.FindBranch(rebaseTo)
		if endIdx < 0 {
			return fmt.Errorf("branch %q not found in stack", rebaseTo)
		}
	}

	if startIdx > endIdx {
		return fmt.Errorf("--from branch must come before --to branch in stack")
	}

	// Take snapshot for atomic rollback (unless disabled)
	if !rebaseNoAtomic {
		fmt.Println(ui.IconCamera + " Saving branch positions for rollback...")
		if err := Manager().TakeSnapshot(stack, func(name string) (string, error) {
			return Git().SHA(name)
		}); err != nil {
			return fmt.Errorf("failed to take snapshot: %w", err)
		}
	}

	// Perform rebases
	success := true
	for i := startIdx; i <= endIdx; i++ {
		branch := stack.Branches[i].Name
		var base string
		if i == 0 {
			base = stack.Base
		} else {
			base = stack.Branches[i-1].Name
		}

		fmt.Printf("\n%s Rebasing %s%s%s onto %s%s%s\n",
			ui.IconArrow,
			ui.Bold, branch, ui.Reset,
			ui.Dim, base, ui.Reset)

		if err := Git().RebaseBranchOnto(branch, base); err != nil {
			ui.Error("Rebase failed")
			success = false

			if !rebaseNoAtomic {
				rollbackStack(stack, originalBranch)
			} else {
				fmt.Println("\nResolve conflicts, then run:")
				fmt.Println("  git rebase --continue")
				fmt.Println("Then continue with:")
				fmt.Printf("  stk rebase --from %s\n", branch)
			}
			return fmt.Errorf("rebase failed")
		}
	}

	// Clear snapshot on success
	if success && !rebaseNoAtomic {
		_ = Manager().ClearSnapshot(stack)
	}

	// Return to original branch if possible
	if originalBranch != "" {
		_ = Git().CheckoutSilent(originalBranch)
	}

	fmt.Println()
	ui.Success("Stack rebase complete")
	return nil
}

func rollbackStack(stk *stack.Stack, originalBranch string) {
	if stk.Snapshot == nil {
		ui.Warning("No snapshot available for rollback")
		return
	}

	fmt.Printf("\n%s Rolling back all branches...\n", ui.IconRollback)

	// Abort any in-progress rebase
	_ = Git().RebaseAbort()

	// Reset all branches to their snapshot SHAs
	for branchName, sha := range stk.Snapshot.Refs {
		if branchName == stk.Base {
			continue // Don't touch base branch
		}
		shortSHA := sha
		if len(shortSHA) > 8 {
			shortSHA = shortSHA[:8]
		}
		fmt.Printf("  Resetting %s to %s\n", branchName, shortSHA)
		if err := Git().ResetBranchToSHA(branchName, sha); err != nil {
			ui.Warning("Failed to reset %s: %v", branchName, err)
		}
	}

	// Return to original branch
	if originalBranch != "" {
		_ = Git().CheckoutSilent(originalBranch)
	}

	// Clear the snapshot
	_ = Manager().ClearSnapshot(stk)

	fmt.Println()
	ui.Success("Rollback complete - stack restored to original state")
}

var editCmd = &cobra.Command{
	Use:   "edit [branch]",
	Short: "Interactive rebase within a branch",
	Long: `Start an interactive rebase for commits within a single branch.

This allows you to edit, squash, or reorder commits within the current
(or specified) branch, from the parent branch.

Examples:
  stk edit              # Edit current branch's commits
  stk edit feature-api  # Edit specific branch's commits`,
	RunE: runEdit,
}

func init() {
	rootCmd.AddCommand(editCmd)
}

func runEdit(cmd *cobra.Command, args []string) error {
	stack := RequireStack()
	RequireCleanTree()

	var branch string
	if len(args) > 0 {
		branch = args[0]
		if !stack.HasBranch(branch) {
			return fmt.Errorf("branch %q not in stack", branch)
		}
	} else {
		var err error
		branch, err = Git().CurrentBranch()
		if err != nil {
			return fmt.Errorf("could not determine current branch: %w", err)
		}
		if !stack.HasBranch(branch) {
			return fmt.Errorf("current branch %q not in stack", branch)
		}
	}

	// Checkout the branch
	if err := Git().Checkout(branch); err != nil {
		return err
	}

	// Get parent
	parent := stack.GetParent(branch)

	fmt.Printf("Starting interactive rebase of %s onto %s\n", branch, parent)
	return Git().RebaseInteractive(parent)
}
