// Package stack provides data structures and operations for managing stacked branches.
package stack

import "time"

// Stack represents a collection of dependent branches.
type Stack struct {
	Version  int       `yaml:"version"`
	Name     string    `yaml:"name"`
	Base     string    `yaml:"base"`
	Created  time.Time `yaml:"created"`
	Updated  time.Time `yaml:"updated"`
	Branches []Branch  `yaml:"branches"`
	Snapshot *Snapshot `yaml:"snapshot,omitempty"`
}

// Branch represents a single branch in the stack.
type Branch struct {
	Name     string `yaml:"name"`
	Upstream string `yaml:"upstream,omitempty"`
	PR       *PR    `yaml:"pr,omitempty"`
}

// PR represents pull request metadata for a branch.
type PR struct {
	Number int    `yaml:"number"`
	URL    string `yaml:"url"`
	State  string `yaml:"state"` // open, closed, merged, draft
	Title  string `yaml:"title,omitempty"`
}

// Snapshot stores branch SHAs for atomic rollback.
type Snapshot struct {
	TakenAt time.Time         `yaml:"taken_at"`
	Refs    map[string]string `yaml:"refs"` // branch name -> SHA
}

// Node represents a branch in the computed dependency graph.
type Node struct {
	Branch   *Branch
	Parent   *Node
	Children []*Node
	SHA      string
}

// Graph represents the computed dependency graph of a stack.
type Graph struct {
	Base  string
	Nodes map[string]*Node
	Order []string // topological order (base first, then branches)
}

// ValidationError represents a stack validation issue.
type ValidationError struct {
	Branch  string
	Message string
}

// NewStack creates a new stack with the given name and base branch.
func NewStack(name, base string) *Stack {
	now := time.Now()
	return &Stack{
		Version:  1,
		Name:     name,
		Base:     base,
		Created:  now,
		Updated:  now,
		Branches: []Branch{},
	}
}

// NewBranch creates a new branch entry.
func NewBranch(name string) Branch {
	return Branch{
		Name: name,
	}
}

// FindBranch returns the index of a branch by name, or -1 if not found.
func (s *Stack) FindBranch(name string) int {
	for i, b := range s.Branches {
		if b.Name == name {
			return i
		}
	}
	return -1
}

// HasBranch checks if a branch exists in the stack.
func (s *Stack) HasBranch(name string) bool {
	return s.FindBranch(name) >= 0
}

// GetParent returns the parent branch name for a given branch.
// Returns the base branch if it's the first branch in the stack.
func (s *Stack) GetParent(name string) string {
	idx := s.FindBranch(name)
	if idx <= 0 {
		return s.Base
	}
	return s.Branches[idx-1].Name
}

// GetChildren returns all branches that depend on the given branch.
func (s *Stack) GetChildren(name string) []string {
	idx := s.FindBranch(name)
	if idx < 0 || idx >= len(s.Branches)-1 {
		return nil
	}
	return []string{s.Branches[idx+1].Name}
}

// AllBranches returns base + all stack branches in order.
func (s *Stack) AllBranches() []string {
	result := make([]string, 0, len(s.Branches)+1)
	result = append(result, s.Base)
	for _, b := range s.Branches {
		result = append(result, b.Name)
	}
	return result
}

// BuildGraph constructs a dependency graph from the stack.
func (s *Stack) BuildGraph() *Graph {
	g := &Graph{
		Base:  s.Base,
		Nodes: make(map[string]*Node),
		Order: s.AllBranches(),
	}

	// Create base node
	baseNode := &Node{
		Branch: &Branch{Name: s.Base},
	}
	g.Nodes[s.Base] = baseNode

	// Create branch nodes with parent links
	var prevNode *Node = baseNode
	for i := range s.Branches {
		node := &Node{
			Branch: &s.Branches[i],
			Parent: prevNode,
		}
		prevNode.Children = append(prevNode.Children, node)
		g.Nodes[s.Branches[i].Name] = node
		prevNode = node
	}

	return g
}
