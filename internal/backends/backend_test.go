package backends_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Molecule-AI/molecule-cli/internal/backends"
	_ "github.com/Molecule-AI/molecule-cli/internal/backends/mock" // register
)

func TestRegister_DuplicatePanics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic on duplicate registration")
		}
		msg, ok := r.(string)
		if !ok || !strings.Contains(msg, "duplicate backend name") {
			t.Fatalf("unexpected panic value: %v", r)
		}
	}()
	// "mock" is already registered via the package-level import above.
	backends.Register("mock", func(backends.Config) (backends.Backend, error) {
		return nil, nil
	})
}

func TestRegister_EmptyNamePanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on empty name")
		}
	}()
	backends.Register("", func(backends.Config) (backends.Backend, error) {
		return nil, nil
	})
}

func TestRegister_NilFactoryPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on nil factory")
		}
	}()
	backends.Register("nilfactory", nil)
}

func TestBuild_UnknownBackend(t *testing.T) {
	_, err := backends.Build("does-not-exist", nil)
	if err == nil {
		t.Fatal("expected error on unknown backend")
	}
	// Error must include the known list so users can see what's available.
	if !strings.Contains(err.Error(), "registered:") {
		t.Errorf("error missing registered-list hint: %v", err)
	}
}

func TestBuild_MockBackend(t *testing.T) {
	b, err := backends.Build("mock", backends.Config{"reply": "pong: %s"})
	if err != nil {
		t.Fatalf("Build(mock): %v", err)
	}
	defer b.Close()

	resp, err := b.HandleA2A(context.Background(), backends.Request{
		WorkspaceID: "ws-test",
		Parts:       []backends.Part{{Type: "text", Text: "ping"}},
	})
	if err != nil {
		t.Fatalf("HandleA2A: %v", err)
	}
	if !resp.Final {
		t.Error("expected Final=true on terminal response")
	}
	if len(resp.Parts) != 1 {
		t.Fatalf("expected 1 part, got %d", len(resp.Parts))
	}
	if got, want := resp.Parts[0].Text, "pong: ping"; got != want {
		t.Errorf("reply: got %q, want %q", got, want)
	}
}

func TestBuild_MockBackend_DefaultTemplate(t *testing.T) {
	b, err := backends.Build("mock", nil)
	if err != nil {
		t.Fatalf("Build(mock, nil cfg): %v", err)
	}
	defer b.Close()

	resp, err := b.HandleA2A(context.Background(), backends.Request{
		Parts: []backends.Part{{Type: "text", Text: "hi"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := resp.Parts[0].Text, "echo: hi"; got != want {
		t.Errorf("default template: got %q, want %q", got, want)
	}
}

func TestBuild_MockBackend_ConcatenatesTextParts(t *testing.T) {
	b, err := backends.Build("mock", backends.Config{"reply": "%s"})
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()

	resp, _ := b.HandleA2A(context.Background(), backends.Request{
		Parts: []backends.Part{
			{Type: "text", Text: "hello "},
			{Type: "data", Data: map[string]interface{}{"k": "v"}}, // ignored
			{Type: "text", Text: "world"},
		},
	})
	if got, want := resp.Parts[0].Text, "hello world"; got != want {
		t.Errorf("concatenation: got %q, want %q", got, want)
	}
}

func TestNames_IncludesMock(t *testing.T) {
	names := backends.Names()
	found := false
	for _, n := range names {
		if n == "mock" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Names() missing 'mock': %v", names)
	}
}

func TestConfig_GetWithFallback(t *testing.T) {
	cfg := backends.Config{"present": "yes"}
	if got := cfg.Get("present", "fb"); got != "yes" {
		t.Errorf("Get present: got %q, want yes", got)
	}
	if got := cfg.Get("absent", "fb"); got != "fb" {
		t.Errorf("Get absent: got %q, want fb", got)
	}
	if got := cfg.Get("present", "fb"); got != "yes" {
		t.Errorf("Get present consistent")
	}
	// Empty value triggers fallback (treated as unset).
	cfg["empty"] = ""
	if got := cfg.Get("empty", "fb"); got != "fb" {
		t.Errorf("Get empty: got %q, want fb", got)
	}
}

func TestConfig_RequireMissing(t *testing.T) {
	cfg := backends.Config{}
	_, err := cfg.Require("nope")
	if err == nil {
		t.Fatal("expected error on missing required key")
	}
	if !strings.Contains(err.Error(), "nope") {
		t.Errorf("error should name missing key: %v", err)
	}
}

func TestConfig_RequirePresent(t *testing.T) {
	cfg := backends.Config{"k": "v"}
	got, err := cfg.Require("k")
	if err != nil {
		t.Fatal(err)
	}
	if got != "v" {
		t.Errorf("got %q, want v", got)
	}
}

func TestTextResponse_Shape(t *testing.T) {
	resp := backends.TextResponse("hello")
	if !resp.Final {
		t.Error("TextResponse should be Final")
	}
	if len(resp.Parts) != 1 || resp.Parts[0].Type != "text" || resp.Parts[0].Text != "hello" {
		t.Errorf("bad shape: %+v", resp)
	}
}
