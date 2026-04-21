// Package client provides a thin HTTP client for the Molecule AI platform API.
package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Platform is the root API client. All methods return an error or typed result.
type Platform struct {
	BaseURL string
	client  *http.Client
}

// New returns a Platform client configured with baseURL.
func New(baseURL string) *Platform {
	return &Platform{
		BaseURL: baseURL,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Workspace represents a Molecule AI workspace as returned by the platform.
type Workspace struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Status       string  `json:"status"`
	Role         string  `json:"role,omitempty"`
	ParentID     string  `json:"parent_id,omitempty"`
	Runtime      string  `json:"runtime,omitempty"`
	WorkspaceDir string  `json:"workspace_dir,omitempty"`
	CreatedAt    string  `json:"created_at,omitempty"`
	Canvas       *Canvas `json:"canvas,omitempty"`
}

// Canvas holds the workspace's position on the canvas.
type Canvas struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// ListWorkspaces returns all workspaces in the org.
func (p *Platform) ListWorkspaces() ([]Workspace, error) {
	url := p.BaseURL + "/workspaces"
	resp, err := p.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GET %s: HTTP %d — %s", url, resp.StatusCode, string(body))
	}
	var out []Workspace
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode workspaces: %w", err)
	}
	return out, nil
}

// GetWorkspace returns a single workspace by ID.
func (p *Platform) GetWorkspace(id string) (*Workspace, error) {
	url := p.BaseURL + "/workspaces/" + id
	resp, err := p.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("workspace %q not found", id)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GET %s: HTTP %d — %s", url, resp.StatusCode, string(body))
	}
	var ws Workspace
	if err := json.NewDecoder(resp.Body).Decode(&ws); err != nil {
		return nil, fmt.Errorf("decode workspace: %w", err)
	}
	return &ws, nil
}
