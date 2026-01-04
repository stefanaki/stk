package pr

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
)

// GitHubProvider implements the Provider interface for GitHub.
type GitHubProvider struct {
	Token string
	Owner string
	Repo  string
}

// Name returns "github".
func (g *GitHubProvider) Name() string {
	return "github"
}

// Detect checks if the remote URL is a GitHub URL.
func (g *GitHubProvider) Detect(remoteURL string) bool {
	return strings.Contains(remoteURL, "github.com")
}

// SetRepo sets the owner and repo from a remote URL.
func (g *GitHubProvider) SetRepo(remoteURL string) error {
	owner, repo, err := ParseRemoteURL(remoteURL)
	if err != nil {
		return err
	}
	g.Owner = owner
	g.Repo = repo
	return nil
}

// getToken retrieves the GitHub token from environment or gh CLI.
func (g *GitHubProvider) getToken() (string, error) {
	if g.Token != "" {
		return g.Token, nil
	}

	// Check environment variable
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		g.Token = token
		return token, nil
	}

	// Try gh CLI
	cmd := exec.Command("gh", "auth", "token")
	out, err := cmd.Output()
	if err == nil {
		g.Token = strings.TrimSpace(string(out))
		return g.Token, nil
	}

	return "", fmt.Errorf("no GitHub token found; set GITHUB_TOKEN or login with 'gh auth login'")
}

// Create creates a new pull request on GitHub.
func (g *GitHubProvider) Create(opts CreateOptions) (*PR, error) {
	token, err := g.getToken()
	if err != nil {
		return nil, err
	}

	// Build request body
	body := map[string]interface{}{
		"title": opts.Title,
		"head":  opts.Head,
		"base":  opts.Base,
		"body":  opts.Body,
		"draft": opts.Draft,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create request
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls", g.Owner, g.Repo)
	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	// Send request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 201 {
		return nil, fmt.Errorf("GitHub API error: %s - %s", resp.Status, string(respBody))
	}

	// Parse response
	var result struct {
		Number  int    `json:"number"`
		HTMLURL string `json:"html_url"`
		State   string `json:"state"`
		Title   string `json:"title"`
		Draft   bool   `json:"draft"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	state := result.State
	if result.Draft {
		state = "draft"
	}

	return &PR{
		Number: result.Number,
		URL:    result.HTMLURL,
		State:  state,
		Title:  result.Title,
		Head:   opts.Head,
		Base:   opts.Base,
	}, nil
}

// Get retrieves a pull request by number.
func (g *GitHubProvider) Get(number int) (*PR, error) {
	token, err := g.getToken()
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls/%d", g.Owner, g.Repo, number)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("PR #%d not found", number)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitHub API error: %s", resp.Status)
	}

	var result struct {
		Number  int    `json:"number"`
		HTMLURL string `json:"html_url"`
		State   string `json:"state"`
		Title   string `json:"title"`
		Draft   bool   `json:"draft"`
		Head    struct {
			Ref string `json:"ref"`
		} `json:"head"`
		Base struct {
			Ref string `json:"ref"`
		} `json:"base"`
		Merged bool `json:"merged"`
	}

	respBody, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	state := result.State
	if result.Merged {
		state = "merged"
	} else if result.Draft {
		state = "draft"
	}

	return &PR{
		Number: result.Number,
		URL:    result.HTMLURL,
		State:  state,
		Title:  result.Title,
		Head:   result.Head.Ref,
		Base:   result.Base.Ref,
	}, nil
}

// GetByBranch retrieves a pull request for a given head branch.
func (g *GitHubProvider) GetByBranch(branch string) (*PR, error) {
	token, err := g.getToken()
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls?head=%s:%s&state=open",
		g.Owner, g.Repo, g.Owner, branch)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitHub API error: %s", resp.Status)
	}

	var results []struct {
		Number  int    `json:"number"`
		HTMLURL string `json:"html_url"`
		State   string `json:"state"`
		Title   string `json:"title"`
		Draft   bool   `json:"draft"`
		Head    struct {
			Ref string `json:"ref"`
		} `json:"head"`
		Base struct {
			Ref string `json:"ref"`
		} `json:"base"`
	}

	respBody, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(respBody, &results); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(results) == 0 {
		return nil, nil // No PR found
	}

	result := results[0]
	state := result.State
	if result.Draft {
		state = "draft"
	}

	return &PR{
		Number: result.Number,
		URL:    result.HTMLURL,
		State:  state,
		Title:  result.Title,
		Head:   result.Head.Ref,
		Base:   result.Base.Ref,
	}, nil
}

// Retarget changes the base branch of a PR.
func (g *GitHubProvider) Retarget(number int, newBase string) error {
	token, err := g.getToken()
	if err != nil {
		return err
	}

	body := map[string]interface{}{
		"base": newBase,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls/%d", g.Owner, g.Repo, number)
	req, err := http.NewRequest("PATCH", url, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GitHub API error: %s - %s", resp.Status, string(respBody))
	}

	return nil
}

// Update updates an existing pull request.
func (g *GitHubProvider) Update(number int, opts UpdateOptions) error {
	token, err := g.getToken()
	if err != nil {
		return err
	}

	body := make(map[string]interface{})
	if opts.Title != nil {
		body["title"] = *opts.Title
	}
	if opts.Body != nil {
		body["body"] = *opts.Body
	}
	if opts.State != nil {
		body["state"] = *opts.State
	}

	if len(body) == 0 {
		return nil // Nothing to update
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls/%d", g.Owner, g.Repo, number)
	req, err := http.NewRequest("PATCH", url, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GitHub API error: %s - %s", resp.Status, string(respBody))
	}

	return nil
}

// Close closes a pull request without merging.
func (g *GitHubProvider) Close(number int) error {
	state := "closed"
	return g.Update(number, UpdateOptions{State: &state})
}

// Merge merges a pull request.
func (g *GitHubProvider) Merge(number int, opts MergeOptions) error {
	token, err := g.getToken()
	if err != nil {
		return err
	}

	body := make(map[string]interface{})

	// Set merge method (default to merge)
	method := opts.Method
	if method == "" {
		method = "merge"
	}
	body["merge_method"] = method

	if opts.CommitTitle != "" {
		body["commit_title"] = opts.CommitTitle
	}
	if opts.CommitMsg != "" {
		body["commit_message"] = opts.CommitMsg
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls/%d/merge", g.Owner, g.Repo, number)
	req, err := http.NewRequest("PUT", url, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 405 {
		return fmt.Errorf("PR cannot be merged (not mergeable or requires review)")
	}

	if resp.StatusCode == 409 {
		return fmt.Errorf("PR has conflicts that must be resolved")
	}

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GitHub API error: %s - %s", resp.Status, string(respBody))
	}

	return nil
}

// DeleteBranch deletes a branch on GitHub.
func (g *GitHubProvider) DeleteBranch(branch string) error {
	token, err := g.getToken()
	if err != nil {
		return err
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/git/refs/heads/%s", g.Owner, g.Repo, branch)
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 204 && resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GitHub API error: %s - %s", resp.Status, string(respBody))
	}

	return nil
}
