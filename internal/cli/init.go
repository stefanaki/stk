package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/gstefan/stk/internal/ui"
)

var initCmd = &cobra.Command{
	Use:   "init <stack-name> [--base <branch>]",
	Short: "Initialize a new stack",
	Long: `Initialize a new stack with the given name.

The current branch will be used as the starting point. If --base is not
specified, the tool will try to detect the default branch (main/master)
or use the upstream branch.

Examples:
  stk init my-feature              # Create stack, auto-detect base
  stk init my-feature --base main  # Create stack with explicit base
  stk init my-feature -b develop   # Use develop as base`,
	Args: cobra.ExactArgs(1),
	RunE: runInit,
}

var initBase string

func init() {
	initCmd.Flags().StringVarP(&initBase, "base", "b", "", "base branch for the stack")
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	stackName := args[0]

	// Check if stack already exists
	if Manager().Storage().Exists(stackName) {
		return fmt.Errorf("stack %q already exists", stackName)
	}

	// Determine base branch
	base := initBase
	if base == "" {
		// Try to auto-detect
		var err error
		base, err = Git().DefaultBranch()
		if err != nil {
			// Try upstream
			base, err = Git().UpstreamBranch()
			if err != nil {
				return fmt.Errorf("could not determine base branch; use --base to specify")
			}
		}
	}

	// Verify base branch exists
	if !Git().BranchExists(base) {
		return fmt.Errorf("base branch %q does not exist", base)
	}

	// Get current branch
	current, err := Git().CurrentBranch()
	if err != nil || current == "" {
		return fmt.Errorf("could not determine current branch (detached HEAD?)")
	}

	// Create the stack
	stack, err := Manager().Create(stackName, base)
	if err != nil {
		return err
	}

	// If current branch is not the base, add it to the stack
	if current != base {
		if err := Manager().AppendBranch(stack, current); err != nil {
			return err
		}
	}

	// Set as current stack
	if err := Manager().SetCurrent(stackName); err != nil {
		return err
	}

	ui.Success("Initialized stack %q", stackName)
	fmt.Println()
	fmt.Printf("  Base: %s\n", base)
	if current != base {
		fmt.Printf("  Branch: %s\n", current)
	}
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  stk branch <name>  Create a new branch in the stack")
	fmt.Println("  stk status         Show stack status")

	return nil
}
