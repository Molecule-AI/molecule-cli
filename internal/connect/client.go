// Package connect implements the runtime side of `molecule connect <id>` —
// register, heartbeat, poll, dispatch.
//
// Layout:
//   - client.go    — thin platform-API client (Register, Heartbeat, Activity, ReplyA2A)
//   - state.go     — cursor file at ~/.config/molecule/state/<workspace-id>.json
//   - connect.go   — Run() orchestrator that wires the loops to a Backend
//
// Robustness contract per RFC #10:
//   - Heartbeat and poll use independent goroutines so a slow backend dispatch
//     doesn't starve heartbeats (workspace would flip to 'awaiting_agent').
//   - Both loops respect ctx cancellation for clean SIGTERM shutdown.
//   - Network errors trigger exponential backoff (cap 60s); permanent errors
//     (4xx) abort with a clear message.
//   - Cursor file is written AFTER successful dispatch — a crash mid-batch
//     re-delivers the in-flight message, never drops it.
//   - Dispatch is idempotent against MessageID + IdempotencyKey so the
//     re-delivery doesn't double-fire the backend.
package connect

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Client is the platform-API surface that the loops talk to. Single
// struct so the heartbeat + poll goroutines share one http.Client with
// connection pooling. Concurrency-safe — each call builds its own
// *http.Request.
type Client struct {
	apiURL      string // e.g. "https://platform.example.com"
	workspaceID string
	token       string
	httpClient  *http.Client
}

// NewClient builds a platform-API client. apiURL must be the base URL
// (no trailing slash); the methods append paths.
func NewClient(apiURL, workspaceID, token string) *Client {
	return &Client{
		apiURL:      apiURL,
		workspaceID: workspaceID,
		token:       token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Register POSTs /registry/register with delivery_mode=poll and no URL
// (poll-mode workspaces don't need a public endpoint).
func (c *Client) Register(ctx context.Context, agentName string) error {
	body := map[string]interface{}{
		"id": c.workspaceID,
		"agent_card": map[string]interface{}{
			"name":        agentName,
			"description": "molecule connect (CLI bridge)",
			"version":     "0.1.0",
		},
		"delivery_mode": "poll",
	}
	return c.do(ctx, "POST", "/registry/register", body, nil)
}

// Heartbeat POSTs /registry/heartbeat. Called periodically by the
// heartbeat goroutine. The platform's TTL on the workspace's online
// status is short (~60s) so we beat every 30s by default.
func (c *Client) Heartbeat(ctx context.Context) error {
	body := map[string]interface{}{
		"workspace_id":   c.workspaceID,
		"runtime_state":  "ok",
		"uptime_seconds": 0, // not tracked by the bridge yet
	}
	return c.do(ctx, "POST", "/registry/heartbeat", body, nil)
}

// ActivityRow mirrors the activity_logs row shape returned by GET
// /workspaces/:id/activity. Only the fields the connect loops use are
// pulled; the rest pass through unread.
type ActivityRow struct {
	ID           string          `json:"id"`
	WorkspaceID  string          `json:"workspace_id"`
	ActivityType string          `json:"activity_type"`
	SourceID     *string         `json:"source_id"`
	TargetID     *string         `json:"target_id"`
	Method       *string         `json:"method"`
	Summary      *string         `json:"summary"`
	RequestBody  json.RawMessage `json:"request_body"`
	Status       string          `json:"status"`
	CreatedAt    string          `json:"created_at"`
}

// Activity GETs /workspaces/:id/activity with the given cursor. sinceID
// empty means "first call after register" — server returns the most
// recent backlog up to limit.
func (c *Client) Activity(ctx context.Context, sinceID string, limit int) ([]ActivityRow, error) {
	q := url.Values{}
	q.Set("type", "a2a_receive")
	if sinceID != "" {
		q.Set("since_id", sinceID)
	}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	path := "/workspaces/" + c.workspaceID + "/activity?" + q.Encode()
	var rows []ActivityRow
	if err := c.do(ctx, "GET", path, nil, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

// ReplyA2A posts a JSON-RPC reply envelope to the source workspace's
// /a2a endpoint. This is the inter-agent reply path; canvas-origin
// messages (source_id == nil) need a different convention — see
// connect.go for the canvas-reply TODO.
func (c *Client) ReplyA2A(ctx context.Context, sourceWorkspaceID string, envelope []byte) error {
	path := "/workspaces/" + sourceWorkspaceID + "/a2a"
	return c.doRaw(ctx, "POST", path, envelope, nil)
}

// do runs a JSON request: marshal body, decode response into out (when
// non-nil). 4xx is a permanent error, 5xx is a retryable error — the
// caller decides what to do with each.
func (c *Client) do(ctx context.Context, method, path string, body interface{}, out interface{}) error {
	var raw []byte
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal %s %s: %w", method, path, err)
		}
		raw = b
	}
	return c.doRaw(ctx, method, path, raw, out)
}

// doRaw is do() but with a pre-marshaled body — used by ReplyA2A which
// passes through the original JSON-RPC envelope without re-encoding.
func (c *Client) doRaw(ctx context.Context, method, path string, body []byte, out interface{}) error {
	var reader io.Reader
	if len(body) > 0 {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.apiURL+path, reader)
	if err != nil {
		return fmt.Errorf("build request %s %s: %w", method, path, err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	if out != nil {
		req.Header.Set("Accept", "application/json")
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		// Network/transport error — caller treats as retryable.
		return &TransientError{Op: method + " " + path, Err: err}
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	if resp.StatusCode >= 400 {
		// 4xx = permanent (caller config bug); 5xx = retryable.
		if resp.StatusCode >= 500 {
			return &TransientError{
				Op:     method + " " + path,
				Status: resp.StatusCode,
				Err:    fmt.Errorf("server error %d: %s", resp.StatusCode, truncate(respBody, 200)),
			}
		}
		return &PermanentError{
			Op:     method + " " + path,
			Status: resp.StatusCode,
			Body:   string(truncate(respBody, 200)),
		}
	}

	if out != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("decode %s %s: %w (body: %s)", method, path, err, truncate(respBody, 200))
		}
	}
	return nil
}

// TransientError is a network or 5xx error — the caller should retry
// with backoff.
type TransientError struct {
	Op     string
	Status int
	Err    error
}

func (e *TransientError) Error() string {
	if e.Status > 0 {
		return fmt.Sprintf("%s: transient %d: %v", e.Op, e.Status, e.Err)
	}
	return fmt.Sprintf("%s: transient: %v", e.Op, e.Err)
}

func (e *TransientError) Unwrap() error { return e.Err }

// PermanentError is a 4xx error — the caller should abort or surface
// the message to the user. Usually means token wrong, workspace
// removed, or payload malformed.
type PermanentError struct {
	Op     string
	Status int
	Body   string
}

func (e *PermanentError) Error() string {
	return fmt.Sprintf("%s: %d: %s", e.Op, e.Status, e.Body)
}

func truncate(b []byte, n int) []byte {
	if len(b) <= n {
		return b
	}
	out := make([]byte, 0, n+3)
	out = append(out, b[:n]...)
	out = append(out, "..."...)
	return out
}
