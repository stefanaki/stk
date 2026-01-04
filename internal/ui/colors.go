// Package ui provides terminal UI helpers for the CLI.
package ui

import (
	"fmt"
)

// Color codes for terminal output.
const (
	Reset   = "\033[0m"
	Bold    = "\033[1m"
	Dim     = "\033[2m"
	Red     = "\033[31m"
	Green   = "\033[32m"
	Yellow  = "\033[33m"
	Blue    = "\033[34m"
	Magenta = "\033[35m"
	Cyan    = "\033[36m"
	White   = "\033[37m"
)

// Icons for status display.
const (
	IconSuccess  = "✅"
	IconError    = "❌"
	IconWarning  = "⚠️"
	IconInfo     = "ℹ️"
	IconArrow    = "▶"
	IconCheck    = "✓"
	IconCross    = "✗"
	IconBranch   = "├──"
	IconBranchL  = "└──"
	IconPipe     = "│"
	IconDot      = "●"
	IconCircle   = "○"
	IconRollback = "⏪"
	IconCamera   = "📸"
	IconStack    = "📚"
	IconPR       = "🔗"
)

// Colorize wraps text in color codes.
func Colorize(color, text string) string {
	return color + text + Reset
}

// Success prints a success message.
func Success(format string, args ...interface{}) {
	fmt.Printf(Green+IconCheck+" "+format+Reset+"\n", args...)
}

// Error prints an error message.
func Error(format string, args ...interface{}) {
	fmt.Printf(Red+IconCross+" "+format+Reset+"\n", args...)
}

// Warning prints a warning message.
func Warning(format string, args ...interface{}) {
	fmt.Printf(Yellow+IconWarning+" "+format+Reset+"\n", args...)
}

// Info prints an info message.
func Info(format string, args ...interface{}) {
	fmt.Printf(Cyan+IconInfo+" "+format+Reset+"\n", args...)
}

// Header prints a header.
func Header(format string, args ...interface{}) {
	fmt.Printf(Bold+format+Reset+"\n", args...)
}

// Dim prints dimmed text.
func DimText(format string, args ...interface{}) {
	fmt.Printf(Dim+format+Reset+"\n", args...)
}

// BranchName formats a branch name.
func BranchName(name string, current bool) string {
	if current {
		return Bold + Green + name + Reset
	}
	return name
}

// CommitSHA formats a commit SHA.
func CommitSHA(sha string) string {
	if len(sha) > 7 {
		sha = sha[:7]
	}
	return Dim + sha + Reset
}

// PRBadge formats a PR number badge.
func PRBadge(number int, state string) string {
	color := Blue
	switch state {
	case "merged":
		color = Magenta
	case "closed":
		color = Red
	case "draft":
		color = Dim
	}
	return color + fmt.Sprintf("#%d", number) + Reset
}
