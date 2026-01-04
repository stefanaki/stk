package git

import "fmt"

// Checkout switches to a branch.
func (g *Git) Checkout(branch string) error {
	return g.Run("checkout", branch)
}

// CheckoutSilent switches to a branch without output.
func (g *Git) CheckoutSilent(branch string) error {
	return g.RunSilent("checkout", branch)
}

// CreateBranch creates a new branch at the current HEAD.
func (g *Git) CreateBranch(name string) error {
	return g.Run("branch", name)
}

// CreateAndCheckout creates and checks out a new branch.
func (g *Git) CreateAndCheckout(name string) error {
	return g.Run("checkout", "-b", name)
}

// DeleteBranch deletes a branch.
func (g *Git) DeleteBranch(name string, force bool) error {
	flag := "-d"
	if force {
		flag = "-D"
	}
	return g.Run("branch", flag, name)
}

// RenameBranch renames a branch.
func (g *Git) RenameBranch(oldName, newName string) error {
	return g.Run("branch", "-m", oldName, newName)
}

// ResetHard resets the current branch to a ref.
func (g *Git) ResetHard(ref string) error {
	return g.Run("reset", "--hard", ref)
}

// ResetHardSilent resets without output.
func (g *Git) ResetHardSilent(ref string) error {
	return g.RunSilent("reset", "--hard", ref)
}

// ResetBranchToSHA checks out a branch and resets it to a SHA.
func (g *Git) ResetBranchToSHA(branch, sha string) error {
	if err := g.CheckoutSilent(branch); err != nil {
		return fmt.Errorf("failed to checkout %s: %w", branch, err)
	}
	if err := g.ResetHardSilent(sha); err != nil {
		return fmt.Errorf("failed to reset %s to %s: %w", branch, sha, err)
	}
	return nil
}

// SetUpstream sets the upstream branch for a local branch.
func (g *Git) SetUpstream(branch, upstream string) error {
	return g.RunSilent("branch", "--set-upstream-to="+upstream, branch)
}
