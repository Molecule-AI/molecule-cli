package connect_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.moleculesai.app/cli/internal/backends"
	_ "go.moleculesai.app/cli/internal/backends/mock" // register mock for tests
	"go.moleculesai.app/cli/internal/connect"
)

// fakeServer is the minimum workspace-server stub the loops need:
// /registry/register, /registry/heartbeat, /workspaces/:id/activity,
// /workspaces/:id/a2a (reply target).
type fakeServer struct {
	t         *testing.T
	registers atomic.Int32
	heartbeats atomic.Int32

	mu          sync.Mutex
	queue       []connect.ActivityRow
	repliesTo   map[string][]json.RawMessage // sourceID → reply envelopes
	replyStatus int                          // override response status when non-zero
	pollStatus  int                          // override response status when non-zero (one-shot)
}

func newFakeServer(t *testing.T) (*fakeServer, *httptest.Server) {
	fs := &fakeServer{t: t, repliesTo: map[string][]json.RawMessage{}}
	mux := http.NewServeMux()

	mux.HandleFunc("/registry/register", func(w http.ResponseWriter, r *http.Request) {
		fs.registers.Add(1)
		_ = r.Body.Close()
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/registry/heartbeat", func(w http.ResponseWriter, r *http.Request) {
		fs.heartbeats.Add(1)
		_ = r.Body.Close()
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/workspaces/", func(w http.ResponseWriter, r *http.Request) {
		// Routing: /workspaces/<id>/activity (GET) or /workspaces/<id>/a2a (POST).
		path := r.URL.Path
		switch {
		case strings.HasSuffix(path, "/activity") && r.Method == "GET":
			fs.mu.Lock()
			status := fs.pollStatus
			fs.pollStatus = 0
			if status != 0 {
				// Override response — leave queue intact so subsequent
				// polls can drain it after the override is consumed.
				fs.mu.Unlock()
				w.WriteHeader(status)
				return
			}
			rows := append([]connect.ActivityRow(nil), fs.queue...)
			fs.queue = nil
			fs.mu.Unlock()
			body, _ := json.Marshal(rows)
			w.Header().Set("Content-Type", "application/json")
			w.Write(body)
		case strings.HasSuffix(path, "/a2a") && r.Method == "POST":
			fs.mu.Lock()
			status := fs.replyStatus
			fs.replyStatus = 0
			fs.mu.Unlock()
			if status != 0 {
				w.WriteHeader(status)
				return
			}
			body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
			parts := strings.Split(strings.TrimPrefix(path, "/workspaces/"), "/")
			source := parts[0]
			fs.mu.Lock()
			fs.repliesTo[source] = append(fs.repliesTo[source], body)
			fs.mu.Unlock()
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return fs, srv
}

func (fs *fakeServer) enqueue(rows ...connect.ActivityRow) {
	fs.mu.Lock()
	fs.queue = append(fs.queue, rows...)
	fs.mu.Unlock()
}

func (fs *fakeServer) replyCount(source string) int {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return len(fs.repliesTo[source])
}

// TestRun_RoundTrip_AgentReply: end-to-end round-trip for an inter-agent
// message. enqueue an activity row → poll loop fetches → mock backend
// echoes → reply posted to source's /a2a → cursor saved.
func TestRun_RoundTrip_AgentReply(t *testing.T) {
	fs, srv := newFakeServer(t)
	source := "ws-source"
	fs.enqueue(connect.ActivityRow{
		ID:           "act-1",
		WorkspaceID:  "ws-target",
		ActivityType: "a2a_receive",
		SourceID:     &source,
		Method:       strPtr("message/send"),
		RequestBody:  json.RawMessage(`{"jsonrpc":"2.0","method":"message/send","params":{"message":{"parts":[{"type":"text","text":"ping"}]}}}`),
	})

	mock, err := backends.Build("mock", backends.Config{"reply": "pong: %s"})
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	stateDir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- connect.Run(ctx, connect.Options{
			APIURL:         srv.URL,
			WorkspaceID:    "ws-target",
			Token:          "tok",
			Backend:        mock,
			PollEvery:      20 * time.Millisecond,
			HeartbeatEvery: 50 * time.Millisecond,
			StateDir:       stateDir,
		})
	}()

	// Spin until both signals fire (reply lands + at least one heartbeat
	// ticked) or ctx times out.
	deadline := time.Now().Add(2 * time.Second)
	for (fs.replyCount(source) == 0 || fs.heartbeats.Load() == 0) && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	<-done

	if fs.registers.Load() == 0 {
		t.Error("expected at least one register call")
	}
	if fs.heartbeats.Load() == 0 {
		t.Error("expected at least one heartbeat call")
	}
	if got := fs.replyCount(source); got != 1 {
		t.Fatalf("reply count: got %d, want 1", got)
	}

	// Verify the reply envelope shape.
	var env map[string]interface{}
	if err := json.Unmarshal(fs.repliesTo[source][0], &env); err != nil {
		t.Fatal(err)
	}
	if env["jsonrpc"] != "2.0" {
		t.Errorf("missing jsonrpc field: %v", env)
	}
	params := env["params"].(map[string]interface{})
	msg := params["message"].(map[string]interface{})
	parts := msg["parts"].([]interface{})
	got := parts[0].(map[string]interface{})["text"].(string)
	if got != "pong: ping" {
		t.Errorf("reply text: got %q, want %q", got, "pong: ping")
	}

	// Cursor was persisted past act-1.
	state, _ := connect.LoadState(stateDir, "ws-target")
	if state.LastSinceID != "act-1" {
		t.Errorf("cursor: got %q, want act-1", state.LastSinceID)
	}
}

// TestRun_CanvasOriginMessageNotReplied: source_id == nil → backend
// dispatches but no reply post (canvas-reply convention deferred).
func TestRun_CanvasOriginMessageNotReplied(t *testing.T) {
	fs, srv := newFakeServer(t)
	fs.enqueue(connect.ActivityRow{
		ID:           "act-canvas",
		WorkspaceID:  "ws-target",
		ActivityType: "a2a_receive",
		SourceID:     nil, // canvas
		Method:       strPtr("message/send"),
		RequestBody:  json.RawMessage(`{"params":{"message":{"parts":[{"type":"text","text":"hi"}]}}}`),
	})

	dispatched := atomic.Int32{}
	be := &spyBackend{onHandle: func() { dispatched.Add(1) }}

	stateDir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- connect.Run(ctx, connect.Options{
			APIURL:      srv.URL,
			WorkspaceID: "ws-target",
			Token:       "tok",
			Backend:     be,
			PollEvery:   20 * time.Millisecond,
			StateDir:    stateDir,
		})
	}()

	deadline := time.Now().Add(800 * time.Millisecond)
	for dispatched.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	<-done

	if dispatched.Load() == 0 {
		t.Error("expected backend dispatch for canvas-origin message")
	}
	// No reply target; just verify the cursor advanced.
	state, _ := connect.LoadState(stateDir, "ws-target")
	if state.LastSinceID != "act-canvas" {
		t.Errorf("cursor: got %q, want act-canvas", state.LastSinceID)
	}
}

// TestRun_CursorPruned410ResetsAndContinues: when the platform returns
// 410 Gone on the cursor, the loop resets to "" and re-fetches.
func TestRun_CursorPruned410ResetsAndContinues(t *testing.T) {
	fs, srv := newFakeServer(t)

	stateDir := t.TempDir()
	// Pre-seed a cursor that the server will reject.
	connect.SaveState(stateDir, connect.State{WorkspaceID: "ws-target", LastSinceID: "act-pruned"})

	// First poll responds 410; next polls return the row.
	fs.mu.Lock()
	fs.pollStatus = http.StatusGone
	fs.mu.Unlock()
	source := "ws-source"
	fs.enqueue(connect.ActivityRow{
		ID:           "act-fresh",
		WorkspaceID:  "ws-target",
		ActivityType: "a2a_receive",
		SourceID:     &source,
		Method:       strPtr("message/send"),
		RequestBody:  json.RawMessage(`{"params":{"message":{"parts":[{"type":"text","text":"x"}]}}}`),
	})

	mock, _ := backends.Build("mock", nil)
	defer mock.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- connect.Run(ctx, connect.Options{
			APIURL:      srv.URL,
			WorkspaceID: "ws-target",
			Token:       "tok",
			Backend:     mock,
			PollEvery:   20 * time.Millisecond,
			StateDir:    stateDir,
		})
	}()

	deadline := time.Now().Add(1200 * time.Millisecond)
	for fs.replyCount(source) == 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	<-done

	if fs.replyCount(source) == 0 {
		t.Fatal("expected reply after cursor reset")
	}
	state, _ := connect.LoadState(stateDir, "ws-target")
	if state.LastSinceID != "act-fresh" {
		t.Errorf("cursor: got %q, want act-fresh", state.LastSinceID)
	}
}

// TestRun_PermanentRegisterErrorAborts: 401 on register surfaces and
// Run returns the error (no infinite retry loop).
func TestRun_PermanentRegisterErrorAborts(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/registry/register", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	mock, _ := backends.Build("mock", nil)
	defer mock.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := connect.Run(ctx, connect.Options{
		APIURL:      srv.URL,
		WorkspaceID: "ws-x",
		Token:       "bad",
		Backend:     mock,
		StateDir:    t.TempDir(),
	})
	if err == nil {
		t.Fatal("expected register to fail with 401")
	}
	if !strings.Contains(err.Error(), "register") {
		t.Errorf("error should mention register: %v", err)
	}
}

// TestRun_TransientRegisterErrorRetries: 500 on register triggers retry,
// then succeeds — Run proceeds to start loops.
func TestRun_TransientRegisterErrorRetries(t *testing.T) {
	calls := atomic.Int32{}
	mux := http.NewServeMux()
	mux.HandleFunc("/registry/register", func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/registry/heartbeat", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/workspaces/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("[]"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	mock, _ := backends.Build("mock", nil)
	defer mock.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- connect.Run(ctx, connect.Options{
			APIURL:      srv.URL,
			WorkspaceID: "ws-x",
			Token:       "tok",
			Backend:     mock,
			PollEvery:   50 * time.Millisecond,
			StateDir:    t.TempDir(),
		})
	}()

	// Wait until at least 2 register attempts (1 fail + 1 success).
	deadline := time.Now().Add(4 * time.Second)
	for calls.Load() < 2 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	<-done

	if calls.Load() < 2 {
		t.Errorf("expected at least 2 register attempts (retry), got %d", calls.Load())
	}
}

// TestRun_OptionsValidation: missing required fields surface immediately.
func TestRun_OptionsValidation(t *testing.T) {
	mock, _ := backends.Build("mock", nil)
	defer mock.Close()

	cases := []struct {
		name string
		opts connect.Options
		want string
	}{
		{"no backend", connect.Options{WorkspaceID: "ws"}, "Backend"},
		{"no workspace id", connect.Options{Backend: mock}, "WorkspaceID"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := connect.Run(context.Background(), tc.opts)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q missing %q", err.Error(), tc.want)
			}
		})
	}
}

// spyBackend lets a test count HandleA2A invocations + inject behavior.
type spyBackend struct {
	onHandle func()
	err      error
}

func (s *spyBackend) HandleA2A(_ context.Context, _ backends.Request) (backends.Response, error) {
	if s.onHandle != nil {
		s.onHandle()
	}
	if s.err != nil {
		return backends.Response{}, s.err
	}
	return backends.TextResponse("ok"), nil
}

func (s *spyBackend) Close() error { return nil }

// strPtr returns a pointer to s — convenience for the *string fields on
// ActivityRow.
func strPtr(s string) *string { return &s }

// Compile-time assertion that connect.PermanentError + TransientError
// satisfy the typical errors.As idiom callers will use.
var (
	_ error = (*connect.PermanentError)(nil)
	_ error = (*connect.TransientError)(nil)
)

// Quick sanity check of the error wrapping shape — exercised in dispatch
// error paths in callers.
func TestPermanentError_Format(t *testing.T) {
	e := &connect.PermanentError{Op: "POST /x", Status: 401, Body: "bad token"}
	if !strings.Contains(e.Error(), "401") {
		t.Errorf("error missing status: %s", e.Error())
	}
}

func TestTransientError_Unwrap(t *testing.T) {
	inner := errors.New("dial failed")
	e := &connect.TransientError{Op: "GET /x", Err: inner}
	if !errors.Is(e, inner) {
		t.Error("transient error should unwrap to inner")
	}
}
