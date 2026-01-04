package cli

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/gstefan/stk/internal/ui"
)

var branchCmd = &cobra.Command{
	Use:   "branch <name>",
	Short: "Create a new branch and add it to the stack",
	Long: `Create a new branch at the current HEAD and add it to the stack.

The branch is created from the current HEAD and added to the stack
after the current branch. If you're on the base branch, it becomes
the first branch in the stack.

Examples:
  stk branch feature-auth      # Create and add to stack
  stk branch feature-api       # Create next branch in sequence`,
	Aliases: []string{"br"},
	Args:    cobra.ExactArgs(1),
	RunE:    runBranch,
}

func init() {
	rootCmd.AddCommand(branchCmd)
}

func runBranch(cmd *cobra.Command, args []string) error {
	branchName := args[0]
	stack := RequireStack()

	RequireCleanTree()

	// Check if branch already exists
	if Git().BranchExists(branchName) {
		return fmt.Errorf("branch %q already exists", branchName)
	}

	// Get current branch to determine insert position
	current, err := Git().CurrentBranch()
	if err != nil {
		return fmt.Errorf("could not determine current branch: %w", err)
	}

	// Create and checkout the new branch
	if err := Git().CreateAndCheckout(branchName); err != nil {
		return fmt.Errorf("failed to create branch: %w", err)
	}

	// Add to stack after current branch
	if current == stack.Base {
		// Insert at beginning
		if err := Manager().AddBranch(stack, branchName, ""); err != nil {
			return err
		}
	} else {
		if err := Manager().AddBranch(stack, branchName, current); err != nil {
			return err
		}
	}

	ui.Success("Created branch %q", branchName)
	if current == stack.Base {
		fmt.Printf("  Added as first branch in stack\n")
	} else {
		fmt.Printf("  Added after %s\n", current)
	}

	return nil
}

var addCmd = &cobra.Command{
	Use:   "add <branch-name>",
	Short: "Add an existing branch to the stack",
	Long: `Add an existing git branch to the current stack.

By default, the branch is added at the end of the stack.
Use --after to insert it after a specific branch.

Examples:
  stk add feature-auth                    # Add at end
  stk add feature-api --after feature-auth # Add after specific branch`,
	Args: cobra.ExactArgs(1),
	RunE: runAdd,
}

var addAfter string

func init() {
	addCmd.Flags().StringVar(&addAfter, "after", "", "add after this branch")
	rootCmd.AddCommand(addCmd)
}

func runAdd(cmd *cobra.Command, args []string) error {
	branchName := args[0]
	stack := RequireStack()

	// Check branch exists
	if !Git().BranchExists(branchName) {
		return fmt.Errorf("branch %q does not exist", branchName)
	}

	// Check not already in stack
	if stack.HasBranch(branchName) {
		return fmt.Errorf("branch %q is already in the stack", branchName)
	}

	if addAfter != "" {
		if err := Manager().AddBranch(stack, branchName, addAfter); err != nil {
			return err
		}
		ui.Success("Added %q after %q", branchName, addAfter)
	} else {
		if err := Manager().AppendBranch(stack, branchName); err != nil {
			return err
		}
		ui.Success("Added %q to stack", branchName)
	}

	return nil
}

var removeCmd = &cobra.Command{
	Use:   "remove <branch-name>",
	Short: "Remove a branch from the stack",
	Long: `Remove a branch from the stack.

This only removes the branch from the stack metadata.
The git branch is NOT deleted.`,
	Args: cobra.ExactArgs(1),
	RunE: runRemove,
}

func init() {
	rootCmd.AddCommand(removeCmd)
}

func runRemove(cmd *cobra.Command, args []string) error {
	branchName := args[0]
	stack := RequireStack()

	if err := Manager().RemoveBranch(stack, branchName); err != nil {
		return err
	}

	ui.Success("Removed %q from stack", branchName)
	fmt.Println(ui.Dim + "Note: Git branch was not deleted" + ui.Reset)
	return nil
}

var moveCmd = &cobra.Command{
	Use:   "move <branch> --after <other-branch>",
	Short: "Move a branch to a new position in the stack",
	Long: `Reorder a branch within the stack.

Use --after to specify the new position.
Use --after with the base branch name to move to the beginning.`,
	Args: cobra.ExactArgs(1),
	RunE: runMove,
}

var moveAfter string

func init() {
	moveCmd.Flags().StringVar(&moveAfter, "after", "", "move after this branch (required)")
	moveCmd.MarkFlagRequired("after")
	rootCmd.AddCommand(moveCmd)
}

func runMove(cmd *cobra.Command, args []string) error {
	branchName := args[0]
	stack := RequireStack()

	if err := Manager().MoveBranch(stack, branchName, moveAfter); err != nil {
		return err
	}

	ui.Success("Moved %q after %q", branchName, moveAfter)
	return nil
}

// Navigation commands

var upCmd = &cobra.Command{
	Use:   "up",
	Short: "Checkout the parent branch",
	Long:  `Checkout the parent branch in the stack (move toward base).`,
	RunE:  runUp,
}

func init() {
	rootCmd.AddCommand(upCmd)
}

func runUp(cmd *cobra.Command, args []string) error {
	stack := RequireStack()
	RequireCleanTree()

	current, err := Git().CurrentBranch()
	if err != nil {
		return fmt.Errorf("could not determine current branch: %w", err)
	}

	parent := stack.GetParent(current)
	if parent == "" {
		return fmt.Errorf("already at base branch")
	}

	if current == stack.Base {
		return fmt.Errorf("already at base branch")
	}

	if err := Git().Checkout(parent); err != nil {
		return fmt.Errorf("failed to checkout %s: %w", parent, err)
	}

	ui.Success("Checked out %s", parent)
	return nil
}

var downCmd = &cobra.Command{
	Use:   "down",
	Short: "Checkout the child branch",
	Long:  `Checkout the first child branch in the stack (move away from base).`,
	RunE:  runDown,
}

func init() {
	rootCmd.AddCommand(downCmd)
}

func runDown(cmd *cobra.Command, args []string) error {
	stack := RequireStack()
	RequireCleanTree()

	current, err := Git().CurrentBranch()
	if err != nil {
		return fmt.Errorf("could not determine current branch: %w", err)
	}

	var child string
	if current == stack.Base {
		if len(stack.Branches) > 0 {
			child = stack.Branches[0].Name
		}
	} else {
		children := stack.GetChildren(current)
		if len(children) > 0 {
			child = children[0]
		}
	}

	if child == "" {
		return fmt.Errorf("no child branch to checkout")
	}

	if err := Git().Checkout(child); err != nil {
		return fmt.Errorf("failed to checkout %s: %w", child, err)
	}

	ui.Success("Checked out %s", child)
	return nil
}

var topCmd = &cobra.Command{
	Use:   "top",
	Short: "Checkout the base branch",
	Long:  `Checkout the base (trunk) branch of the stack.`,
	RunE:  runTop,
}

func init() {
	rootCmd.AddCommand(topCmd)
}

func runTop(cmd *cobra.Command, args []string) error {
	stack := RequireStack()
	RequireCleanTree()

	if err := Git().Checkout(stack.Base); err != nil {
		return fmt.Errorf("failed to checkout %s: %w", stack.Base, err)
	}

	ui.Success("Checked out %s (base)", stack.Base)
	return nil
}

var bottomCmd = &cobra.Command{
	Use:     "bottom",
	Short:   "Checkout the last branch in the stack",
	Long:    `Checkout the last (most derived) branch in the stack.`,
	Aliases: []string{"bot"},
	RunE:    runBottom,
}

func init() {
	rootCmd.AddCommand(bottomCmd)
}

func runBottom(cmd *cobra.Command, args []string) error {
	stack := RequireStack()
	RequireCleanTree()

	if len(stack.Branches) == 0 {
		return fmt.Errorf("stack has no branches")
	}

	last := stack.Branches[len(stack.Branches)-1].Name

	if err := Git().Checkout(last); err != nil {
		return fmt.Errorf("failed to checkout %s: %w", last, err)
	}

	ui.Success("Checked out %s (bottom)", last)
	return nil
}

var gotoCmd = &cobra.Command{
	Use:   "goto <n>",
	Short: "Checkout the nth branch in the stack",
	Long: `Checkout a branch by its position in the stack.

Position 0 is the base branch, position 1 is the first stack branch, etc.

Examples:
  stk goto 0   # Checkout base branch
  stk goto 1   # Checkout first branch in stack
  stk goto 3   # Checkout third branch in stack`,
	Aliases: []string{"go"},
	Args:    cobra.ExactArgs(1),
	RunE:    runGoto,
}

func init() {
	rootCmd.AddCommand(gotoCmd)
}

func runGoto(cmd *cobra.Command, args []string) error {
	stack := RequireStack()
	RequireCleanTree()

	n, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("invalid position: %s", args[0])
	}

	var target string
	if n == 0 {
		target = stack.Base
	} else if n > 0 && n <= len(stack.Branches) {
		target = stack.Branches[n-1].Name
	} else {
		return fmt.Errorf("position %d out of range (stack has %d branches)", n, len(stack.Branches))
	}

	if err := Git().Checkout(target); err != nil {
		return fmt.Errorf("failed to checkout %s: %w", target, err)
	}

	ui.Success("Checked out %s (position %d)", target, n)
	return nil
}

var whichCmd = &cobra.Command{
	Use:   "which",
	Short: "Show current branch's position in stack",
	Long:  `Display the current branch's position within the stack.`,
	RunE:  runWhich,
}

func init() {
	rootCmd.AddCommand(whichCmd)
}

func runWhich(cmd *cobra.Command, args []string) error {
	stack := RequireStack()

	current, err := Git().CurrentBranch()
	if err != nil {
		return fmt.Errorf("could not determine current branch: %w", err)
	}

	if current == stack.Base {
		fmt.Printf("%s (base, position 0)\n", current)
		return nil
	}

	idx := stack.FindBranch(current)
	if idx < 0 {
		fmt.Printf("%s (not in stack)\n", current)
		return nil
	}

	fmt.Printf("%s (position %d of %d)\n", current, idx+1, len(stack.Branches))
	return nil
}
