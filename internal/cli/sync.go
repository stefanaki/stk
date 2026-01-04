package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/gstefan/stk/internal/ui"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Fetch, rebase, and push the entire stack",
	Long: `Synchronize the stack with the remote.

This command performs the following steps:
  1. Fetch updates from origin
  2. Rebase the base branch onto its upstream
  3. Rebase the entire stack
  4. Push all branches to origin (with --force-with-lease)
  5. Update all PR descriptions with current stack info

Use --no-push to skip the push step.
Use --no-fetch to skip fetching.
Use --no-update-prs to skip updating PR descriptions.

Examples:
  stk sync                  # Full sync
  stk sync --no-push        # Rebase only, don't push
  stk sync --no-update-prs  # Don't update PR descriptions`,
	RunE: runSync,
}

var (
	syncNoPush      bool
	syncNoFetch     bool
	syncNoUpdatePRs bool
)

func init() {
	syncCmd.Flags().BoolVar(&syncNoPush, "no-push", false, "don't push after rebasing")
	syncCmd.Flags().BoolVar(&syncNoFetch, "no-fetch", false, "don't fetch before rebasing")
	syncCmd.Flags().BoolVar(&syncNoUpdatePRs, "no-update-prs", false, "don't update PR descriptions")
	viper.BindPFlag("sync.no-push", syncCmd.Flags().Lookup("no-push"))
	viper.BindPFlag("sync.no-update-prs", syncCmd.Flags().Lookup("no-update-prs"))
	rootCmd.AddCommand(syncCmd)
}

func runSync(cmd *cobra.Command, args []string) error {
	stack := RequireStack()
	RequireCleanTree()

	// Step 1: Fetch
	if !syncNoFetch {
		fmt.Println(ui.IconArrow + " Fetching from origin...")
		if err := Git().Fetch("origin"); err != nil {
			ui.Warning("Failed to fetch: %v", err)
			// Continue anyway
		}
	}

	// Step 2: Update base branch if it has an upstream
	if Git().RemoteBranchExists("origin", stack.Base) {
		fmt.Printf("%s Updating base branch %s...\n", ui.IconArrow, stack.Base)

		currentBranch, _ := Git().CurrentBranch()

		if err := Git().Checkout(stack.Base); err != nil {
			return fmt.Errorf("failed to checkout base: %w", err)
		}

		// Try to fast-forward or rebase
		if err := Git().Run("pull", "--rebase", "origin", stack.Base); err != nil {
			ui.Warning("Failed to update base branch: %v", err)
			// Continue anyway, might already be up to date
		}

		// Return to original branch
		if currentBranch != "" && currentBranch != stack.Base {
			_ = Git().CheckoutSilent(currentBranch)
		}
	}

	// Step 3: Rebase the stack
	fmt.Println()
	if len(stack.Branches) > 0 {
		// Use the rebase logic
		if err := runRebase(cmd, []string{}); err != nil {
			return err
		}
	}

	// Step 4: Push
	if !syncNoPush {
		fmt.Println()
		fmt.Println(ui.IconArrow + " Pushing branches to origin...")

		for _, branch := range stack.Branches {
			fmt.Printf("  Pushing %s...\n", branch.Name)
			if err := Git().Push("origin", branch.Name, true); err != nil {
				ui.Warning("Failed to push %s: %v", branch.Name, err)
			}
		}
	}

	// Step 5: Update PR descriptions
	if !syncNoUpdatePRs && !syncNoPush {
		// Only update PRs if we pushed (otherwise descriptions would be stale)
		hasPRs := false
		for _, branch := range stack.Branches {
			if branch.PR != nil && branch.PR.Number > 0 {
				hasPRs = true
				break
			}
		}

		if hasPRs {
			fmt.Println()
			fmt.Println(ui.IconArrow + " Updating PR descriptions...")

			provider, err := getProvider()
			if err != nil {
				ui.Warning("Failed to get PR provider: %v", err)
			} else {
				if err := UpdateAllPRDescriptions(stack, provider); err != nil {
					ui.Warning("Failed to update PR descriptions: %v", err)
				}
			}
		}
	}

	fmt.Println()
	ui.Success("Sync complete")
	return nil
}

var pushCmd = &cobra.Command{
	Use:   "push [branch]",
	Short: "Push branches to origin",
	Long: `Push stack branches to the remote.

Without arguments, pushes all branches in the stack.
With a branch name, pushes only that branch.

Uses --force-with-lease for safety.

Examples:
  stk push              # Push all branches
  stk push feature-api  # Push single branch
  stk push --all        # Explicitly push all`,
	RunE: runPush,
}

var (
	pushAll    bool
	pushRemote string
)

func init() {
	pushCmd.Flags().BoolVar(&pushAll, "all", false, "push all branches")
	pushCmd.Flags().StringVar(&pushRemote, "remote", "origin", "remote to push to")
	rootCmd.AddCommand(pushCmd)
}

func runPush(cmd *cobra.Command, args []string) error {
	stack := RequireStack()

	var branches []string

	if len(args) > 0 {
		// Push specific branch
		branchName := args[0]
		if !stack.HasBranch(branchName) && branchName != stack.Base {
			return fmt.Errorf("branch %q not in stack", branchName)
		}
		branches = []string{branchName}
	} else {
		// Push all stack branches
		for _, b := range stack.Branches {
			branches = append(branches, b.Name)
		}
	}

	if len(branches) == 0 {
		ui.Info("No branches to push")
		return nil
	}

	fmt.Printf("Pushing %d branch(es) to %s...\n", len(branches), pushRemote)

	for _, branch := range branches {
		fmt.Printf("%s Pushing %s\n", ui.IconArrow, branch)
		if err := Git().Push(pushRemote, branch, true); err != nil {
			return fmt.Errorf("failed to push %s: %w", branch, err)
		}
	}

	ui.Success("Push complete")
	return nil
}

var fetchCmd = &cobra.Command{
	Use:   "fetch",
	Short: "Fetch updates from remote",
	Long:  `Fetch updates from the remote repository.`,
	RunE:  runFetch,
}

func init() {
	rootCmd.AddCommand(fetchCmd)
}

func runFetch(cmd *cobra.Command, args []string) error {
	fmt.Println("Fetching from origin...")
	if err := Git().Fetch("origin"); err != nil {
		return fmt.Errorf("failed to fetch: %w", err)
	}
	ui.Success("Fetch complete")
	return nil
}
