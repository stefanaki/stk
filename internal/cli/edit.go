package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/stefanaki/stk/internal/ui"
)

var editCmd = &cobra.Command{
	Use:   "edit [branch]",
	Short: "Interactive rebase within a branch",
	Long: `Start an interactive rebase for commits within a single branch.

This allows you to edit, squash, or reorder commits within the current
(or specified) branch, from the parent branch.

After editing, run 'stk sync --no-fetch' to propagate changes through the stack.

Examples:
  stk edit              # Edit current branch's commits
  stk edit feature-api  # Edit specific branch's commits`,
	RunE: runEdit,
}

func init() {
	rootCmd.AddCommand(editCmd)
}

func runEdit(cmd *cobra.Command, args []string) error {
	stk := RequireStack()
	RequireCleanTree()

	var branch string
	if len(args) > 0 {
		branch = args[0]
		if !stk.HasBranch(branch) {
			return fmt.Errorf("branch %q not in stack", branch)
		}
	} else {
		var err error
		branch, err = Git().CurrentBranch()
		if err != nil {
			return fmt.Errorf("could not determine current branch: %w", err)
		}
		if !stk.HasBranch(branch) {
			return fmt.Errorf("current branch %q not in stack", branch)
		}
	}

	// Checkout the branch if needed
	currentBranch, _ := Git().CurrentBranch()
	if currentBranch != branch {
		if err := Git().Checkout(branch); err != nil {
			return err
		}
	}

	// Get parent
	parent := stk.GetParent(branch)

	fmt.Printf("%s Starting interactive rebase of %s%s%s onto %s%s%s\n",
		ui.IconArrow,
		ui.Bold, branch, ui.Reset,
		ui.Dim, parent, ui.Reset)
	fmt.Println()
	fmt.Println("After editing, run 'stk sync --no-fetch' to propagate changes through the stack.")
	fmt.Println()

	return Git().RebaseInteractive(parent)
}
