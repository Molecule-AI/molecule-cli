// Package client provides a thin HTTP client for the Molecule AI platform API.
package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Platform is the root API client.
type Platform struct {
	BaseURL string
	client  *http.Client
}

// New returns a Platform client configured with baseURL.
func New(baseURL string) *Platform {
	return &Platform{
		BaseURL: baseURL,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

// Workspace represents a Molecule AI workspace.
type Workspace struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Status       string  `json:"status"`
	Role         string  `json:"role,omitempty"`
	ParentID     string  `json:"parent_id,omitempty"`
	Runtime      string  `json:"runtime,omitempty"`
	WorkspaceDir string  `json:"workspace_dir,omitempty"`
	CreatedAt    string  `json:"created_at,omitempty"`
	Tier         int     `json:"tier,omitempty"`
	Canvas       *Canvas `json:"canvas,omitempty"`
}

// Canvas holds the workspace's position on the canvas.
type Canvas struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// Agent represents an agent running in a workspace.
type Agent struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	WorkspaceID string `json:"workspace_id,omitempty"`
	Status      string `json:"status"`
	Model       string `json:"model,omitempty"`
	Runtime     string `json:"runtime,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
}

// CreateWorkspaceRequest mirrors the platform's POST /workspaces body.
type CreateWorkspaceRequest struct {
	Name         string `json:"name"`
	Role         string `json:"role,omitempty"`
	Template     string `json:"template,omitempty"`
	Tier         int    `json:"tier,omitempty"`
	ParentID     string `json:"parent_id,omitempty"`
	Runtime      string `json:"runtime,omitempty"`
	WorkspaceDir string `json:"workspace_dir,omitempty"`
}

// PlatformHealth holds the /health endpoint response.
type PlatformHealth struct {
	Status   string `json:"status"`
	Version  string `json:"version,omitempty"`
	Uptime   string `json:"uptime,omitempty"`
	Database string `json:"database,omitempty"`
}

// ConfigEntry represents a config key-value pair.
type ConfigEntry struct {
	Key   string `json:"key"`
	Value string `json:"value,omitempty"`
}

// ListWorkspaces returns all workspaces in the org.
func (p *Platform) ListWorkspaces() ([]Workspace, error) {
	var out []Workspace
	if err := p.getInto("/workspaces", &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetWorkspace returns a single workspace by ID.
func (p *Platform) GetWorkspace(id string) (*Workspace, error) {
	var out Workspace
	if err := p.getInto(fmt.Sprintf("/workspaces/%s", id), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CreateWorkspace creates a new workspace.
func (p *Platform) CreateWorkspace(req CreateWorkspaceRequest) (*Workspace, error) {
	var out Workspace
	if err := p.postInto("/workspaces", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteWorkspace deletes a workspace by ID.
func (p *Platform) DeleteWorkspace(id string) error {
	_, err := p.delete(fmt.Sprintf("/workspaces/%s?confirm=true", id))
	return err
}

// RestartWorkspace triggers a restart for a workspace.
func (p *Platform) RestartWorkspace(id string) error {
	_, err := p.postEmpty(fmt.Sprintf("/workspaces/%s/restart", id))
	return err
}

// ListAgents returns all agents across the org.
func (p *Platform) ListAgents() ([]Agent, error) {
	var out []Agent
	if err := p.getInto("/agents", &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ListWorkspaceAgents returns agents for a given workspace.
func (p *Platform) ListWorkspaceAgents(workspaceID string) ([]Agent, error) {
	var out []Agent
	if err := p.getInto(fmt.Sprintf("/workspaces/%s/agents", workspaceID), &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetAgent returns a single agent by ID.
func (p *Platform) GetAgent(id string) (*Agent, error) {
	var out Agent
	if err := p.getInto(fmt.Sprintf("/agents/%s", id), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Health returns the platform's /health status.
func (p *Platform) Health() (*PlatformHealth, error) {
	var out PlatformHealth
	if err := p.getInto("/health", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// AuditWorkspaces returns all workspaces and agents.
func (p *Platform) AuditWorkspaces() ([]Workspace, []Agent, error) {
	ws, err := p.ListWorkspaces()
	if err != nil {
		return nil, nil, fmt.Errorf("audit workspaces: %w", err)
	}
	agents, err := p.ListAgents()
	if err != nil {
		return ws, nil, fmt.Errorf("audit agents: %w", err)
	}
	return ws, agents, nil
}

// GetPeers returns peer workspaces reachable from a workspace.
func (p *Platform) GetPeers(workspaceID string) ([]Agent, error) {
	var out []Agent
	if err := p.getInto(fmt.Sprintf("/registry/%s/peers", workspaceID), &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetDelegations returns delegation status for a workspace.
func (p *Platform) GetDelegations(workspaceID string) ([]map[string]interface{}, error) {
	var out []map[string]interface{}
	if err := p.getInto(fmt.Sprintf("/workspaces/%s/delegations", workspaceID), &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Private HTTP helpers
// ---------------------------------------------------------------------------

func (p *Platform) getInto(path string, out interface{}) error {
	url := p.BaseURL + path
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("new GET request: %w", err)
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("GET %s: HTTP %d — %s", url, resp.StatusCode, string(body))
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("decode GET %s: %w", path, err)
	}
	return nil
}

func (p *Platform) postInto(path string, body interface{}, out interface{}) error {
	encoded, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal POST body: %w", err)
	}
	url := p.BaseURL + path
	req, err := http.NewRequest("POST", url, bytes.NewReader(encoded))
	if err != nil {
		return fmt.Errorf("new POST request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("POST %s: %w", url, err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("POST %s: HTTP %d — %s", url, resp.StatusCode, string(respBody))
	}
	if out != nil {
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("decode POST %s response: %w", path, err)
		}
	}
	return nil
}

func (p *Platform) delete(path string) ([]byte, error) {
	url := p.BaseURL + path
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return nil, fmt.Errorf("new DELETE request: %w", err)
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("DELETE %s: %w", url, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("DELETE %s: HTTP %d — %s", url, resp.StatusCode, string(body))
	}
	return body, nil
}

func (p *Platform) postEmpty(path string) ([]byte, error) {
	url := p.BaseURL + path
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return nil, fmt.Errorf("new POST request: %w", err)
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("POST %s: %w", url, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("POST %s: HTTP %d — %s", url, resp.StatusCode, string(body))
	}
	return body, nil
}