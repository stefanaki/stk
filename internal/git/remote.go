package git

// Fetch fetches from a remote.
func (g *Git) Fetch(remote string, args ...string) error {
	cmdArgs := append([]string{"fetch", remote}, args...)
	return g.Run(cmdArgs...)
}

// FetchAll fetches from all remotes.
func (g *Git) FetchAll() error {
	return g.Run("fetch", "--all")
}

// Pull pulls from the upstream.
func (g *Git) Pull(args ...string) error {
	cmdArgs := append([]string{"pull"}, args...)
	return g.Run(cmdArgs...)
}

// Push pushes a branch to a remote.
func (g *Git) Push(remote, branch string, force bool) error {
	args := []string{"push", "-u", remote, branch}
	if force {
		args = append(args, "--force-with-lease")
	}
	return g.Run(args...)
}

// PushSilent pushes without output.
func (g *Git) PushSilent(remote, branch string, force bool) error {
	args := []string{"push", "-u", remote, branch}
	if force {
		args = append(args, "--force-with-lease")
	}
	return g.RunSilent(args...)
}

// PushDelete deletes a remote branch.
func (g *Git) PushDelete(remote, branch string) error {
	return g.Run("push", remote, "--delete", branch)
}
