package cmd

import (
	"strings"
	"testing"
)

func TestParseBackendOpts(t *testing.T) {
	cases := []struct {
		name    string
		input   []string
		want    map[string]string
		wantErr bool
	}{
		{"empty", nil, map[string]string{}, false},
		{"single", []string{"reply=pong"}, map[string]string{"reply": "pong"}, false},
		{
			"multiple",
			[]string{"reply=pong", "cmd=python x.py"},
			map[string]string{"reply": "pong", "cmd": "python x.py"},
			false,
		},
		{
			"value contains equals",
			[]string{"url=https://x.com?a=b"},
			map[string]string{"url": "https://x.com?a=b"},
			false,
		},
		{"empty value allowed", []string{"k="}, map[string]string{"k": ""}, false},
		{"missing equals", []string{"justakey"}, nil, true},
		{"empty key", []string{"=v"}, nil, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseBackendOpts(tc.input)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr = %v", err, tc.wantErr)
			}
			if tc.wantErr {
				return
			}
			if len(got) != len(tc.want) {
				t.Errorf("len: got %d, want %d (%+v)", len(got), len(tc.want), got)
			}
			for k, v := range tc.want {
				if got[k] != v {
					t.Errorf("key %q: got %q, want %q", k, got[k], v)
				}
			}
		})
	}
}

// TestConnect_FlagValidation walks the invalid argument paths the runner
// guards against. We don't actually open a socket — the flags are
// rejected before any I/O.
func TestConnect_FlagValidation(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		wantSub string
	}{
		// Cobra catches missing args before runConnect runs.
		{"no token", []string{"connect", "ws-1", "--backend", "mock"}, "token"},
		{
			"bad mode",
			[]string{"connect", "ws-1", "--backend", "mock", "--token", "t", "--mode", "ftp"},
			"mode",
		},
		{
			"bad backend-opt",
			[]string{"connect", "ws-1", "--backend", "mock", "--token", "t", "--backend-opt", "noequals"},
			"KEY=VALUE",
		},
		{
			"unknown backend",
			[]string{"connect", "ws-1", "--backend", "nonesuch", "--token", "t"},
			"unknown backend",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Reset package-level connectFlags between runs so a prior test's
			// settings don't leak. cobra rebinds on Execute.
			resetConnectFlags()
			rootCmd.SetArgs(tc.args)
			err := rootCmd.Execute()
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Errorf("error %q missing %q", err.Error(), tc.wantSub)
			}
		})
	}
}

// TestConnect_DryRun covers the happy path: valid flags, --dry-run set,
// runner builds the backend and exits without entering loops.
func TestConnect_DryRun(t *testing.T) {
	resetConnectFlags()
	rootCmd.SetArgs([]string{
		"connect", "ws-test",
		"--backend", "mock",
		"--token", "tok",
		"--backend-opt", "reply=ok",
		"--dry-run",
	})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("dry-run should succeed: %v", err)
	}
}

func resetConnectFlags() {
	connectFlags.backend = "claude-code"
	connectFlags.backendOpts = nil
	connectFlags.token = ""
	connectFlags.mode = "poll"
	connectFlags.intervalMs = 1000
	connectFlags.sinceSecs = 30
	connectFlags.dryRun = false
}
