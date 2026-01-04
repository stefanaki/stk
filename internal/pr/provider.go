// Package pr provides pull request integration with various platforms.
package pr

import (
	"fmt"
	"strings"
)

// Provider defines the interface for PR platforms.
type Provider interface {
	// Name returns the provider name (github, gitlab, etc.)
	Name() string

	// Detect checks if this provider can be used for the given remote URL.
	Detect(remoteURL string) bool

	// Create creates a new pull request.
	Create(opts CreateOptions) (*PR, error)

	// Update updates an existing pull request.
	Update(number int, opts UpdateOptions) error

	// Get retrieves a pull request by number.
	Get(number int) (*PR, error)

	// GetByBranch retrieves a pull request for a given branch.
	GetByBranch(branch string) (*PR, error)

	// Retarget changes the base branch of a PR.
	Retarget(number int, newBase string) error

	// Close closes a pull request without merging.
	Close(number int) error

	// Merge merges a pull request.
	Merge(number int, opts MergeOptions) error
}

// PR represents a pull request.
type PR struct {
	Number int
	URL    string
	State  string // open, closed, merged, draft
	Title  string
	Body   string
	Head   string // source branch
	Base   string // target branch
}

// CreateOptions contains options for creating a PR.
type CreateOptions struct {
	Title     string
	Body      string
	Head      string // source branch
	Base      string // target branch
	Draft     bool
	Reviewers []string
	Labels    []string
}

// UpdateOptions contains options for updating a PR.
type UpdateOptions struct {
	Title *string // nil means don't update
	Body  *string // nil means don't update
	State *string // nil means don't update (open, closed)
}

// MergeOptions contains options for merging a PR.
type MergeOptions struct {
	Method       string // merge, squash, rebase
	CommitTitle  string
	CommitMsg    string
	DeleteBranch bool
}

// DetectProvider detects the appropriate provider for a remote URL.
func DetectProvider(remoteURL string) (Provider, error) {
	// Try GitHub
	gh := &GitHubProvider{}
	if gh.Detect(remoteURL) {
		return gh, nil
	}

	// Try GitLab
	gl := &GitLabProvider{}
	if gl.Detect(remoteURL) {
		return gl, nil
	}

	return nil, fmt.Errorf("unsupported remote: %s", remoteURL)
}

// ParseRemoteURL extracts owner and repo from a remote URL.
func ParseRemoteURL(remoteURL string) (owner, repo string, err error) {
	// Handle SSH URLs: git@github.com:owner/repo.git
	if strings.HasPrefix(remoteURL, "git@") {
		parts := strings.SplitN(remoteURL, ":", 2)
		if len(parts) != 2 {
			return "", "", fmt.Errorf("invalid SSH URL: %s", remoteURL)
		}
		path := strings.TrimSuffix(parts[1], ".git")
		ownerRepo := strings.SplitN(path, "/", 2)
		if len(ownerRepo) != 2 {
			return "", "", fmt.Errorf("invalid SSH URL path: %s", path)
		}
		return ownerRepo[0], ownerRepo[1], nil
	}

	// Handle HTTPS URLs: https://github.com/owner/repo.git
	if strings.HasPrefix(remoteURL, "https://") || strings.HasPrefix(remoteURL, "http://") {
		url := strings.TrimPrefix(remoteURL, "https://")
		url = strings.TrimPrefix(url, "http://")
		parts := strings.Split(url, "/")
		if len(parts) < 3 {
			return "", "", fmt.Errorf("invalid HTTPS URL: %s", remoteURL)
		}
		repo := strings.TrimSuffix(parts[len(parts)-1], ".git")
		owner := parts[len(parts)-2]
		return owner, repo, nil
	}

	return "", "", fmt.Errorf("unrecognized URL format: %s", remoteURL)
}

// GenerateStackSection generates the stack info section for PR body.
func GenerateStackSection(stackName string, branches []PRBranchInfo, currentBranch string) string {
	var sb strings.Builder

	sb.WriteString("\n---\n\n")
	sb.WriteString("## 📚 Stack\n\n")
	sb.WriteString(fmt.Sprintf("This PR is part of the **%s** stack:\n\n", stackName))
	sb.WriteString("| # | Branch | PR | Status |\n")
	sb.WriteString("|---|--------|-----|--------|\n")

	for i, b := range branches {
		num := fmt.Sprintf("%d", i+1)
		branch := b.Name
		pr := "-"
		status := "📝 Pending"

		if b.PR != nil {
			pr = fmt.Sprintf("#%d", b.PR.Number)
			switch b.PR.State {
			case "merged":
				status = "✅ Merged"
			case "closed":
				status = "❌ Closed"
			case "draft":
				status = "📝 Draft"
			default:
				status = "🔄 Open"
			}
		}

		if b.Name == currentBranch {
			sb.WriteString(fmt.Sprintf("| **%s** | **`%s`** | **%s** | **🔄 This PR** |\n", num, branch, pr))
		} else {
			sb.WriteString(fmt.Sprintf("| %s | `%s` | %s | %s |\n", num, branch, pr, status))
		}
	}

	sb.WriteString("\n---\n")
	sb.WriteString("*Managed by [stk](https://github.com/stefanaki/stk)*\n")

	return sb.String()
}

// PRBranchInfo contains branch info for PR generation.
type PRBranchInfo struct {
	Name string
	PR   *PR
}
