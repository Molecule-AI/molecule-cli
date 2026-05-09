package connect

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"go.moleculesai.app/cli/internal/backends"
)

// Options carries the runtime knobs Run needs. Constructed by the cmd
// layer from cobra flags.
type Options struct {
	APIURL          string
	WorkspaceID     string
	Token           string
	AgentName       string         // sent in agent_card on register; default "molecule-connect"
	HeartbeatEvery  time.Duration  // default 30s
	PollEvery       time.Duration  // default 1s
	StateDir        string         // default DefaultStateDir()
	Backend         backends.Backend
	Logger          *log.Logger    // default log.Default()
	OnError         func(error)    // optional observer (tests use this)
}

// Run wires register → heartbeat goroutine + poll goroutine, returns
// when ctx is cancelled. Both goroutines drain on shutdown.
//
// Crash semantics: cursor is saved AFTER successful dispatch, so a
// SIGTERM mid-dispatch re-delivers on next start. Idempotency dedup
// (MessageID + IdempotencyKey) is the backend's responsibility — Run
// passes the keys through but does not enforce uniqueness across
// process restarts.
func Run(ctx context.Context, opts Options) error {
	if opts.Backend == nil {
		return fmt.Errorf("connect.Run: Backend is required")
	}
	if opts.WorkspaceID == "" {
		return fmt.Errorf("connect.Run: WorkspaceID is required")
	}
	if opts.HeartbeatEvery == 0 {
		opts.HeartbeatEvery = 30 * time.Second
	}
	if opts.PollEvery == 0 {
		opts.PollEvery = time.Second
	}
	if opts.AgentName == "" {
		opts.AgentName = "molecule-connect"
	}
	if opts.Logger == nil {
		opts.Logger = log.Default()
	}
	if opts.StateDir == "" {
		dir, err := DefaultStateDir()
		if err != nil {
			opts.Logger.Printf("connect: state dir unavailable, continuing without persistence: %v", err)
		}
		opts.StateDir = dir
	}

	state, err := LoadState(opts.StateDir, opts.WorkspaceID)
	if err != nil {
		opts.Logger.Printf("connect: load state failed (starting fresh): %v", err)
		state = State{WorkspaceID: opts.WorkspaceID}
	}

	cl := NewClient(opts.APIURL, opts.WorkspaceID, opts.Token)

	// Register is one-shot; failure here is fatal — we have no auth/identity
	// without it. Retry on transient errors with a short bounded backoff so
	// a flaky network doesn't immediately abort.
	if err := registerWithRetry(ctx, cl, opts); err != nil {
		return fmt.Errorf("register: %w", err)
	}
	opts.Logger.Printf("connect: registered workspace=%s mode=poll", opts.WorkspaceID)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		runHeartbeatLoop(ctx, cl, opts)
	}()
	go func() {
		defer wg.Done()
		runPollLoop(ctx, cl, opts, state)
	}()

	wg.Wait()
	return nil
}

// registerWithRetry handles the bootstrap call. Transient errors retry
// up to 5 times with linear backoff (1s, 2s, 4s, 8s, 16s); permanent
// errors abort.
func registerWithRetry(ctx context.Context, cl *Client, opts Options) error {
	const maxAttempts = 5
	delay := time.Second
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		err := cl.Register(ctx, opts.AgentName)
		if err == nil {
			return nil
		}
		var perm *PermanentError
		if errors.As(err, &perm) {
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		opts.Logger.Printf("connect: register attempt %d/%d failed: %v (retry in %s)",
			attempt, maxAttempts, err, delay)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
		delay *= 2
		if delay > 30*time.Second {
			delay = 30 * time.Second
		}
	}
	return fmt.Errorf("register: gave up after %d attempts", maxAttempts)
}

// runHeartbeatLoop pings /registry/heartbeat every opts.HeartbeatEvery.
// Transient failures log and continue with exponential backoff;
// permanent failures (e.g. 401 token revoked) log loudly and stop the
// loop — but Run() doesn't return until ctx is cancelled, so the user
// sees the error and SIGTERMs.
func runHeartbeatLoop(ctx context.Context, cl *Client, opts Options) {
	ticker := time.NewTicker(opts.HeartbeatEvery)
	defer ticker.Stop()

	failures := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		err := cl.Heartbeat(ctx)
		if err == nil {
			if failures > 0 {
				opts.Logger.Printf("connect: heartbeat recovered after %d failures", failures)
			}
			failures = 0
			continue
		}
		var perm *PermanentError
		if errors.As(err, &perm) {
			opts.Logger.Printf("connect: heartbeat permanent error (loop stopping): %v", err)
			notifyError(opts, err)
			return
		}
		failures++
		opts.Logger.Printf("connect: heartbeat transient error #%d: %v", failures, err)
		notifyError(opts, err)
		// At 5+ failures, slow down the cadence to reduce log spam — the
		// platform will mark us awaiting_agent after ~60s anyway.
		if failures >= 5 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Duration(failures) * opts.HeartbeatEvery):
			}
		}
	}
}

// runPollLoop fetches activity_logs since cursor, dispatches each row to
// the backend, posts the reply (when source is an agent — canvas-origin
// reply needs a different convention, deferred to M1.3+), then advances
// + persists the cursor.
//
// A poll batch is processed sequentially: the backend may be expensive
// (LLM call) and parallelism inside one batch invites out-of-order
// responses for in-flight conversations. Future: per-source serialization
// queue if the backend can be safely parallelized across sources.
func runPollLoop(ctx context.Context, cl *Client, opts Options, state State) {
	ticker := time.NewTicker(opts.PollEvery)
	defer ticker.Stop()

	transientFails := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		rows, err := cl.Activity(ctx, state.LastSinceID, 50)
		if err != nil {
			var perm *PermanentError
			if errors.As(err, &perm) {
				if perm.Status == 410 {
					// Cursor pruned — reset and re-fetch backlog.
					opts.Logger.Printf("connect: cursor %s pruned (410), resetting",
						state.LastSinceID)
					state.LastSinceID = ""
					_ = SaveState(opts.StateDir, state)
					continue
				}
				opts.Logger.Printf("connect: poll permanent error (loop stopping): %v", err)
				notifyError(opts, err)
				return
			}
			transientFails++
			opts.Logger.Printf("connect: poll transient error #%d: %v", transientFails, err)
			notifyError(opts, err)
			continue
		}
		transientFails = 0
		for _, row := range rows {
			if err := dispatchOne(ctx, cl, opts, row); err != nil {
				opts.Logger.Printf("connect: dispatch %s failed: %v", row.ID, err)
				notifyError(opts, err)
				// Save cursor BEFORE the failed row so the next poll
				// re-fetches it. If the failure is deterministic this
				// will spin — that's the operator's signal to fix the
				// backend or the message.
				return
			}
			state.LastSinceID = row.ID
			if err := SaveState(opts.StateDir, state); err != nil {
				opts.Logger.Printf("connect: save cursor failed (continuing): %v", err)
			}
		}
	}
}

// dispatchOne hands one activity row to the backend, posts the reply if
// applicable, and returns. Errors abort the current batch (caller saves
// cursor up to but not including this row).
func dispatchOne(ctx context.Context, cl *Client, opts Options, row ActivityRow) error {
	req, err := parseRequest(row)
	if err != nil {
		// Malformed inbound — log + skip (advance past it). A permanent
		// payload bug shouldn't deadlock the queue.
		opts.Logger.Printf("connect: parse row %s failed (skipping): %v", row.ID, err)
		return nil
	}
	resp, err := opts.Backend.HandleA2A(ctx, req)
	if err != nil {
		return fmt.Errorf("backend: %w", err)
	}

	// Inter-agent reply path. Canvas-origin (source_id == nil) reply
	// uses the activity_logs "task_update" convention (M1.3+); for now,
	// log + skip so the dispatch isn't lost silently.
	if row.SourceID == nil || *row.SourceID == "" {
		opts.Logger.Printf("connect: canvas-origin reply not yet wired (msg=%s parts=%d) — TODO M1.3",
			row.ID, len(resp.Parts))
		return nil
	}

	envelope := buildReplyEnvelope(req, resp)
	raw, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("marshal reply envelope: %w", err)
	}
	if err := cl.ReplyA2A(ctx, *row.SourceID, raw); err != nil {
		return fmt.Errorf("reply to %s: %w", *row.SourceID, err)
	}
	return nil
}

// parseRequest converts an activity_logs row's request_body into a
// backends.Request. Tolerates the common JSON-RPC shape:
//
//	{"jsonrpc":"2.0","method":"message/send","params":{"message":{"parts":[...]}}}
func parseRequest(row ActivityRow) (backends.Request, error) {
	method := ""
	if row.Method != nil {
		method = *row.Method
	}
	caller := ""
	if row.SourceID != nil {
		caller = *row.SourceID
	}

	out := backends.Request{
		WorkspaceID: row.WorkspaceID,
		CallerID:    caller,
		MessageID:   row.ID,
		Method:      method,
		Raw:         row.RequestBody,
	}

	if len(row.RequestBody) == 0 {
		return out, nil
	}
	var env struct {
		Params struct {
			Message struct {
				Parts          []backends.Part `json:"parts"`
				IdempotencyKey string          `json:"idempotency_key"`
				TaskID         string          `json:"task_id"`
			} `json:"message"`
		} `json:"params"`
	}
	if err := json.Unmarshal(row.RequestBody, &env); err != nil {
		// Not a JSON-RPC envelope — pass raw through, backend handles.
		return out, nil
	}
	out.Parts = env.Params.Message.Parts
	out.IdempotencyKey = env.Params.Message.IdempotencyKey
	out.TaskID = env.Params.Message.TaskID
	return out, nil
}

// buildReplyEnvelope shapes resp into the JSON-RPC reply that the
// platform's /workspaces/<source_id>/a2a expects. Mirrors the v0.3
// message/send shape — the source workspace's adapter parses parts the
// same way it parses any inbound message.
func buildReplyEnvelope(req backends.Request, resp backends.Response) map[string]interface{} {
	env := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      req.MessageID,
		"method":  "message/send",
		"params": map[string]interface{}{
			"message": map[string]interface{}{
				"role":  "assistant",
				"parts": resp.Parts,
			},
		},
	}
	if req.TaskID != "" {
		env["params"].(map[string]interface{})["message"].(map[string]interface{})["task_id"] = req.TaskID
	}
	return env
}

func notifyError(opts Options, err error) {
	if opts.OnError != nil {
		opts.OnError(err)
	}
}
