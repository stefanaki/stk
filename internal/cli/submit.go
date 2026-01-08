package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/stefanaki/stk/internal/pr"
	"github.com/stefanaki/stk/internal/stack"
	"github.com/stefanaki/stk/internal/ui"
)

var submitCmd = &cobra.Command{
	Use:   "submit",
	Short: "Push branches and create/update PRs",
	Long: `Push all stack branches to the remote and manage PRs.

This command performs the following steps:
  1. Check if base branch is synced with remote
  2. Push all branches to origin (with --force-with-lease)
  3. Create PRs for branches that don't have one
  4. Update PR descriptions with current stack info

Use --no-create-prs to skip creating new PRs.
Use --no-update-prs to skip updating PR descriptions.
Use --draft to create new PRs as drafts.

Examples:
  stk submit                  # Push and manage all PRs
  stk submit --draft          # Create new PRs as drafts
  stk submit --no-create-prs  # Push only, don't create PRs
  stk submit --no-update-prs  # Don't update existing PRs`,
	RunE: runSubmit,
}

var (
	submitNoCreatePRs bool
	submitNoUpdatePRs bool
	submitDraft       bool
	submitReviewers   []string
	submitTitle       string
	submitForce       bool
)

func init() {
	submitCmd.Flags().BoolVar(&submitNoCreatePRs, "no-create-prs", false, "don't create new PRs")
	submitCmd.Flags().BoolVar(&submitNoUpdatePRs, "no-update-prs", false, "don't update existing PR descriptions")
	submitCmd.Flags().BoolVar(&submitDraft, "draft", false, "create new PRs as drafts")
	submitCmd.Flags().StringSliceVar(&submitReviewers, "reviewer", nil, "add reviewers to new PRs")
	submitCmd.Flags().StringVarP(&submitTitle, "title", "t", "", "title for new PRs (uses branch name if not specified)")
	submitCmd.Flags().BoolVar(&submitForce, "force", false, "skip the 'not synced' warning")
	rootCmd.AddCommand(submitCmd)
}

func runSubmit(cmd *cobra.Command, args []string) error {
	stk := RequireStack()
	RequireCleanTree()

	if len(stk.Branches) == 0 {
		ui.Info("Stack has no branches to submit")
		return nil
	}

	// Step 1: Check if base branch is synced
	if !submitForce {
		if err := checkBaseSynced(stk); err != nil {
			return err
		}
	}

	// Step 2: Push all branches
	fmt.Println(ui.IconArrow + " Pushing branches to origin...")
	for _, branch := range stk.Branches {
		fmt.Printf("  Pushing %s...\n", branch.Name)
		if err := Git().Push("origin", branch.Name, true); err != nil {
			return fmt.Errorf("failed to push %s: %w", branch.Name, err)
		}
	}

	// Get provider for PR operations
	provider, err := getProvider()
	if err != nil {
		if !submitNoCreatePRs || !submitNoUpdatePRs {
			ui.Warning("Failed to get PR provider: %v", err)
			ui.Info("Branches pushed, but PR operations skipped")
			return nil
		}
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

	// Step 3: Create PRs for branches without one
	if !submitNoCreatePRs && provider != nil {
		fmt.Println()
		fmt.Println(ui.IconArrow + " Creating PRs...")

		created := false
		for i, branch := range stk.Branches {
			// Skip if PR already exists
			if branch.PR != nil && branch.PR.Number > 0 {
				continue
			}

			// Check if there's already an open PR for this branch on remote
			existingPR, err := provider.GetByBranch(branch.Name)
			if err == nil && existingPR != nil {
				fmt.Printf("  Found existing PR #%d for %s\n", existingPR.Number, branch.Name)
				_ = Manager().UpdatePR(stk, branch.Name, &stack.PR{
					Number: existingPR.Number,
					URL:    existingPR.URL,
					State:  existingPR.State,
					Title:  existingPR.Title,
				})
				branchInfos[i].PR = existingPR
				continue
			}

			// Determine base branch
			var base string
			idx := stk.FindBranch(branch.Name)
			if idx == 0 {
				base = stk.Base
			} else {
				base = stk.Branches[idx-1].Name
			}

			// Determine title
			title := submitTitle
			if title == "" {
				title = branch.Name
			}

			// Generate body with stack section
			body := pr.GenerateStackSection(stk.Name, branchInfos, branch.Name)

			fmt.Printf("  Creating PR for %s → %s...\n", branch.Name, base)

			newPR, err := provider.Create(pr.CreateOptions{
				Title:     title,
				Body:      body,
				Head:      branch.Name,
				Base:      base,
				Draft:     submitDraft,
				Reviewers: submitReviewers,
			})
			if err != nil {
				ui.Warning("Failed to create PR for %s: %v", branch.Name, err)
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
			created = true

			ui.Success("Created PR #%d: %s", newPR.Number, newPR.URL)
		}

		if !created {
			fmt.Println("  No new PRs to create")
		}
	}

	// Step 4: Update existing PR descriptions
	if !submitNoUpdatePRs && provider != nil {
		// Reload stack to get updated PR info
		stk, _ = Manager().Current()

		hasPRs := false
		for _, branch := range stk.Branches {
			if branch.PR != nil && branch.PR.Number > 0 {
				hasPRs = true
				break
			}
		}

		if hasPRs {
			fmt.Println()
			fmt.Println(ui.IconArrow + " Updating PR descriptions...")

			// Refresh branch infos
			branchInfos = collectBranchInfos(stk, provider, false)

			for _, branch := range stk.Branches {
				if branch.PR == nil || branch.PR.Number == 0 {
					continue
				}
				if branch.PR.State == "merged" || branch.PR.State == "closed" {
					continue
				}

				body := pr.GenerateStackSection(stk.Name, branchInfos, branch.Name)
				fmt.Printf("  Updating PR #%d (%s)...\n", branch.PR.Number, branch.Name)
				if err := provider.Update(branch.PR.Number, pr.UpdateOptions{Body: &body}); err != nil {
					ui.Warning("Failed to update PR #%d: %v", branch.PR.Number, err)
				}
			}
		}
	}

	fmt.Println()
	ui.Success("Submit complete")
	return nil
}

// checkBaseSynced verifies the base branch is up to date with remote.
func checkBaseSynced(stk *stack.Stack) error {
	// Check if remote branch exists
	if !Git().RemoteBranchExists("origin", stk.Base) {
		return nil // No remote to compare against
	}

	localSHA, err := Git().SHA(stk.Base)
	if err != nil {
		return nil // Can't check, proceed anyway
	}

	remoteSHA, err := Git().SHA("origin/" + stk.Base)
	if err != nil {
		return nil // Can't check, proceed anyway
	}

	if localSHA == remoteSHA {
		return nil // In sync
	}

	// Check if local is behind
	if Git().IsAncestor(localSHA, remoteSHA) {
		// Count how many commits behind
		count, _ := Git().CommitCount(localSHA, remoteSHA)
		return fmt.Errorf("base branch %s is %d commit(s) behind origin; run 'stk sync' first (use --force to submit anyway)", stk.Base, count)
	}

	return nil
}
