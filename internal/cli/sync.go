package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/stefanaki/stk/internal/stack"
	"github.com/stefanaki/stk/internal/ui"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync local stack with remote state",
	Long: `Synchronize the local stack with the remote.

This command performs the following steps:
  1. Fetch updates from origin
  2. Update base branch (pull --rebase)
  3. Refresh PR states from remote
  4. Process merged PRs (remove from stack, retarget downstream PRs)
  5. Process closed PRs (clear PR metadata, will recreate on submit)
  6. Rebase entire stack onto updated base

This command never pushes to the remote. Use 'stk submit' to push and manage PRs.

Use --no-fetch to skip fetching (local rebase only).
Use --no-rebase to only refresh PR states.
Use --delete-merged to delete local branches for merged PRs.

Examples:
  stk sync                # Full sync with remote
  stk sync --no-fetch     # Local rebase only
  stk sync --no-rebase    # Only refresh PR states`,
	RunE: runSync,
}

var (
	syncNoFetch      bool
	syncNoRebase     bool
	syncDeleteMerged bool
)

func init() {
	syncCmd.Flags().BoolVar(&syncNoFetch, "no-fetch", false, "skip fetching from remote")
	syncCmd.Flags().BoolVar(&syncNoRebase, "no-rebase", false, "only refresh PR states, don't rebase")
	syncCmd.Flags().BoolVar(&syncDeleteMerged, "delete-merged", false, "delete local branches for merged PRs")
	rootCmd.AddCommand(syncCmd)
}

func runSync(cmd *cobra.Command, args []string) error {
	stk := RequireStack()
	RequireCleanTree()

	// Step 1: Fetch
	if !syncNoFetch {
		fmt.Println(ui.IconArrow + " Fetching from origin...")
		if err := Git().Fetch("origin"); err != nil {
			ui.Warning("Failed to fetch: %v", err)
		}
	}

	// Step 2: Update base branch if it has an upstream
	if !syncNoRebase && Git().RemoteBranchExists("origin", stk.Base) {
		fmt.Printf("%s Updating base branch %s...\n", ui.IconArrow, stk.Base)

		originalBranch, _ := Git().CurrentBranch()

		if err := Git().Checkout(stk.Base); err != nil {
			return fmt.Errorf("failed to checkout base: %w", err)
		}

		if err := Git().Run("pull", "--rebase", "origin", stk.Base); err != nil {
			ui.Warning("Failed to update base branch: %v", err)
		}

		if originalBranch != "" && originalBranch != stk.Base {
			_ = Git().CheckoutSilent(originalBranch)
		}
	}

	// Step 3: Refresh PR states from remote
	fmt.Println()
	fmt.Println(ui.IconArrow + " Refreshing PR states...")

	provider, err := getProvider()
	if err != nil {
		ui.Warning("Failed to get PR provider: %v", err)
		provider = nil
	}

	var mergedBranches []string
	var closedBranches []string

	if provider != nil {
		for _, branch := range stk.Branches {
			if branch.PR == nil || branch.PR.Number == 0 {
				continue
			}

			remotePR, err := provider.Get(branch.PR.Number)
			if err != nil {
				ui.Warning("Failed to fetch PR #%d: %v", branch.PR.Number, err)
				continue
			}

			// Update local state
			_ = Manager().UpdatePR(stk, branch.Name, &stack.PR{
				Number: remotePR.Number,
				URL:    remotePR.URL,
				State:  remotePR.State,
				Title:  remotePR.Title,
			})

			switch remotePR.State {
			case "merged":
				fmt.Printf("  PR #%d (%s): %s%s%s\n", remotePR.Number, branch.Name, ui.Magenta, "merged", ui.Reset)
				mergedBranches = append(mergedBranches, branch.Name)
			case "closed":
				fmt.Printf("  PR #%d (%s): %s%s%s\n", remotePR.Number, branch.Name, ui.Red, "closed", ui.Reset)
				closedBranches = append(closedBranches, branch.Name)
			default:
				fmt.Printf("  PR #%d (%s): %s%s%s\n", remotePR.Number, branch.Name, ui.Green, remotePR.State, ui.Reset)
			}
		}
	}

	// Step 4: Process merged PRs
	if len(mergedBranches) > 0 {
		fmt.Println()
		fmt.Println(ui.IconArrow + " Processing merged branches...")

		for _, branchName := range mergedBranches {
			// Reload stack to get fresh state
			stk, _ = Manager().Current()

			idx := stk.FindBranch(branchName)
			if idx < 0 {
				continue
			}

			fmt.Printf("  Removing %s from stack\n", branchName)

			// Retarget all downstream PRs
			if provider != nil {
				newBase := stk.Base
				if idx > 0 {
					newBase = stk.Branches[idx-1].Name
				}

				// Retarget all PRs after the merged one
				for i := idx + 1; i < len(stk.Branches); i++ {
					downstream := stk.Branches[i]
					if downstream.PR != nil && downstream.PR.Number > 0 {
						targetBase := newBase
						if i > idx+1 {
							// PRs after the immediate child keep their current parent
							// (which will be adjusted after removal)
							targetBase = stk.Branches[i-1].Name
						}

						// Only retarget immediate child
						if i == idx+1 {
							fmt.Printf("  Retargeting PR #%d to %s\n", downstream.PR.Number, targetBase)
							if err := provider.Retarget(downstream.PR.Number, targetBase); err != nil {
								ui.Warning("Failed to retarget PR #%d: %v", downstream.PR.Number, err)
							}
						}
					}
				}
			}

			// Remove from stack
			if err := Manager().RemoveBranch(stk, branchName); err != nil {
				ui.Warning("Failed to remove %s from stack: %v", branchName, err)
			}

			// Optionally delete local branch
			if syncDeleteMerged {
				fmt.Printf("  Deleting local branch %s\n", branchName)
				if err := Git().DeleteBranch(branchName, true); err != nil {
					ui.Warning("Failed to delete branch %s: %v", branchName, err)
				}
			}
		}
	}

	// Step 5: Process closed PRs (clear metadata, will recreate on submit)
	if len(closedBranches) > 0 {
		fmt.Println()
		fmt.Println(ui.IconArrow + " Processing closed PRs...")

		for _, branchName := range closedBranches {
			fmt.Printf("  Cleared PR metadata for %s (will recreate on submit)\n", branchName)
			_ = Manager().UpdatePR(stk, branchName, nil)
		}
	}

	// Reload stack after modifications
	stk, _ = Manager().Current()

	// Step 6: Rebase stack
	if !syncNoRebase && len(stk.Branches) > 0 {
		fmt.Println()
		if err := rebaseStack(stk); err != nil {
			return err
		}
	}

	fmt.Println()
	ui.Success("Sync complete")
	return nil
}

// rebaseStack rebases all branches in the stack atomically.
func rebaseStack(stk *stack.Stack) error {
	if len(stk.Branches) == 0 {
		return nil
	}

	originalBranch, _ := Git().CurrentBranch()

	// Take snapshot for atomic rollback
	fmt.Println(ui.IconCamera + " Saving branch positions for rollback...")
	if err := Manager().TakeSnapshot(stk, func(name string) (string, error) {
		return Git().SHA(name)
	}); err != nil {
		return fmt.Errorf("failed to take snapshot: %w", err)
	}

	// Perform rebases
	for i := range stk.Branches {
		branch := stk.Branches[i].Name
		var base string
		if i == 0 {
			base = stk.Base
		} else {
			base = stk.Branches[i-1].Name
		}

		fmt.Printf("%s Rebasing %s%s%s onto %s%s%s\n",
			ui.IconArrow,
			ui.Bold, branch, ui.Reset,
			ui.Dim, base, ui.Reset)

		if err := Git().RebaseBranchOnto(branch, base); err != nil {
			ui.Error("Rebase failed")
			rollbackStack(stk, originalBranch)
			return fmt.Errorf("rebase failed")
		}
	}

	// Clear snapshot on success
	_ = Manager().ClearSnapshot(stk)

	// Return to original branch if possible
	if originalBranch != "" {
		_ = Git().CheckoutSilent(originalBranch)
	}

	return nil
}

// rollbackStack restores all branches to their snapshot positions.
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
			continue
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

	if originalBranch != "" {
		_ = Git().CheckoutSilent(originalBranch)
	}

	_ = Manager().ClearSnapshot(stk)

	fmt.Println()
	ui.Success("Rollback complete - stack restored to original state")
}
