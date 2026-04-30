package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/Molecule-AI/molecule-cli/internal/backends"
	_ "github.com/Molecule-AI/molecule-cli/internal/backends/mock" // register backend
	"github.com/Molecule-AI/molecule-cli/internal/connect"
	"github.com/spf13/cobra"
)

// ---------------------------------------------------------------------------
// molecule connect — bridge an external-runtime workspace to a local backend.
//
// The full M1+ design lives in the RFC at
// https://github.com/Molecule-AI/molecule-cli/issues/10. This file owns the
// command surface; the wiring (heartbeat, activity poll, dispatch) lands in
// internal/connect/ in subsequent PRs.
// ---------------------------------------------------------------------------

var connectFlags struct {
	backend     string
	backendOpts []string // KEY=VALUE pairs, repeatable
	token       string
	mode        string // "poll" (default) | "push"
	intervalMs  int    // poll cadence, milliseconds
	sinceSecs   int    // initial activity-cursor lookback
	dryRun      bool   // build backend + print summary, do not start loops
}

var connectCmd = &cobra.Command{
	Use:   "connect <workspace-id>",
	Short: "Bridge an external workspace to a local backend (Claude Code, exec, mock, ...)",
	Long: `connect attaches the calling process to an external-runtime workspace.

Inbound A2A messages routed to the workspace are dispatched to the
selected --backend. The default backend is "claude-code" (bridges into
a running Claude Code session via the channel plugin); use "mock" for
CI smoke and "exec" for arbitrary shell handlers.

Authentication: the per-workspace token from the create response is
read from --token or the MOLECULE_WORKSPACE_TOKEN env var. The
platform URL is read from --api-url or MOLECULE_API_URL.

Mode: poll (default) is the no-public-URL path — the CLI long-polls
the platform for inbound activity. push requires the local box to be
reachable from the platform (public HTTPS); use only when running on
a server with an inbound URL.

Examples:

  # Default: bridge into a running Claude Code session
  molecule connect ws_01HF2K... --token $WS_TOKEN

  # CI smoke / demo — replies are deterministic
  molecule connect ws_01HF2K... --backend mock \
      --backend-opt reply="echo: %s"

  # Arbitrary shell handler
  molecule connect ws_01HF2K... --backend exec \
      --backend-opt cmd="python myhandler.py"

See full design: https://github.com/Molecule-AI/molecule-cli/issues/10`,
	Args: cobra.ExactArgs(1),
	RunE: runConnect,
}

func init() {
	connectCmd.Flags().StringVar(&connectFlags.backend, "backend", "claude-code",
		fmt.Sprintf("Backend that handles inbound A2A messages (registered: %s)",
			strings.Join(backends.Names(), ", ")))
	connectCmd.Flags().StringArrayVar(&connectFlags.backendOpts, "backend-opt", nil,
		"Backend-specific option, KEY=VALUE (repeatable)")
	connectCmd.Flags().StringVar(&connectFlags.token, "token",
		envOr("MOLECULE_WORKSPACE_TOKEN", ""),
		"Workspace auth token (env: MOLECULE_WORKSPACE_TOKEN)")
	connectCmd.Flags().StringVar(&connectFlags.mode, "mode", "poll",
		"Delivery mode: poll (no public URL needed) | push")
	connectCmd.Flags().IntVar(&connectFlags.intervalMs, "interval-ms", 1000,
		"Poll-mode interval between activity fetches, in milliseconds")
	connectCmd.Flags().IntVar(&connectFlags.sinceSecs, "since-secs", 30,
		"Poll-mode initial cursor lookback, in seconds")
	connectCmd.Flags().BoolVar(&connectFlags.dryRun, "dry-run", false,
		"Build the backend and print the connection summary, but do not start loops")
}

func runConnect(_ *cobra.Command, args []string) error {
	workspaceID := strings.TrimSpace(args[0])
	if workspaceID == "" {
		return &exitError{code: 2, msg: "connect: workspace-id is required"}
	}
	if connectFlags.token == "" {
		return &exitError{code: 2, msg: "connect: --token (or MOLECULE_WORKSPACE_TOKEN) is required"}
	}
	if connectFlags.mode != "poll" && connectFlags.mode != "push" {
		return &exitError{code: 2, msg: "connect: --mode must be poll or push"}
	}

	cfg, err := parseBackendOpts(connectFlags.backendOpts)
	if err != nil {
		return &exitError{code: 2, msg: err.Error()}
	}

	backend, err := backends.Build(connectFlags.backend, cfg)
	if err != nil {
		return &exitError{code: 2, msg: err.Error()}
	}

	fmt.Fprintf(os.Stderr, "molecule connect: workspace=%s backend=%s mode=%s api=%s\n",
		workspaceID, connectFlags.backend, connectFlags.mode, apiURL)

	if connectFlags.dryRun {
		fmt.Fprintln(os.Stderr, "molecule connect: --dry-run; backend built ok, not starting loops")
		return backend.Close()
	}

	// Push mode is the M4 work — for M1+ poll-mode is the supported path.
	// Surface a clear error so users with --mode push see why they're
	// blocked instead of getting confusing "Not Found" responses.
	if connectFlags.mode == "push" {
		return &exitError{code: 2,
			msg: "connect: push mode is not yet implemented (M4); use --mode poll"}
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	err = connect.Run(ctx, connect.Options{
		APIURL:         apiURL,
		WorkspaceID:    workspaceID,
		Token:          connectFlags.token,
		Backend:        backend,
		PollEvery:      time.Duration(connectFlags.intervalMs) * time.Millisecond,
		HeartbeatEvery: 30 * time.Second,
	})
	closeErr := backend.Close()
	if err != nil && err != context.Canceled {
		return err
	}
	return closeErr
}

// parseBackendOpts converts repeated KEY=VALUE flags into a Config map.
func parseBackendOpts(opts []string) (backends.Config, error) {
	cfg := backends.Config{}
	for _, opt := range opts {
		k, v, ok := strings.Cut(opt, "=")
		if !ok || k == "" {
			return nil, fmt.Errorf("--backend-opt: %q is not KEY=VALUE", opt)
		}
		cfg[k] = v
	}
	return cfg, nil
}
