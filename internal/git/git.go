// Package git provides a wrapper around git commands.
package git

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Git provides methods for executing git commands.
type Git struct {
	// WorkDir is the working directory for git commands.
	// If empty, uses the current directory.
	WorkDir string
}

// New creates a new Git instance.
func New() *Git {
	return &Git{}
}

// NewWithWorkDir creates a new Git instance with a specific working directory.
func NewWithWorkDir(workDir string) *Git {
	return &Git{WorkDir: workDir}
}

// Run executes a git command with output to stdout/stderr.
func (g *Git) Run(args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if g.WorkDir != "" {
		cmd.Dir = g.WorkDir
	}
	return cmd.Run()
}

// RunSilent executes a git command without output.
func (g *Git) RunSilent(args ...string) error {
	cmd := exec.Command("git", args...)
	if g.WorkDir != "" {
		cmd.Dir = g.WorkDir
	}
	return cmd.Run()
}

// Output executes a git command and returns the output.
func (g *Git) Output(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	if g.WorkDir != "" {
		cmd.Dir = g.WorkDir
	}
	out, err := cmd.Output()
	return string(out), err
}

// OutputTrim executes a git command and returns trimmed output.
func (g *Git) OutputTrim(args ...string) (string, error) {
	out, err := g.Output(args...)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// OutputLines executes a git command and returns output as lines.
func (g *Git) OutputLines(args ...string) ([]string, error) {
	out, err := g.OutputTrim(args...)
	if err != nil {
		return nil, err
	}
	if out == "" {
		return []string{}, nil
	}
	return strings.Split(out, "\n"), nil
}

// GitDir returns the path to the .git directory.
func (g *Git) GitDir() (string, error) {
	return g.OutputTrim("rev-parse", "--git-dir")
}

// RepoRoot returns the root directory of the repository.
func (g *Git) RepoRoot() (string, error) {
	return g.OutputTrim("rev-parse", "--show-toplevel")
}

// IsInsideWorkTree returns true if we're inside a git work tree.
func (g *Git) IsInsideWorkTree() bool {
	out, err := g.OutputTrim("rev-parse", "--is-inside-work-tree")
	return err == nil && out == "true"
}

// IsClean returns true if the working tree is clean.
func (g *Git) IsClean() (bool, error) {
	out, err := g.Output("status", "--porcelain")
	if err != nil {
		return false, err
	}
	return len(strings.TrimSpace(out)) == 0, nil
}

// EnsureClean returns an error if the working tree is not clean.
func (g *Git) EnsureClean() error {
	clean, err := g.IsClean()
	if err != nil {
		return fmt.Errorf("failed to check working tree status: %w", err)
	}
	if !clean {
		return fmt.Errorf("working tree is not clean; commit or stash changes first")
	}
	return nil
}

// CurrentBranch returns the name of the current branch.
func (g *Git) CurrentBranch() (string, error) {
	return g.OutputTrim("branch", "--show-current")
}

// DefaultBranch attempts to determine the default branch (main/master).
func (g *Git) DefaultBranch() (string, error) {
	// Try to get from remote HEAD
	out, err := g.OutputTrim("symbolic-ref", "refs/remotes/origin/HEAD")
	if err == nil {
		return strings.TrimPrefix(out, "refs/remotes/origin/"), nil
	}

	// Fall back to checking common names
	for _, name := range []string{"main", "master"} {
		if g.BranchExists(name) {
			return name, nil
		}
	}

	return "", fmt.Errorf("could not determine default branch")
}

// UpstreamBranch returns the upstream branch for the current branch.
func (g *Git) UpstreamBranch() (string, error) {
	return g.OutputTrim("rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
}

// BranchExists checks if a branch exists.
func (g *Git) BranchExists(name string) bool {
	err := g.RunSilent("show-ref", "--verify", "--quiet", "refs/heads/"+name)
	return err == nil
}

// RemoteBranchExists checks if a remote branch exists.
func (g *Git) RemoteBranchExists(remote, branch string) bool {
	err := g.RunSilent("show-ref", "--verify", "--quiet", "refs/remotes/"+remote+"/"+branch)
	return err == nil
}

// SHA returns the commit SHA for a ref.
func (g *Git) SHA(ref string) (string, error) {
	return g.OutputTrim("rev-parse", ref)
}

// ShortSHA returns the short commit SHA for a ref.
func (g *Git) ShortSHA(ref string) (string, error) {
	return g.OutputTrim("rev-parse", "--short", ref)
}

// ListBranches returns all local branch names.
func (g *Git) ListBranches() ([]string, error) {
	out, err := g.OutputTrim("branch", "--format=%(refname:short)")
	if err != nil {
		return nil, err
	}
	if out == "" {
		return []string{}, nil
	}
	return strings.Split(out, "\n"), nil
}

// CommitCount returns the number of commits between two refs.
func (g *Git) CommitCount(base, head string) (int, error) {
	out, err := g.OutputTrim("rev-list", "--count", base+".."+head)
	if err != nil {
		return 0, err
	}
	var count int
	fmt.Sscanf(out, "%d", &count)
	return count, nil
}

// MergeBase returns the merge base of two refs.
func (g *Git) MergeBase(a, b string) (string, error) {
	return g.OutputTrim("merge-base", a, b)
}

// IsAncestor returns true if a is an ancestor of b.
func (g *Git) IsAncestor(a, b string) bool {
	err := g.RunSilent("merge-base", "--is-ancestor", a, b)
	return err == nil
}

// Remote returns the URL for a remote.
func (g *Git) Remote(name string) (string, error) {
	return g.OutputTrim("remote", "get-url", name)
}

// HasRemote checks if a remote exists.
func (g *Git) HasRemote(name string) bool {
	_, err := g.Remote(name)
	return err == nil
}
