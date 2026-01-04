package git

import "fmt"

// RebaseResult represents the outcome of a rebase operation.
type RebaseResult struct {
	Success     bool
	HasConflict bool
	Message     string
}

// Rebase rebases the current branch onto a target.
func (g *Git) Rebase(onto string) error {
	return g.Run("rebase", onto)
}

// RebaseOnto rebases using --onto syntax.
func (g *Git) RebaseOnto(onto, upstream, branch string) error {
	return g.Run("rebase", "--onto", onto, upstream, branch)
}

// RebaseInteractive starts an interactive rebase.
func (g *Git) RebaseInteractive(onto string) error {
	return g.Run("rebase", "-i", onto)
}

// RebaseAbort aborts an in-progress rebase.
func (g *Git) RebaseAbort() error {
	return g.RunSilent("rebase", "--abort")
}

// RebaseContinue continues a rebase after conflict resolution.
func (g *Git) RebaseContinue() error {
	return g.Run("rebase", "--continue")
}

// IsRebaseInProgress checks if a rebase is in progress.
func (g *Git) IsRebaseInProgress() bool {
	gitDir, err := g.GitDir()
	if err != nil {
		return false
	}
	// Check for rebase-merge or rebase-apply directories
	_, err1 := g.Output("ls", gitDir+"/rebase-merge")
	_, err2 := g.Output("ls", gitDir+"/rebase-apply")
	return err1 == nil || err2 == nil
}

// RebaseBranchOnto rebases a branch onto a new base.
// This is the main operation for stack rebasing.
func (g *Git) RebaseBranchOnto(branch, onto string) error {
	// Checkout the branch
	if err := g.Checkout(branch); err != nil {
		return fmt.Errorf("failed to checkout %s: %w", branch, err)
	}

	// Rebase onto target
	if err := g.Rebase(onto); err != nil {
		return fmt.Errorf("rebase of %s onto %s failed: %w", branch, onto, err)
	}

	return nil
}

// CherryPick cherry-picks commits.
func (g *Git) CherryPick(commits ...string) error {
	args := append([]string{"cherry-pick"}, commits...)
	return g.Run(args...)
}

// CherryPickAbort aborts a cherry-pick.
func (g *Git) CherryPickAbort() error {
	return g.RunSilent("cherry-pick", "--abort")
}
