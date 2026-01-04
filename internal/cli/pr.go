package cli

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"github.com/gstefan/stk/internal/pr"
	"github.com/gstefan/stk/internal/stack"
	"github.com/gstefan/stk/internal/ui"
)

var prCmd = &cobra.Command{
	Use:   "pr",
	Short: "Pull request operations",
	Long:  `Commands for creating and managing pull requests.`,
}

func init() {
	rootCmd.AddCommand(prCmd)
}

// getProvider returns the configured PR provider for the current repo.
func getProvider() (pr.Provider, error) {
	remoteURL, err := Git().Remote("origin")
	if err != nil {
		return nil, fmt.Errorf("failed to get remote URL: %w", err)
	}

	provider, err := pr.DetectProvider(remoteURL)
	if err != nil {
		return nil, err
	}

	// Set up provider with repo info
	switch p := provider.(type) {
	case *pr.GitHubProvider:
		if err := p.SetRepo(remoteURL); err != nil {
			return nil, err
		}
	}

	return provider, nil
}

// collectBranchInfos gathers PR info for all branches in the stack.
func collectBranchInfos(stk *stack.Stack, provider pr.Provider, refresh bool) []pr.PRBranchInfo {
	var branchInfos []pr.PRBranchInfo
	for _, b := range stk.Branches {
		info := pr.PRBranchInfo{Name: b.Name}

		// If we have cached PR info
		if b.PR != nil {
			if refresh {
				// Refresh from remote
				remotePR, err := provider.Get(b.PR.Number)
				if err == nil && remotePR != nil {
					info.PR = remotePR
					// Update local cache
					_ = Manager().UpdatePR(stk, b.Name, &stack.PR{
						Number: remotePR.Number,
						URL:    remotePR.URL,
						State:  remotePR.State,
						Title:  remotePR.Title,
					})
				} else {
					info.PR = &pr.PR{
						Number: b.PR.Number,
						State:  b.PR.State,
					}
				}
			} else {
				info.PR = &pr.PR{
					Number: b.PR.Number,
					State:  b.PR.State,
				}
			}
		}
		branchInfos = append(branchInfos, info)
	}
	return branchInfos
}

// UpdateAllPRDescriptions updates the description of all PRs in the stack with current stack info.
func UpdateAllPRDescriptions(stk *stack.Stack, provider pr.Provider) error {
	branchInfos := collectBranchInfos(stk, provider, true)

	for _, branch := range stk.Branches {
		if branch.PR == nil || branch.PR.Number == 0 {
			continue
		}

		// Generate new body with updated stack section
		body := pr.GenerateStackSection(stk.Name, branchInfos, branch.Name)

		fmt.Printf("  Updating PR #%d (%s)...\n", branch.PR.Number, branch.Name)
		if err := provider.Update(branch.PR.Number, pr.UpdateOptions{Body: &body}); err != nil {
			ui.Warning("Failed to update PR #%d: %v", branch.PR.Number, err)
		}
	}

	return nil
}

var prCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create PRs for the stack",
	Long: `Create pull requests for all branches in the stack.

Each branch gets a PR targeting its parent branch:
  - First branch targets the base branch
  - Subsequent branches target their parent in the stack

The PR description includes a "Stack" section showing all related PRs.

Examples:
  stk pr create              # Create PRs for all branches
  stk pr create --draft      # Create as drafts
  stk pr create feature-api  # Create PR for specific branch only`,
	RunE: runPRCreate,
}

var (
	prCreateDraft     bool
	prCreateReviewers []string
	prCreateTitle     string
)

func init() {
	prCreateCmd.Flags().BoolVar(&prCreateDraft, "draft", false, "create PRs as drafts")
	prCreateCmd.Flags().StringSliceVar(&prCreateReviewers, "reviewer", nil, "add reviewers")
	prCreateCmd.Flags().StringVarP(&prCreateTitle, "title", "t", "", "PR title (uses branch name if not specified)")
	prCmd.AddCommand(prCreateCmd)
}

func runPRCreate(cmd *cobra.Command, args []string) error {
	stk := RequireStack()

	// Get remote URL to detect provider
	remoteURL, err := Git().Remote("origin")
	if err != nil {
		return fmt.Errorf("failed to get remote URL: %w", err)
	}

	provider, err := pr.DetectProvider(remoteURL)
	if err != nil {
		return err
	}

	// Set up provider with repo info
	switch p := provider.(type) {
	case *pr.GitHubProvider:
		if err := p.SetRepo(remoteURL); err != nil {
			return err
		}
	}

	fmt.Printf("Using %s provider\n\n", provider.Name())

	// Determine which branches to create PRs for
	var branches []stack.Branch
	if len(args) > 0 {
		idx := stk.FindBranch(args[0])
		if idx < 0 {
			return fmt.Errorf("branch %q not in stack", args[0])
		}
		branches = []stack.Branch{stk.Branches[idx]}
	} else {
		branches = stk.Branches
	}

	// Collect branch info for stack section
	var branchInfos []pr.PRBranchInfo
	for _, b := range stk.Branches {
		info := pr.PRBranchInfo{Name: b.Name}
		if b.PR != nil {
			info.PR = &pr.PR{
				Number: b.PR.Number,
				State:  b.PR.State,
			}
		}
		branchInfos = append(branchInfos, info)
	}

	// Create PRs
	for i, branch := range branches {
		// Determine base branch
		var base string
		idx := stk.FindBranch(branch.Name)
		if idx == 0 {
			base = stk.Base
		} else {
			base = stk.Branches[idx-1].Name
		}

		// Check if PR already exists
		if branch.PR != nil && branch.PR.Number > 0 {
			fmt.Printf("%s Skipping %s - PR #%d already exists\n",
				ui.IconInfo, branch.Name, branch.PR.Number)
			continue
		}

		// Check if there's already an open PR for this branch
		existingPR, err := provider.GetByBranch(branch.Name)
		if err == nil && existingPR != nil {
			fmt.Printf("%s Found existing PR #%d for %s\n",
				ui.IconInfo, existingPR.Number, branch.Name)

			// Update stack metadata
			_ = Manager().UpdatePR(stk, branch.Name, &stack.PR{
				Number: existingPR.Number,
				URL:    existingPR.URL,
				State:  existingPR.State,
				Title:  existingPR.Title,
			})
			continue
		}

		// Determine title
		title := prCreateTitle
		if title == "" {
			title = branch.Name
		}

		// Generate body with stack section
		body := pr.GenerateStackSection(stk.Name, branchInfos, branch.Name)

		fmt.Printf("%s Creating PR for %s → %s\n", ui.IconArrow, branch.Name, base)

		// Push branch first to ensure it exists on remote
		if err := Git().Push("origin", branch.Name, true); err != nil {
			ui.Warning("Failed to push %s: %v", branch.Name, err)
			continue
		}

		// Create the PR
		newPR, err := provider.Create(pr.CreateOptions{
			Title:     title,
			Body:      body,
			Head:      branch.Name,
			Base:      base,
			Draft:     prCreateDraft,
			Reviewers: prCreateReviewers,
		})
		if err != nil {
			ui.Error("Failed to create PR for %s: %v", branch.Name, err)
			continue
		}

		// Update stack metadata
		_ = Manager().UpdatePR(stk, branch.Name, &stack.PR{
			Number: newPR.Number,
			URL:    newPR.URL,
			State:  newPR.State,
			Title:  newPR.Title,
		})

		// Update branchInfos for subsequent PRs
		branchInfos[i].PR = newPR

		ui.Success("Created PR #%d: %s", newPR.Number, newPR.URL)
	}

	fmt.Println()
	ui.Success("PR creation complete")
	return nil
}

var prViewCmd = &cobra.Command{
	Use:   "view [branch]",
	Short: "Open PR in browser",
	Long: `Open the pull request for a branch in your browser.

Without arguments, opens the PR for the current branch.`,
	RunE: runPRView,
}

func init() {
	prCmd.AddCommand(prViewCmd)
}

func runPRView(cmd *cobra.Command, args []string) error {
	stk := RequireStack()

	var branchName string
	if len(args) > 0 {
		branchName = args[0]
	} else {
		var err error
		branchName, err = Git().CurrentBranch()
		if err != nil {
			return err
		}
	}

	idx := stk.FindBranch(branchName)
	if idx < 0 {
		return fmt.Errorf("branch %q not in stack", branchName)
	}

	branch := stk.Branches[idx]
	if branch.PR == nil || branch.PR.URL == "" {
		return fmt.Errorf("no PR found for %s; run 'stk pr create' first", branchName)
	}

	fmt.Printf("Opening %s\n", branch.PR.URL)
	return openBrowser(branch.PR.URL)
}

func openBrowser(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "linux":
		cmd = "xdg-open"
	case "darwin":
		cmd = "open"
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start"}
	default:
		return fmt.Errorf("unsupported platform")
	}

	args = append(args, url)
	return exec.Command(cmd, args...).Start()
}

// ============================================================================
// pr status - Show PR status for all branches
// ============================================================================

var prStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show PR status for all branches",
	Long: `Display the status of all pull requests in the stack.

Shows PR numbers, states, and URLs for each branch.`,
	Aliases: []string{"st"},
	RunE:    runPRStatus,
}

var prStatusRefresh bool

func init() {
	prStatusCmd.Flags().BoolVar(&prStatusRefresh, "refresh", false, "refresh PR status from remote")
	prCmd.AddCommand(prStatusCmd)
}

func runPRStatus(cmd *cobra.Command, args []string) error {
	stk := RequireStack()

	provider, err := getProvider()
	if err != nil {
		return err
	}

	fmt.Printf("%s Stack: %s%s%s\n\n", ui.IconStack, ui.Bold, stk.Name, ui.Reset)

	// Table header
	fmt.Printf("%-30s %-8s %-12s %s\n", "BRANCH", "PR", "STATE", "URL")
	fmt.Println(strings.Repeat("-", 80))

	for _, branch := range stk.Branches {
		prNum := "-"
		state := "none"
		url := "-"

		if branch.PR != nil && branch.PR.Number > 0 {
			// Optionally refresh from remote
			if prStatusRefresh {
				remotePR, err := provider.Get(branch.PR.Number)
				if err == nil && remotePR != nil {
					// Update local cache
					_ = Manager().UpdatePR(stk, branch.Name, &stack.PR{
						Number: remotePR.Number,
						URL:    remotePR.URL,
						State:  remotePR.State,
						Title:  remotePR.Title,
					})
					prNum = fmt.Sprintf("#%d", remotePR.Number)
					state = remotePR.State
					url = remotePR.URL
				}
			} else {
				prNum = fmt.Sprintf("#%d", branch.PR.Number)
				state = branch.PR.State
				if branch.PR.URL != "" {
					url = branch.PR.URL
				}
			}
		}

		// Color state
		stateColored := state
		switch state {
		case "open":
			stateColored = ui.Green + state + ui.Reset
		case "merged":
			stateColored = ui.Magenta + state + ui.Reset
		case "closed":
			stateColored = ui.Red + state + ui.Reset
		case "draft":
			stateColored = ui.Dim + state + ui.Reset
		}

		fmt.Printf("%-30s %-8s %-12s %s\n", branch.Name, prNum, stateColored, url)
	}

	return nil
}

// ============================================================================
// pr update - Update PR descriptions with current stack info
// ============================================================================

var prUpdateCmd = &cobra.Command{
	Use:   "update [branch]",
	Short: "Update PR descriptions with current stack info",
	Long: `Update the descriptions of all (or specific) PRs in the stack.

This updates the "Stack" section in each PR description to reflect
the current state of all PRs in the stack.

Examples:
  stk pr update              # Update all PRs
  stk pr update feature-api  # Update specific PR only`,
	RunE: runPRUpdate,
}

func init() {
	prCmd.AddCommand(prUpdateCmd)
}

func runPRUpdate(cmd *cobra.Command, args []string) error {
	stk := RequireStack()

	provider, err := getProvider()
	if err != nil {
		return err
	}

	fmt.Printf("Using %s provider\n\n", provider.Name())

	// Collect current branch info (refresh from remote)
	branchInfos := collectBranchInfos(stk, provider, true)

	// Determine which branches to update
	var branches []stack.Branch
	if len(args) > 0 {
		idx := stk.FindBranch(args[0])
		if idx < 0 {
			return fmt.Errorf("branch %q not in stack", args[0])
		}
		branches = []stack.Branch{stk.Branches[idx]}
	} else {
		branches = stk.Branches
	}

	for _, branch := range branches {
		if branch.PR == nil || branch.PR.Number == 0 {
			fmt.Printf("%s Skipping %s - no PR found\n", ui.IconInfo, branch.Name)
			continue
		}

		// Generate new body with updated stack section
		body := pr.GenerateStackSection(stk.Name, branchInfos, branch.Name)

		fmt.Printf("%s Updating PR #%d (%s)...\n", ui.IconArrow, branch.PR.Number, branch.Name)
		if err := provider.Update(branch.PR.Number, pr.UpdateOptions{Body: &body}); err != nil {
			ui.Error("Failed to update PR #%d: %v", branch.PR.Number, err)
			continue
		}
		ui.Success("Updated PR #%d", branch.PR.Number)
	}

	fmt.Println()
	ui.Success("PR update complete")
	return nil
}

// ============================================================================
// pr close - Close a PR without merging
// ============================================================================

var prCloseCmd = &cobra.Command{
	Use:   "close <branch>",
	Short: "Close a PR without merging",
	Long: `Close the pull request for a branch without merging.

The branch will remain in the stack. Use 'stk remove' to also remove
it from the stack.

Examples:
  stk pr close feature-api`,
	Args: cobra.ExactArgs(1),
	RunE: runPRClose,
}

func init() {
	prCmd.AddCommand(prCloseCmd)
}

func runPRClose(cmd *cobra.Command, args []string) error {
	stk := RequireStack()
	branchName := args[0]

	idx := stk.FindBranch(branchName)
	if idx < 0 {
		return fmt.Errorf("branch %q not in stack", branchName)
	}

	branch := stk.Branches[idx]
	if branch.PR == nil || branch.PR.Number == 0 {
		return fmt.Errorf("no PR found for %s", branchName)
	}

	provider, err := getProvider()
	if err != nil {
		return err
	}

	fmt.Printf("%s Closing PR #%d (%s)...\n", ui.IconArrow, branch.PR.Number, branchName)

	if err := provider.Close(branch.PR.Number); err != nil {
		return fmt.Errorf("failed to close PR: %w", err)
	}

	// Update local state
	_ = Manager().UpdatePR(stk, branchName, &stack.PR{
		Number: branch.PR.Number,
		URL:    branch.PR.URL,
		State:  "closed",
		Title:  branch.PR.Title,
	})

	ui.Success("Closed PR #%d", branch.PR.Number)
	return nil
}

// ============================================================================
// pr merge - Merge the bottom-most mergeable PR
// ============================================================================

var prMergeCmd = &cobra.Command{
	Use:   "merge [branch]",
	Short: "Merge a PR and update the stack",
	Long: `Merge a pull request and update the stack accordingly.

Without arguments, merges the first (bottom-most) PR that is mergeable.
With a branch name, merges that specific PR.

After merging:
  1. The merged PR's child PRs are retargeted to the new base
  2. The branch is optionally removed from the stack
  3. The remaining PRs are updated with new stack info

Examples:
  stk pr merge              # Merge first mergeable PR
  stk pr merge feature-api  # Merge specific PR
  stk pr merge --squash     # Use squash merge
  stk pr merge --delete     # Delete branch after merge`,
	RunE: runPRMerge,
}

var (
	prMergeMethod string
	prMergeDelete bool
	prMergeRemove bool
)

func init() {
	prMergeCmd.Flags().StringVar(&prMergeMethod, "method", "merge", "merge method: merge, squash, rebase")
	prMergeCmd.Flags().BoolVar(&prMergeDelete, "delete", false, "delete branch on remote after merge")
	prMergeCmd.Flags().BoolVar(&prMergeRemove, "remove", true, "remove branch from stack after merge")
	prCmd.AddCommand(prMergeCmd)
}

func runPRMerge(cmd *cobra.Command, args []string) error {
	stk := RequireStack()

	provider, err := getProvider()
	if err != nil {
		return err
	}

	ghProvider, isGH := provider.(*pr.GitHubProvider)

	// Determine which branch to merge
	var branchToMerge *stack.Branch
	var branchIdx int

	if len(args) > 0 {
		idx := stk.FindBranch(args[0])
		if idx < 0 {
			return fmt.Errorf("branch %q not in stack", args[0])
		}
		branchToMerge = &stk.Branches[idx]
		branchIdx = idx
	} else {
		// Find first branch with an open/mergeable PR
		for i := range stk.Branches {
			b := &stk.Branches[i]
			if b.PR != nil && b.PR.Number > 0 {
				// Check if it's open
				remotePR, err := provider.Get(b.PR.Number)
				if err == nil && remotePR != nil && remotePR.State == "open" {
					branchToMerge = b
					branchIdx = i
					break
				}
			}
		}
	}

	if branchToMerge == nil {
		return fmt.Errorf("no mergeable PR found in stack")
	}

	if branchToMerge.PR == nil || branchToMerge.PR.Number == 0 {
		return fmt.Errorf("no PR found for %s", branchToMerge.Name)
	}

	fmt.Printf("%s Merging PR #%d (%s)...\n", ui.IconArrow, branchToMerge.PR.Number, branchToMerge.Name)

	// Perform the merge
	if err := provider.Merge(branchToMerge.PR.Number, pr.MergeOptions{
		Method: prMergeMethod,
	}); err != nil {
		return fmt.Errorf("failed to merge PR: %w", err)
	}

	ui.Success("Merged PR #%d", branchToMerge.PR.Number)

	// Update local state
	_ = Manager().UpdatePR(stk, branchToMerge.Name, &stack.PR{
		Number: branchToMerge.PR.Number,
		URL:    branchToMerge.PR.URL,
		State:  "merged",
		Title:  branchToMerge.PR.Title,
	})

	// Delete remote branch if requested
	if prMergeDelete && isGH {
		fmt.Printf("%s Deleting remote branch %s...\n", ui.IconArrow, branchToMerge.Name)
		if err := ghProvider.DeleteBranch(branchToMerge.Name); err != nil {
			ui.Warning("Failed to delete remote branch: %v", err)
		}
	}

	// Retarget child PRs
	if branchIdx < len(stk.Branches)-1 {
		childBranch := stk.Branches[branchIdx+1]
		if childBranch.PR != nil && childBranch.PR.Number > 0 {
			// Determine new base
			var newBase string
			if branchIdx == 0 {
				newBase = stk.Base
			} else {
				newBase = stk.Branches[branchIdx-1].Name
			}

			fmt.Printf("%s Retargeting PR #%d to %s...\n", ui.IconArrow, childBranch.PR.Number, newBase)
			if err := provider.Retarget(childBranch.PR.Number, newBase); err != nil {
				ui.Warning("Failed to retarget PR #%d: %v", childBranch.PR.Number, err)
			}
		}
	}

	// Remove from stack if requested
	if prMergeRemove {
		fmt.Printf("%s Removing %s from stack...\n", ui.IconArrow, branchToMerge.Name)
		if err := Manager().RemoveBranch(stk, branchToMerge.Name); err != nil {
			ui.Warning("Failed to remove from stack: %v", err)
		}
		// Reload stack for PR updates
		stk, _ = Manager().Current()
	}

	// Update remaining PRs with new stack info
	if len(stk.Branches) > 0 {
		fmt.Printf("\n%s Updating remaining PR descriptions...\n", ui.IconArrow)
		_ = UpdateAllPRDescriptions(stk, provider)
	}

	fmt.Println()
	ui.Success("Merge complete")
	return nil
}
