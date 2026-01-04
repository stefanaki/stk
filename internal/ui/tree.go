package ui

import (
	"fmt"
	"strings"

	"github.com/gstefan/stk/internal/stack"
)

// TreeOptions configures tree rendering.
type TreeOptions struct {
	ShowSHA       bool
	ShowPR        bool
	ShowCommits   bool
	CurrentBranch string
	GetSHA        func(string) string
	GetCommits    func(base, head string) int
}

// RenderTree renders a stack as a tree.
func RenderTree(s *stack.Stack, opts TreeOptions) string {
	var sb strings.Builder

	// Header
	sb.WriteString(IconStack + " Stack: " + Bold + s.Name + Reset + "\n\n")

	// Base branch
	baseLine := renderBranchLine(s.Base, 0, false, opts)
	sb.WriteString(baseLine + "\n")

	// Stack branches
	for i, branch := range s.Branches {
		isLast := i == len(s.Branches)-1
		line := renderBranchLine(branch.Name, i+1, isLast, opts)

		// Add PR info if available
		if opts.ShowPR && branch.PR != nil {
			line += " " + PRBadge(branch.PR.Number, branch.PR.State)
		}

		sb.WriteString(line + "\n")
	}

	return sb.String()
}

func renderBranchLine(name string, depth int, isLast bool, opts TreeOptions) string {
	var sb strings.Builder

	// Indentation
	if depth > 0 {
		for i := 0; i < depth-1; i++ {
			sb.WriteString(IconPipe + "   ")
		}
		if isLast {
			sb.WriteString(IconBranchL + " ")
		} else {
			sb.WriteString(IconBranch + " ")
		}
	}

	// Branch indicator
	isCurrent := name == opts.CurrentBranch
	if isCurrent {
		sb.WriteString(IconDot + " ")
	} else {
		sb.WriteString(IconCircle + " ")
	}

	// Branch name
	sb.WriteString(BranchName(name, isCurrent))

	// SHA
	if opts.ShowSHA && opts.GetSHA != nil {
		sha := opts.GetSHA(name)
		if sha != "" {
			sb.WriteString(" " + CommitSHA(sha))
		}
	}

	// Commit count
	if opts.ShowCommits && opts.GetCommits != nil && depth > 0 {
		// This would need the parent branch name passed in
		// For now, skip this feature
	}

	return sb.String()
}

// RenderStatus renders a detailed status view.
func RenderStatus(s *stack.Stack, opts TreeOptions) string {
	var sb strings.Builder

	sb.WriteString(RenderTree(s, opts))
	sb.WriteString("\n")

	// Show additional info
	sb.WriteString(Dim + fmt.Sprintf("Base: %s", s.Base) + Reset + "\n")
	sb.WriteString(Dim + fmt.Sprintf("Branches: %d", len(s.Branches)) + Reset + "\n")

	if s.Snapshot != nil {
		sb.WriteString(Dim + fmt.Sprintf("Snapshot: %s", s.Snapshot.TakenAt.Format("2006-01-02 15:04:05")) + Reset + "\n")
	}

	return sb.String()
}

// RenderList renders a list of stacks.
func RenderList(stacks []string, current string) string {
	var sb strings.Builder

	if len(stacks) == 0 {
		sb.WriteString(Dim + "No stacks found. Run 'stk init <name>' to create one." + Reset + "\n")
		return sb.String()
	}

	for _, name := range stacks {
		if name == current {
			sb.WriteString(Green + IconDot + " " + Bold + name + Reset + " (current)\n")
		} else {
			sb.WriteString("  " + name + "\n")
		}
	}

	return sb.String()
}
