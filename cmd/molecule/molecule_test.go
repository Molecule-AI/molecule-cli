package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// mockServer returns an httptest.Server that handles molecule-cli API calls.
// It serves responses under basePath so tests can hit <server.URL + basePath>.
func mockServer(t *testing.T, basePath string) *httptest.Server {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)

	// --- Workspaces ---
	workspaces := []map[string]interface{}{
		{
			"id":         "ws-001",
			"name":       "test-workspace",
			"status":     "online",
			"role":       "researcher",
			"runtime":    "claude-code",
			"created_at": "2026-04-01T12:00:00Z",
			"tier":       2,
		},
		{
			"id":     "ws-002",
			"name":   "prod-workspace",
			"status": "online",
			"role":   "pm",
			"tier":   3,
		},
	}

	mux.HandleFunc(basePath+"/workspaces", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(workspaces)
		case http.MethodPost:
			var req map[string]interface{}
			json.NewDecoder(r.Body).Decode(&req)
			resp := map[string]interface{}{
				"id":         "ws-new",
				"name":       req["name"],
				"status":     "creating",
				"created_at": "2026-04-21T00:00:00Z",
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(resp)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc(basePath+"/workspaces/ws-001", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(workspaces[0])
		case http.MethodDelete:
			// CLI may send ?confirm=true query param
			w.WriteHeader(http.StatusNoContent)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc(basePath+"/workspaces/ws-missing", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		http.Error(w, `{"error":"workspace not found"}`, http.StatusNotFound)
	})

	mux.HandleFunc(basePath+"/workspaces/ws-001/restart", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc(basePath+"/workspaces/ws-001/delete", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc(basePath+"/workspaces/ws-001/agents", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		agents := []map[string]interface{}{
			{"id": "ag-001", "name": "researcher-agent", "workspace_id": "ws-001", "status": "online", "model": "claude-opus-4"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(agents)
	})

	mux.HandleFunc(basePath+"/workspaces/ws-001/delegate", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		resp := map[string]interface{}{
			"delegation_id": "del-001",
			"status":        "queued",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(resp)
	})

	// --- Agents ---
	agents := []map[string]interface{}{
		{"id": "ag-001", "name": "researcher-agent", "workspace_id": "ws-001", "status": "online", "model": "claude-opus-4"},
		{"id": "ag-002", "name": "pm-agent", "workspace_id": "ws-002", "status": "online", "model": "claude-sonnet-4"},
	}

	mux.HandleFunc(basePath+"/agents", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(agents)
	})

	mux.HandleFunc(basePath+"/agents/ag-001", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(agents[0])
	})

	mux.HandleFunc(basePath+"/agents/ag-missing", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"agent not found"}`, http.StatusNotFound)
	})

	mux.HandleFunc(basePath+"/registry/ws-001/peers", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		peers := []map[string]interface{}{
			{"id": "ws-002", "name": "prod-workspace", "workspace_id": "ws-002", "status": "online", "model": "claude-sonnet-4"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(peers)
	})

	mux.HandleFunc(basePath+"/workspaces/ws-001/a2a", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req map[string]string
		json.NewDecoder(r.Body).Decode(&req)
		resp := map[string]interface{}{
			"result": "Message delivered to researcher-agent: " + req["message"],
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	// --- Health ---
	mux.HandleFunc(basePath+"/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		resp := map[string]interface{}{
			"status":  "ok",
			"version": "1.2.3",
			"uptime":  "42h",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	return server
}

// repoRoot returns the repo root directory.
func repoRoot() string {
	// test file is at cmd/molecule/molecule_test.go
	// ../.. from molecule/ goes to cmd, ../../.. goes to clone-cli
	return filepath.Dir(filepath.Dir(filepath.Dir("/workspace/repos/clone-cli/cmd/molecule/")))
}

// mol returns the path to the CLI binary, building it if needed.
func mol(t *testing.T) string {
	root := repoRoot()
	exe := filepath.Join(t.TempDir(), "mol")
	cmd := exec.Command("/tmp/go/bin/go", "build", "-o", exe, "./cmd/molecule")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build ./cmd/molecule: %v\n%s", err, out)
	}
	return exe
}

// TestMain exists so we can skip tests when go build fails outside of normal circumstances.
// The real test logic is in the functions below.
func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

func TestCLI_Help(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		stderr bool // expect no stderr output on success
	}{
		{"root help", []string{"--help"}, false},
		{"workspace help", []string{"workspace", "--help"}, false},
		{"agent help", []string{"agent", "--help"}, false},
		{"platform help", []string{"platform", "--help"}, false},
		{"config help", []string{"config", "--help"}, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			exe := mol(t)
			root := repoRoot()
			cmd := exec.Command(exe, tc.args...)
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			cmd.Dir = root
			err := cmd.Run()
			if err != nil {
				t.Fatalf("mol %v: %v\nstdout: %s\nstderr: %s", strings.Join(tc.args, " "), err, stdout.String(), stderr.String())
			}
			if stderr.Len() > 0 && tc.stderr {
				t.Errorf("unexpected stderr:\n%s", stderr.String())
			}
			out := stdout.String()
			if out == "" {
				t.Errorf("empty stdout for mol %v", strings.Join(tc.args, " "))
			}
		})
	}
}

func TestCLI_Version(t *testing.T) {
	exe := mol(t)
	root := repoRoot()
	cmd := exec.Command(exe, "--version")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Dir = root
	err := cmd.Run()
	if err != nil {
		t.Fatalf("mol --version: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "mol") {
		t.Errorf("expected 'mol' in version output, got: %s", out)
	}
}

func TestCLI_WorkspaceList(t *testing.T) {
	server := mockServer(t, "")
	defer server.Close()

	exe := mol(t)
	root := repoRoot()
	cmd := exec.Command(exe, "--api-url", server.URL, "workspace", "list")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Dir = root
	err := cmd.Run()
	if err != nil {
		t.Fatalf("mol workspace list: %v\nstderr: %s", err, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "test-workspace") {
		t.Errorf("expected 'test-workspace' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "prod-workspace") {
		t.Errorf("expected 'prod-workspace' in output, got:\n%s", out)
	}
}

func TestCLI_WorkspaceList_JSON(t *testing.T) {
	server := mockServer(t, "")
	defer server.Close()

	exe := mol(t)
	root := repoRoot()
	cmd := exec.Command(exe, "--api-url", server.URL, "--output", "json", "workspace", "list")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Dir = root
	err := cmd.Run()
	if err != nil {
		t.Fatalf("mol workspace list --output json: %v\nstderr: %s", err, stderr.String())
	}
	var out []map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("non-JSON output: %s\nstderr: %s", stdout.String(), stderr.String())
	}
	if len(out) != 2 {
		t.Errorf("expected 2 workspaces, got %d", len(out))
	}
}

func TestCLI_WorkspaceInspect(t *testing.T) {
	server := mockServer(t, "")
	defer server.Close()

	exe := mol(t)
	root := repoRoot()
	cmd := exec.Command(exe, "--api-url", server.URL, "workspace", "inspect", "ws-001")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Dir = root
	err := cmd.Run()
	if err != nil {
		t.Fatalf("mol workspace inspect ws-001: %v\nstderr: %s", err, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{"ws-001", "test-workspace", "online", "researcher"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}

func TestCLI_WorkspaceInspect_NotFound(t *testing.T) {
	server := mockServer(t, "")
	defer server.Close()

	exe := mol(t)
	root := repoRoot()
	cmd := exec.Command(exe, "--api-url", server.URL, "workspace", "inspect", "ws-missing")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Dir = root
	err := cmd.Run()
	if err == nil {
		t.Fatalf("expected error for missing workspace, got none")
	}
	// Should exit with non-zero code
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected *exec.ExitError, got %T", err)
	}
	if exitErr.ExitCode() == 0 {
		t.Errorf("expected non-zero exit code for missing workspace, got 0")
	}
}

func TestCLI_WorkspaceCreate(t *testing.T) {
	server := mockServer(t, "")
	defer server.Close()

	exe := mol(t)
	root := repoRoot()
	cmd := exec.Command(exe, "--api-url", server.URL, "workspace", "create", "--name", "my-workspace")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Dir = root
	err := cmd.Run()
	if err != nil {
		t.Fatalf("mol workspace create: %v\nstderr: %s", err, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "my-workspace") {
		t.Errorf("expected 'my-workspace' in output, got:\n%s", out)
	}
}

func TestCLI_WorkspaceCreate_MissingName(t *testing.T) {
	server := mockServer(t, "")
	defer server.Close()

	exe := mol(t)
	root := repoRoot()
	cmd := exec.Command(exe, "--api-url", server.URL, "workspace", "create")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Dir = root
	err := cmd.Run()
	// Missing required flag should exit with non-zero code
	if err == nil {
		t.Fatalf("expected error for missing --name, got none")
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected *exec.ExitError, got %T", err)
	}
	if exitErr.ExitCode() == 0 {
		t.Errorf("expected non-zero exit code for missing required flag, got 0")
	}
}

func TestCLI_WorkspaceDelete(t *testing.T) {
	server := mockServer(t, "")
	defer server.Close()

	exe := mol(t)
	root := repoRoot()
	cmd := exec.Command(exe, "--api-url", server.URL, "workspace", "delete", "ws-001")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Dir = root
	err := cmd.Run()
	if err != nil {
		t.Fatalf("mol workspace delete ws-001: %v\nstderr: %s", err, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "deleted") {
		t.Errorf("expected 'deleted' in output, got:\n%s", out)
	}
}

func TestCLI_WorkspaceRestart(t *testing.T) {
	server := mockServer(t, "")
	defer server.Close()

	exe := mol(t)
	root := repoRoot()
	cmd := exec.Command(exe, "--api-url", server.URL, "workspace", "restart", "ws-001")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Dir = root
	err := cmd.Run()
	if err != nil {
		t.Fatalf("mol workspace restart ws-001: %v\nstderr: %s", err, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "Restart") {
		t.Errorf("expected 'Restart' in output, got:\n%s", out)
	}
}

func TestCLI_WorkspaceAudit(t *testing.T) {
	server := mockServer(t, "")
	defer server.Close()

	exe := mol(t)
	root := repoRoot()
	cmd := exec.Command(exe, "--api-url", server.URL, "workspace", "audit")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Dir = root
	err := cmd.Run()
	if err != nil {
		t.Fatalf("mol workspace audit: %v\nstderr: %s", err, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{"test-workspace", "prod-workspace", "researcher-agent"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}

func TestCLI_WorkspaceDelegate(t *testing.T) {
	server := mockServer(t, "")
	defer server.Close()

	exe := mol(t)
	root := repoRoot()
	cmd := exec.Command(exe, "--api-url", server.URL, "workspace", "delegate", "ws-001", "ws-002", "do the thing")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Dir = root
	err := cmd.Run()
	if err != nil {
		t.Fatalf("mol workspace delegate: %v\nstderr: %s", err, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "Delegation") && !strings.Contains(out, "ws-002") {
		t.Errorf("expected delegation confirmation in output, got:\n%s", out)
	}
}

func TestCLI_AgentList(t *testing.T) {
	server := mockServer(t, "")
	defer server.Close()

	exe := mol(t)
	root := repoRoot()
	cmd := exec.Command(exe, "--api-url", server.URL, "agent", "list")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Dir = root
	err := cmd.Run()
	if err != nil {
		t.Fatalf("mol agent list: %v\nstderr: %s", err, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{"researcher-agent", "pm-agent"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}

func TestCLI_AgentList_WorkspaceFiltered(t *testing.T) {
	server := mockServer(t, "")
	defer server.Close()

	exe := mol(t)
	root := repoRoot()
	cmd := exec.Command(exe, "--api-url", server.URL, "agent", "list", "ws-001")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Dir = root
	err := cmd.Run()
	if err != nil {
		t.Fatalf("mol agent list ws-001: %v\nstderr: %s", err, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "researcher-agent") {
		t.Errorf("expected 'researcher-agent' in output, got:\n%s", out)
	}
}

func TestCLI_AgentInspect(t *testing.T) {
	server := mockServer(t, "")
	defer server.Close()

	exe := mol(t)
	root := repoRoot()
	cmd := exec.Command(exe, "--api-url", server.URL, "agent", "inspect", "ag-001")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Dir = root
	err := cmd.Run()
	if err != nil {
		t.Fatalf("mol agent inspect ag-001: %v\nstderr: %s", err, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{"ag-001", "researcher-agent", "online"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}

func TestCLI_AgentInspect_NotFound(t *testing.T) {
	server := mockServer(t, "")
	defer server.Close()

	exe := mol(t)
	root := repoRoot()
	cmd := exec.Command(exe, "--api-url", server.URL, "agent", "inspect", "ag-missing")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Dir = root
	err := cmd.Run()
	if err == nil {
		t.Fatalf("expected error for missing agent, got none")
	}
}

func TestCLI_AgentSend(t *testing.T) {
	server := mockServer(t, "")
	defer server.Close()

	exe := mol(t)
	root := repoRoot()
	cmd := exec.Command(exe, "--api-url", server.URL, "agent", "send", "ag-001", "hello world")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Dir = root
	err := cmd.Run()
	if err != nil {
		t.Fatalf("mol agent send: %v\nstderr: %s", err, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "hello world") && !strings.Contains(out, "delivered") {
		t.Errorf("expected delivery confirmation in output, got:\n%s", out)
	}
}

func TestCLI_AgentPeers(t *testing.T) {
	server := mockServer(t, "")
	defer server.Close()

	exe := mol(t)
	root := repoRoot()
	cmd := exec.Command(exe, "--api-url", server.URL, "agent", "peers", "ws-001")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Dir = root
	err := cmd.Run()
	if err != nil {
		t.Fatalf("mol agent peers ws-001: %v\nstderr: %s", err, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "prod-workspace") {
		t.Errorf("expected 'prod-workspace' in peers output, got:\n%s", out)
	}
}

func TestCLI_PlatformHealth(t *testing.T) {
	server := mockServer(t, "")
	defer server.Close()

	exe := mol(t)
	root := repoRoot()
	cmd := exec.Command(exe, "--api-url", server.URL, "platform", "health")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Dir = root
	err := cmd.Run()
	if err != nil {
		t.Fatalf("mol platform health: %v\nstderr: %s", err, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "ok") && !strings.Contains(out, "1.2.3") {
		t.Errorf("expected health info in output, got:\n%s", out)
	}
}

func TestCLI_PlatformAudit(t *testing.T) {
	server := mockServer(t, "")
	defer server.Close()

	exe := mol(t)
	root := repoRoot()
	cmd := exec.Command(exe, "--api-url", server.URL, "platform", "audit")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Dir = root
	err := cmd.Run()
	if err != nil {
		t.Fatalf("mol platform audit: %v\nstderr: %s", err, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "test-workspace") || !strings.Contains(out, "prod-workspace") {
		t.Errorf("expected workspaces in platform audit output, got:\n%s", out)
	}
}

func TestCLI_UnknownSubcommand(t *testing.T) {
	exe := mol(t)
	root := repoRoot()
	cmd := exec.Command(exe, "agen", "inspect")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Dir = root
	err := cmd.Run()
	if err == nil {
		t.Fatalf("expected error for unknown subcommand, got none")
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected *exec.ExitError, got %T", err)
	}
	if exitErr.ExitCode() == 0 {
		t.Errorf("expected non-zero exit code for unknown subcommand, got 0")
	}
}

func TestCLI_MissingRequiredArg(t *testing.T) {
	server := mockServer(t, "")
	defer server.Close()

	exe := mol(t)
	root := repoRoot()
	cmd := exec.Command(exe, "--api-url", server.URL, "agent", "inspect")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Dir = root
	err := cmd.Run()
	if err == nil {
		t.Fatalf("expected error for missing required arg, got none")
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected *exec.ExitError, got %T", err)
	}
	if exitErr.ExitCode() == 0 {
		t.Errorf("expected non-zero exit code for missing required arg, got 0")
	}
}

func TestCLI_ConfigInit(t *testing.T) {
	exe := mol(t)
	dir := t.TempDir()
	cmd := exec.Command(exe, "config", "init")
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		t.Fatalf("mol config init: %v\nstderr: %s", err, stderr.String())
	}
	f := filepath.Join(dir, "mol.yaml")
	if _, err := os.Stat(f); err != nil {
		t.Errorf("mol.yaml not scaffolded at %s", f)
	}
}

func TestCLI_ConfigList(t *testing.T) {
	server := mockServer(t, "")
	defer server.Close()

	exe := mol(t)
	root := repoRoot()
	cmd := exec.Command(exe, "--api-url", server.URL, "config", "list")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Dir = root
	err := cmd.Run()
	if err != nil {
		t.Fatalf("mol config list: %v\nstderr: %s", err, stderr.String())
	}
	// Should at least show something without crashing
	out := stdout.String()
	if out == "" {
		t.Errorf("empty stdout for mol config list")
	}
}
