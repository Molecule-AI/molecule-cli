// Package mock implements a deterministic Backend for tests, demos,
// and CI smoke checks.
//
// Config keys:
//   - reply: response template. `%s` is replaced with the concatenated
//     inbound text parts. Default: "echo: %s".
//
// Registers itself as "mock". Activate via:
//
//	molecule connect <id> --backend mock --backend-opt reply="pong"
package mock

import (
	"context"
	"strings"

	"github.com/Molecule-AI/molecule-cli/internal/backends"
)

func init() {
	backends.Register("mock", New)
}

// New builds a mock backend from cfg.
func New(cfg backends.Config) (backends.Backend, error) {
	return &Backend{
		template: cfg.Get("reply", "echo: %s"),
	}, nil
}

// Backend is the mock implementation. Pure function: no state, no I/O,
// safe for concurrent use without locks.
type Backend struct {
	template string
}

// HandleA2A renders the reply template against the request's text
// parts and returns it as a single-part terminal response.
func (b *Backend) HandleA2A(_ context.Context, req backends.Request) (backends.Response, error) {
	var sb strings.Builder
	for _, p := range req.Parts {
		if p.Type == "text" {
			sb.WriteString(p.Text)
		}
	}
	reply := strings.ReplaceAll(b.template, "%s", sb.String())
	return backends.TextResponse(reply), nil
}

// Close is a no-op — nothing to release.
func (b *Backend) Close() error { return nil }
