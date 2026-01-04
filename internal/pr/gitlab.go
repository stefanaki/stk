package pr

import (
	"strings"
)

// GitLabProvider implements the Provider interface for GitLab.
// This is a placeholder implementation - full implementation would require
// similar API calls as GitHub but with GitLab's API.
type GitLabProvider struct {
	Token   string
	BaseURL string
	Project string // project ID or path
}

// Name returns "gitlab".
func (g *GitLabProvider) Name() string {
	return "gitlab"
}

// Detect checks if the remote URL is a GitLab URL.
func (g *GitLabProvider) Detect(remoteURL string) bool {
	return strings.Contains(remoteURL, "gitlab.com") ||
		strings.Contains(remoteURL, "gitlab.")
}

// Create creates a new merge request on GitLab.
func (g *GitLabProvider) Create(opts CreateOptions) (*PR, error) {
	// TODO: Implement GitLab API calls
	// GitLab uses "merge requests" instead of "pull requests"
	// API endpoint: POST /projects/:id/merge_requests
	return nil, nil
}

// Get retrieves a merge request by ID.
func (g *GitLabProvider) Get(number int) (*PR, error) {
	// TODO: Implement GitLab API calls
	// API endpoint: GET /projects/:id/merge_requests/:merge_request_iid
	return nil, nil
}

// GetByBranch retrieves a merge request for a given source branch.
func (g *GitLabProvider) GetByBranch(branch string) (*PR, error) {
	// TODO: Implement GitLab API calls
	// API endpoint: GET /projects/:id/merge_requests?source_branch=:branch
	return nil, nil
}

// Retarget changes the target branch of a merge request.
func (g *GitLabProvider) Retarget(number int, newBase string) error {
	// TODO: Implement GitLab API calls
	// API endpoint: PUT /projects/:id/merge_requests/:merge_request_iid
	return nil
}

// Update updates an existing merge request.
func (g *GitLabProvider) Update(number int, opts UpdateOptions) error {
	// TODO: Implement GitLab API calls
	// API endpoint: PUT /projects/:id/merge_requests/:merge_request_iid
	return nil
}

// Close closes a merge request without merging.
func (g *GitLabProvider) Close(number int) error {
	// TODO: Implement GitLab API calls
	state := "close"
	return g.Update(number, UpdateOptions{State: &state})
}

// Merge merges a merge request.
func (g *GitLabProvider) Merge(number int, opts MergeOptions) error {
	// TODO: Implement GitLab API calls
	// API endpoint: PUT /projects/:id/merge_requests/:merge_request_iid/merge
	return nil
}
