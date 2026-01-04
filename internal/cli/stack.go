package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/stefanaki/stk/internal/ui"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current stack status",
	Long: `Display the current stack with branch information.

Shows:
  - Stack name and base branch
  - All branches in the stack
  - Current branch indicator
  - Commit SHAs (with --sha flag)
  - PR status (if available)`,
	Aliases: []string{"st"},
	RunE:    runStatus,
}

var statusShowSHA bool

func init() {
	statusCmd.Flags().BoolVar(&statusShowSHA, "sha", false, "show commit SHAs")
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	stack := RequireStack()

	current, _ := Git().CurrentBranch()

	opts := ui.TreeOptions{
		ShowSHA:       statusShowSHA,
		ShowPR:        true,
		CurrentBranch: current,
		GetSHA: func(name string) string {
			sha, _ := Git().ShortSHA(name)
			return sha
		},
	}

	fmt.Print(ui.RenderStatus(stack, opts))
	return nil
}

var listCmd = &cobra.Command{
	Use:     "list",
	Short:   "List all stacks",
	Long:    `List all stacks in the repository.`,
	Aliases: []string{"ls"},
	RunE:    runList,
}

func init() {
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) error {
	stacks, err := Manager().List()
	if err != nil {
		return err
	}

	current, _ := Manager().Storage().GetCurrent()
	fmt.Print(ui.RenderList(stacks, current))
	return nil
}

var switchCmd = &cobra.Command{
	Use:   "switch <stack-name>",
	Short: "Switch to a different stack",
	Long: `Switch the active stack to the specified stack.

This only changes which stack stk commands operate on.
It does not checkout any branches.`,
	Aliases: []string{"sw"},
	Args:    cobra.ExactArgs(1),
	RunE:    runSwitch,
}

func init() {
	rootCmd.AddCommand(switchCmd)
}

func runSwitch(cmd *cobra.Command, args []string) error {
	name := args[0]

	if !Manager().Storage().Exists(name) {
		return fmt.Errorf("stack %q not found", name)
	}

	if err := Manager().SetCurrent(name); err != nil {
		return err
	}

	ui.Success("Switched to stack %q", name)
	return nil
}

var deleteCmd = &cobra.Command{
	Use:   "delete <stack-name>",
	Short: "Delete a stack",
	Long: `Delete a stack definition.

This removes the stack metadata but does NOT delete the git branches.
Use 'git branch -d <branch>' to delete branches manually.`,
	Aliases: []string{"rm"},
	Args:    cobra.ExactArgs(1),
	RunE:    runDelete,
}

var deleteForce bool

func init() {
	deleteCmd.Flags().BoolVarP(&deleteForce, "force", "f", false, "skip confirmation")
	rootCmd.AddCommand(deleteCmd)
}

func runDelete(cmd *cobra.Command, args []string) error {
	name := args[0]

	if !Manager().Storage().Exists(name) {
		return fmt.Errorf("stack %q not found", name)
	}

	if err := Manager().Delete(name); err != nil {
		return err
	}

	ui.Success("Deleted stack %q", name)
	fmt.Println(ui.Dim + "Note: Git branches were not deleted" + ui.Reset)
	return nil
}

var renameCmd = &cobra.Command{
	Use:   "rename <old-name> <new-name>",
	Short: "Rename a stack",
	Long:  `Rename a stack to a new name.`,
	Args:  cobra.ExactArgs(2),
	RunE:  runRename,
}

func init() {
	rootCmd.AddCommand(renameCmd)
}

func runRename(cmd *cobra.Command, args []string) error {
	oldName := args[0]
	newName := args[1]

	if err := Manager().Rename(oldName, newName); err != nil {
		return err
	}

	ui.Success("Renamed stack %q to %q", oldName, newName)
	return nil
}

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Validate stack integrity",
	Long: `Check the current stack for common issues.

Validates:
  - All branches in the stack exist
  - Base branch exists
  - No duplicate branches`,
	RunE: runDoctor,
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}

func runDoctor(cmd *cobra.Command, args []string) error {
	stack := RequireStack()

	errors := Manager().Validate(stack, func(name string) bool {
		return Git().BranchExists(name)
	})

	if len(errors) == 0 {
		ui.Success("Stack %q is healthy", stack.Name)
		return nil
	}

	ui.Error("Found %d issue(s):", len(errors))
	for _, e := range errors {
		fmt.Printf("  %s: %s\n", e.Branch, e.Message)
	}

	return fmt.Errorf("stack has validation errors")
}

var logCmd = &cobra.Command{
	Use:   "log",
	Short: "Show stack as a tree",
	Long:  `Display the stack as a visual tree with branch relationships.`,
	RunE:  runLog,
}

func init() {
	rootCmd.AddCommand(logCmd)
}

func runLog(cmd *cobra.Command, args []string) error {
	stack := RequireStack()
	current, _ := Git().CurrentBranch()

	opts := ui.TreeOptions{
		ShowSHA:       true,
		ShowPR:        true,
		CurrentBranch: current,
		GetSHA: func(name string) string {
			sha, _ := Git().ShortSHA(name)
			return sha
		},
	}

	fmt.Print(ui.RenderTree(stack, opts))
	return nil
}
