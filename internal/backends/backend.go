// Package backends defines the pluggable handler interface that
// `molecule connect` dispatches inbound A2A messages to.
//
// Each backend impl is a sub-package (claude_code/, exec/, mock/, etc.)
// that registers itself via `Register()` from an `init()` block.
// Runtime selection is done via the --backend flag.
//
// See RFC: https://github.com/Molecule-AI/molecule-cli/issues/10
package backends

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

// Request is the inbound A2A message handed to a Backend. Mirrors the
// JSON-RPC `params` shape that workspace-server's /workspaces/:id/a2a
// endpoint consumes — kept lossless so backends can re-issue the request
// to a downstream system without re-parsing.
//
// The fields here are the stable contract; new optional fields can be
// added but must be additive.
type Request struct {
	// WorkspaceID is the ID of the receiving workspace (this side).
	WorkspaceID string `json:"workspace_id"`
	// CallerID is the workspace ID of the sender, when known. Empty for
	// canvas-originated messages.
	CallerID string `json:"caller_id,omitempty"`
	// MessageID is the per-message UUID. Unique per send; backends use
	// this for idempotency dedupe.
	MessageID string `json:"message_id,omitempty"`
	// IdempotencyKey is the caller-supplied dedupe key. If set, prefer
	// it over MessageID for de-dup.
	IdempotencyKey string `json:"idempotency_key,omitempty"`
	// TaskID is the long-running task this message belongs to (when the
	// caller is in a delegation flow).
	TaskID string `json:"task_id,omitempty"`
	// Parts carries the message content (text/file/data parts per A2A
	// v0.3). Backends that only handle text concatenate the text parts.
	Parts []Part `json:"parts"`
	// Method is the JSON-RPC method ("message/send", "message/stream",
	// etc.) — backends that can stream may branch on this.
	Method string `json:"method,omitempty"`
	// Raw is the unparsed JSON-RPC envelope, kept for backends that need
	// to forward the full request shape (mcp, openai-passthrough).
	Raw []byte `json:"-"`
}

// Part is one A2A message part. Type is "text", "file", "data".
type Part struct {
	Type     string                 `json:"type"`
	Text     string                 `json:"text,omitempty"`
	MimeType string                 `json:"mime_type,omitempty"`
	URI      string                 `json:"uri,omitempty"`
	Data     map[string]interface{} `json:"data,omitempty"`
}

// Response is the backend's reply to an A2A request.
type Response struct {
	// Parts is the response content. At least one part is required;
	// most backends produce a single text part.
	Parts []Part `json:"parts"`
	// Final indicates this is the terminal response for the request
	// (vs an intermediate streaming chunk). Single-shot backends
	// always set true.
	Final bool `json:"final"`
}

// TextResponse is a convenience constructor for the common case: a
// single text part, terminal response.
func TextResponse(text string) Response {
	return Response{
		Parts: []Part{{Type: "text", Text: text}},
		Final: true,
	}
}

// Backend is the seam every concrete handler implements. Two methods,
// no inheritance, no surprise side effects: HandleA2A is called once
// per inbound message, Close once at shutdown.
//
// Backends MUST be safe for concurrent HandleA2A calls — `molecule
// connect` may dispatch poll-batch messages in parallel.
type Backend interface {
	// HandleA2A processes one inbound message and returns the reply.
	// Implementations should respect ctx cancellation; the caller may
	// cancel on shutdown.
	HandleA2A(ctx context.Context, req Request) (Response, error)
	// Close releases backend resources (subprocess, network conn, etc.).
	// Called exactly once during graceful shutdown. Must be idempotent.
	Close() error
}

// Factory builds a Backend from per-backend config. Returned by
// each backend impl's `init()`-time registration.
type Factory func(cfg Config) (Backend, error)

// Config is the loosely-typed bag of per-backend options. Each backend
// documents the keys it consumes in its package-level doc. Unknown
// keys are ignored so adding a key doesn't break existing setups.
type Config map[string]string

// Get returns cfg[key], or fallback if unset.
func (c Config) Get(key, fallback string) string {
	if v, ok := c[key]; ok && v != "" {
		return v
	}
	return fallback
}

// Require returns cfg[key], or an error if unset/empty. Use for keys
// the backend cannot start without.
func (c Config) Require(key string) (string, error) {
	v := c[key]
	if v == "" {
		return "", fmt.Errorf("backend config: %q is required", key)
	}
	return v, nil
}

var (
	registryMu sync.RWMutex
	registry   = map[string]Factory{}
)

// Register adds a backend Factory under name. Called from each backend
// impl's init() block. Panics on duplicate name — registration drift
// is a programming error and should fail loudly at startup.
func Register(name string, factory Factory) {
	if name == "" {
		panic("backends.Register: name must be non-empty")
	}
	if factory == nil {
		panic("backends.Register: factory must be non-nil")
	}
	registryMu.Lock()
	defer registryMu.Unlock()
	if _, dup := registry[name]; dup {
		panic("backends.Register: duplicate backend name " + name)
	}
	registry[name] = factory
}

// Build instantiates the named backend with cfg. Returns an error if
// no backend is registered under that name (typo, missing build tag,
// etc.) — callers should surface the error with a clear message that
// includes the list from `Names()`.
func Build(name string, cfg Config) (Backend, error) {
	registryMu.RLock()
	factory, ok := registry[name]
	registryMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("backends.Build: unknown backend %q (registered: %v)", name, Names())
	}
	return factory(cfg)
}

// Names returns the sorted list of registered backend names. Used in
// `--help` rendering and error messages.
func Names() []string {
	registryMu.RLock()
	out := make([]string, 0, len(registry))
	for k := range registry {
		out = append(out, k)
	}
	registryMu.RUnlock()
	sort.Strings(out)
	return out
}
