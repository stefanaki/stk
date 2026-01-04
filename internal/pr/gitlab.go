package pr

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
)

// GitLabProvider implements the Provider interface for GitLab.
type GitLabProvider struct {
	Token   string
	BaseURL string // e.g., "https://gitlab.com" or self-hosted instance
	Project string // URL-encoded project path (e.g., "owner%2Frepo")
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

// SetRepo sets the project path and base URL from a remote URL.
func (g *GitLabProvider) SetRepo(remoteURL string) error {
	// Parse SSH URL: git@gitlab.com:owner/repo.git
	if strings.HasPrefix(remoteURL, "git@") {
		parts := strings.SplitN(remoteURL, ":", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid SSH URL: %s", remoteURL)
		}
		host := strings.TrimPrefix(parts[0], "git@")
		path := strings.TrimSuffix(parts[1], ".git")
		g.BaseURL = "https://" + host
		g.Project = url.PathEscape(path)
		return nil
	}

	// Parse HTTPS URL: https://gitlab.com/owner/repo.git
	if strings.HasPrefix(remoteURL, "https://") || strings.HasPrefix(remoteURL, "http://") {
		u, err := url.Parse(remoteURL)
		if err != nil {
			return fmt.Errorf("invalid URL: %s", remoteURL)
		}
		g.BaseURL = u.Scheme + "://" + u.Host
		path := strings.TrimPrefix(u.Path, "/")
		path = strings.TrimSuffix(path, ".git")
		g.Project = url.PathEscape(path)
		return nil
	}

	return fmt.Errorf("unrecognized URL format: %s", remoteURL)
}

// getToken retrieves the GitLab token from environment or glab CLI.
func (g *GitLabProvider) getToken() (string, error) {
	if g.Token != "" {
		return g.Token, nil
	}

	// Check environment variable
	if token := os.Getenv("GITLAB_TOKEN"); token != "" {
		g.Token = token
		return token, nil
	}

	// Also check GITLAB_PRIVATE_TOKEN (common alternative)
	if token := os.Getenv("GITLAB_PRIVATE_TOKEN"); token != "" {
		g.Token = token
		return token, nil
	}

	// Try glab CLI (GitLab CLI tool)
	cmd := exec.Command("glab", "auth", "token")
	out, err := cmd.Output()
	if err == nil {
		g.Token = strings.TrimSpace(string(out))
		return g.Token, nil
	}

	return "", fmt.Errorf("no GitLab token found; set GITLAB_TOKEN or login with 'glab auth login'")
}

// getBaseURL returns the base URL for the GitLab API.
func (g *GitLabProvider) getBaseURL() string {
	if g.BaseURL == "" {
		return "https://gitlab.com"
	}
	return g.BaseURL
}

// Create creates a new merge request on GitLab.
func (g *GitLabProvider) Create(opts CreateOptions) (*PR, error) {
	token, err := g.getToken()
	if err != nil {
		return nil, err
	}

	// Build request body
	body := map[string]interface{}{
		"title":         opts.Title,
		"source_branch": opts.Head,
		"target_branch": opts.Base,
		"description":   opts.Body,
	}

	// GitLab doesn't have draft as a simple boolean, but uses WIP prefix or draft flag
	if opts.Draft {
		body["title"] = "Draft: " + opts.Title
	}

	// Add reviewers if specified (GitLab uses reviewer_ids, which requires user IDs)
	// For simplicity, we'll skip reviewers as it requires additional API calls to resolve usernames to IDs

	// Add labels if specified
	if len(opts.Labels) > 0 {
		body["labels"] = strings.Join(opts.Labels, ",")
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create request
	apiURL := fmt.Sprintf("%s/api/v4/projects/%s/merge_requests", g.getBaseURL(), g.Project)
	req, err := http.NewRequest("POST", apiURL, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("PRIVATE-TOKEN", token)
	req.Header.Set("Content-Type", "application/json")

	// Send request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 201 {
		return nil, fmt.Errorf("GitLab API error: %s - %s", resp.Status, string(respBody))
	}

	// Parse response
	var result struct {
		IID          int    `json:"iid"`
		WebURL       string `json:"web_url"`
		State        string `json:"state"` // opened, closed, merged
		Title        string `json:"title"`
		SourceBranch string `json:"source_branch"`
		TargetBranch string `json:"target_branch"`
		Draft        bool   `json:"draft"`
		WorkInProg   bool   `json:"work_in_progress"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	state := g.mapState(result.State, result.Draft || result.WorkInProg)

	return &PR{
		Number: result.IID,
		URL:    result.WebURL,
		State:  state,
		Title:  result.Title,
		Head:   result.SourceBranch,
		Base:   result.TargetBranch,
	}, nil
}

// mapState converts GitLab state to unified state.
func (g *GitLabProvider) mapState(state string, isDraft bool) string {
	switch state {
	case "merged":
		return "merged"
	case "closed":
		return "closed"
	case "opened":
		if isDraft {
			return "draft"
		}
		return "open"
	default:
		return state
	}
}

// Get retrieves a merge request by IID (internal ID).
func (g *GitLabProvider) Get(number int) (*PR, error) {
	token, err := g.getToken()
	if err != nil {
		return nil, err
	}

	apiURL := fmt.Sprintf("%s/api/v4/projects/%s/merge_requests/%d", g.getBaseURL(), g.Project, number)
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("PRIVATE-TOKEN", token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("MR !%d not found", number)
	}

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitLab API error: %s - %s", resp.Status, string(respBody))
	}

	var result struct {
		IID            int    `json:"iid"`
		WebURL         string `json:"web_url"`
		State          string `json:"state"`
		Title          string `json:"title"`
		Description    string `json:"description"`
		SourceBranch   string `json:"source_branch"`
		TargetBranch   string `json:"target_branch"`
		Draft          bool   `json:"draft"`
		WorkInProgress bool   `json:"work_in_progress"`
	}

	respBody, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	state := g.mapState(result.State, result.Draft || result.WorkInProgress)

	return &PR{
		Number: result.IID,
		URL:    result.WebURL,
		State:  state,
		Title:  result.Title,
		Body:   result.Description,
		Head:   result.SourceBranch,
		Base:   result.TargetBranch,
	}, nil
}

// GetByBranch retrieves a merge request for a given source branch.
func (g *GitLabProvider) GetByBranch(branch string) (*PR, error) {
	token, err := g.getToken()
	if err != nil {
		return nil, err
	}

	apiURL := fmt.Sprintf("%s/api/v4/projects/%s/merge_requests?source_branch=%s&state=opened",
		g.getBaseURL(), g.Project, url.QueryEscape(branch))
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("PRIVATE-TOKEN", token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitLab API error: %s - %s", resp.Status, string(respBody))
	}

	var results []struct {
		IID            int    `json:"iid"`
		WebURL         string `json:"web_url"`
		State          string `json:"state"`
		Title          string `json:"title"`
		Description    string `json:"description"`
		SourceBranch   string `json:"source_branch"`
		TargetBranch   string `json:"target_branch"`
		Draft          bool   `json:"draft"`
		WorkInProgress bool   `json:"work_in_progress"`
	}

	respBody, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(respBody, &results); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(results) == 0 {
		return nil, nil // No MR found
	}

	result := results[0]
	state := g.mapState(result.State, result.Draft || result.WorkInProgress)

	return &PR{
		Number: result.IID,
		URL:    result.WebURL,
		State:  state,
		Title:  result.Title,
		Body:   result.Description,
		Head:   result.SourceBranch,
		Base:   result.TargetBranch,
	}, nil
}

// Retarget changes the target branch of a merge request.
func (g *GitLabProvider) Retarget(number int, newBase string) error {
	token, err := g.getToken()
	if err != nil {
		return err
	}

	body := map[string]interface{}{
		"target_branch": newBase,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	apiURL := fmt.Sprintf("%s/api/v4/projects/%s/merge_requests/%d", g.getBaseURL(), g.Project, number)
	req, err := http.NewRequest("PUT", apiURL, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("PRIVATE-TOKEN", token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GitLab API error: %s - %s", resp.Status, string(respBody))
	}

	return nil
}

// Update updates an existing merge request.
func (g *GitLabProvider) Update(number int, opts UpdateOptions) error {
	token, err := g.getToken()
	if err != nil {
		return err
	}

	body := make(map[string]interface{})
	if opts.Title != nil {
		body["title"] = *opts.Title
	}
	if opts.Body != nil {
		body["description"] = *opts.Body // GitLab uses "description" instead of "body"
	}
	if opts.State != nil {
		// GitLab uses "state_event" with values: close, reopen
		switch *opts.State {
		case "closed", "close":
			body["state_event"] = "close"
		case "open", "opened", "reopen":
			body["state_event"] = "reopen"
		}
	}

	if len(body) == 0 {
		return nil // Nothing to update
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	apiURL := fmt.Sprintf("%s/api/v4/projects/%s/merge_requests/%d", g.getBaseURL(), g.Project, number)
	req, err := http.NewRequest("PUT", apiURL, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("PRIVATE-TOKEN", token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GitLab API error: %s - %s", resp.Status, string(respBody))
	}

	return nil
}

// Close closes a merge request without merging.
func (g *GitLabProvider) Close(number int) error {
	state := "close"
	return g.Update(number, UpdateOptions{State: &state})
}

// Merge merges a merge request.
func (g *GitLabProvider) Merge(number int, opts MergeOptions) error {
	token, err := g.getToken()
	if err != nil {
		return err
	}

	body := make(map[string]interface{})

	// GitLab merge options
	// squash: Squash commits when merging
	// should_remove_source_branch: Remove the source branch after merge
	switch opts.Method {
	case "squash":
		body["squash"] = true
	case "rebase":
		// GitLab handles this through merge request settings, not API
		// The merge will use fast-forward if possible when rebase is set in project settings
		body["merge_when_pipeline_succeeds"] = false
	}

	if opts.CommitMsg != "" {
		body["merge_commit_message"] = opts.CommitMsg
	}
	if opts.CommitTitle != "" {
		// GitLab combines title and message into merge_commit_message
		if opts.CommitMsg != "" {
			body["merge_commit_message"] = opts.CommitTitle + "\n\n" + opts.CommitMsg
		} else {
			body["merge_commit_message"] = opts.CommitTitle
		}
	}

	if opts.DeleteBranch {
		body["should_remove_source_branch"] = true
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	apiURL := fmt.Sprintf("%s/api/v4/projects/%s/merge_requests/%d/merge", g.getBaseURL(), g.Project, number)
	req, err := http.NewRequest("PUT", apiURL, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("PRIVATE-TOKEN", token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 405 {
		return fmt.Errorf("MR cannot be merged (not mergeable, requires approval, or has conflicts)")
	}

	if resp.StatusCode == 406 {
		return fmt.Errorf("MR has conflicts that must be resolved")
	}

	if resp.StatusCode == 401 {
		return fmt.Errorf("unauthorized: check your GitLab token permissions")
	}

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GitLab API error: %s - %s", resp.Status, string(respBody))
	}

	return nil
}

// DeleteBranch deletes a branch on GitLab.
func (g *GitLabProvider) DeleteBranch(branch string) error {
	token, err := g.getToken()
	if err != nil {
		return err
	}

	apiURL := fmt.Sprintf("%s/api/v4/projects/%s/repository/branches/%s",
		g.getBaseURL(), g.Project, url.PathEscape(branch))
	req, err := http.NewRequest("DELETE", apiURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("PRIVATE-TOKEN", token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 204 && resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GitLab API error: %s - %s", resp.Status, string(respBody))
	}

	return nil
}
