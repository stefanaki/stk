package stack

import (
	"fmt"
	"time"
)

// Manager provides high-level operations on stacks.
type Manager struct {
	storage *Storage
}

// NewManager creates a new stack manager.
func NewManager(gitDir string) *Manager {
	return &Manager{
		storage: NewStorage(gitDir),
	}
}

// Storage returns the underlying storage.
func (m *Manager) Storage() *Storage {
	return m.storage
}

// Create creates and saves a new stack.
func (m *Manager) Create(name, base string) (*Stack, error) {
	if m.storage.Exists(name) {
		return nil, fmt.Errorf("stack %q already exists", name)
	}

	stack := NewStack(name, base)
	if err := m.storage.Save(stack); err != nil {
		return nil, err
	}

	// Set as current if no current stack
	current, _ := m.storage.GetCurrent()
	if current == "" {
		_ = m.storage.SetCurrent(name)
	}

	return stack, nil
}

// Load loads a stack by name.
func (m *Manager) Load(name string) (*Stack, error) {
	return m.storage.Load(name)
}

// Current loads the current active stack.
func (m *Manager) Current() (*Stack, error) {
	return m.storage.LoadCurrent()
}

// SetCurrent sets the current active stack.
func (m *Manager) SetCurrent(name string) error {
	return m.storage.SetCurrent(name)
}

// List returns all stack names.
func (m *Manager) List() ([]string, error) {
	return m.storage.List()
}

// Delete deletes a stack.
func (m *Manager) Delete(name string) error {
	return m.storage.Delete(name)
}

// Rename renames a stack.
func (m *Manager) Rename(oldName, newName string) error {
	return m.storage.Rename(oldName, newName)
}

// AddBranch adds a branch to a stack after the specified branch.
// If afterBranch is empty, adds at the end.
func (m *Manager) AddBranch(stack *Stack, branchName, afterBranch string) error {
	if stack.HasBranch(branchName) {
		return fmt.Errorf("branch %q already in stack", branchName)
	}

	branch := NewBranch(branchName)

	if afterBranch == "" || afterBranch == stack.Base {
		// Insert at beginning
		if len(stack.Branches) == 0 {
			stack.Branches = []Branch{branch}
		} else {
			// Find where to insert
			idx := stack.FindBranch(afterBranch)
			if idx < 0 {
				// afterBranch not found, append at end
				stack.Branches = append(stack.Branches, branch)
			} else {
				// Insert after idx
				stack.Branches = append(stack.Branches[:idx+1], append([]Branch{branch}, stack.Branches[idx+1:]...)...)
			}
		}
	} else {
		idx := stack.FindBranch(afterBranch)
		if idx < 0 {
			return fmt.Errorf("branch %q not found in stack", afterBranch)
		}
		// Insert after idx
		newBranches := make([]Branch, 0, len(stack.Branches)+1)
		newBranches = append(newBranches, stack.Branches[:idx+1]...)
		newBranches = append(newBranches, branch)
		newBranches = append(newBranches, stack.Branches[idx+1:]...)
		stack.Branches = newBranches
	}

	stack.Updated = time.Now()
	return m.storage.Save(stack)
}

// AppendBranch adds a branch at the end of the stack.
func (m *Manager) AppendBranch(stack *Stack, branchName string) error {
	if stack.HasBranch(branchName) {
		return fmt.Errorf("branch %q already in stack", branchName)
	}

	stack.Branches = append(stack.Branches, NewBranch(branchName))
	stack.Updated = time.Now()
	return m.storage.Save(stack)
}

// RemoveBranch removes a branch from the stack.
func (m *Manager) RemoveBranch(stack *Stack, branchName string) error {
	idx := stack.FindBranch(branchName)
	if idx < 0 {
		return fmt.Errorf("branch %q not found in stack", branchName)
	}

	stack.Branches = append(stack.Branches[:idx], stack.Branches[idx+1:]...)
	stack.Updated = time.Now()
	return m.storage.Save(stack)
}

// MoveBranch moves a branch to a new position after the specified branch.
func (m *Manager) MoveBranch(stack *Stack, branchName, afterBranch string) error {
	idx := stack.FindBranch(branchName)
	if idx < 0 {
		return fmt.Errorf("branch %q not found in stack", branchName)
	}

	// Remove the branch
	branch := stack.Branches[idx]
	stack.Branches = append(stack.Branches[:idx], stack.Branches[idx+1:]...)

	// Find new position
	if afterBranch == "" || afterBranch == stack.Base {
		// Insert at beginning
		stack.Branches = append([]Branch{branch}, stack.Branches...)
	} else {
		newIdx := stack.FindBranch(afterBranch)
		if newIdx < 0 {
			return fmt.Errorf("branch %q not found in stack", afterBranch)
		}
		// Insert after newIdx
		newBranches := make([]Branch, 0, len(stack.Branches)+1)
		newBranches = append(newBranches, stack.Branches[:newIdx+1]...)
		newBranches = append(newBranches, branch)
		newBranches = append(newBranches, stack.Branches[newIdx+1:]...)
		stack.Branches = newBranches
	}

	stack.Updated = time.Now()
	return m.storage.Save(stack)
}

// TakeSnapshot saves the current SHA of all branches for rollback.
func (m *Manager) TakeSnapshot(stack *Stack, getSHA func(string) (string, error)) error {
	refs := make(map[string]string)

	// Save base branch SHA
	sha, err := getSHA(stack.Base)
	if err != nil {
		return fmt.Errorf("failed to get SHA for %s: %w", stack.Base, err)
	}
	refs[stack.Base] = sha

	// Save all branch SHAs
	for _, b := range stack.Branches {
		sha, err := getSHA(b.Name)
		if err != nil {
			return fmt.Errorf("failed to get SHA for %s: %w", b.Name, err)
		}
		refs[b.Name] = sha
	}

	stack.Snapshot = &Snapshot{
		TakenAt: time.Now(),
		Refs:    refs,
	}

	return m.storage.Save(stack)
}

// ClearSnapshot removes the snapshot from a stack.
func (m *Manager) ClearSnapshot(stack *Stack) error {
	stack.Snapshot = nil
	return m.storage.Save(stack)
}

// UpdatePR updates PR metadata for a branch.
func (m *Manager) UpdatePR(stack *Stack, branchName string, pr *PR) error {
	idx := stack.FindBranch(branchName)
	if idx < 0 {
		return fmt.Errorf("branch %q not found in stack", branchName)
	}

	stack.Branches[idx].PR = pr
	stack.Updated = time.Now()
	return m.storage.Save(stack)
}

// Validate checks the stack for common issues.
func (m *Manager) Validate(stack *Stack, branchExists func(string) bool) []ValidationError {
	var errors []ValidationError

	// Check base exists
	if !branchExists(stack.Base) {
		errors = append(errors, ValidationError{
			Branch:  stack.Base,
			Message: "base branch does not exist",
		})
	}

	// Check all branches exist
	for _, b := range stack.Branches {
		if !branchExists(b.Name) {
			errors = append(errors, ValidationError{
				Branch:  b.Name,
				Message: "branch does not exist",
			})
		}
	}

	// Check for duplicates
	seen := make(map[string]bool)
	for _, b := range stack.Branches {
		if seen[b.Name] {
			errors = append(errors, ValidationError{
				Branch:  b.Name,
				Message: "duplicate branch in stack",
			})
		}
		seen[b.Name] = true
	}

	return errors
}
