// Package cmd implements the CLI command tree.
package cmd

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"text/tabwriter"

	"github.com/Molecule-AI/molecule-cli/internal/client"
	"github.com/spf13/cobra"
)

// ---------------------------------------------------------------------------
// Platform command group
// ---------------------------------------------------------------------------

var platformCmd = &cobra.Command{
	Use:   "platform",
	Short: "Platform-level operations",
	Long:  `Audit the platform, check health, and inspect raw API responses.`,
}

func init() {
	platformCmd.AddCommand(platformAuditCmd, platformHealthCmd)
}

// ===========================================================================
// mol platform audit
// ===========================================================================
var platformAuditCmd = &cobra.Command{
	Use:   "audit",
	Short: "Full platform audit: workspaces, agents, delegation summary",
	RunE:  runPlatformAudit,
}

func runPlatformAudit(cmd *cobra.Command, _ []string) error {
	cl := client.New(apiURL)
	workspaces, agents, err := cl.AuditWorkspaces()
	if err != nil {
		return fmt.Errorf("platform audit: %w", err)
	}

	delegationsByWS := map[string]int{}
	for _, ws := range workspaces {
		dels, err := cl.GetDelegations(ws.ID)
		if err == nil {
			delegationsByWS[ws.ID] = len(dels)
		}
	}

	type wsRow struct {
		ID, Name, Status, Role string
		AgentCount, DelegationCount int
	}
	byStatus := map[string]int{}
	for _, ws := range workspaces {
		byStatus[ws.Status]++
	}
	rows := make([]wsRow, 0, len(workspaces))
	for _, ws := range workspaces {
		ac := 0
		for _, a := range agents {
			if a.WorkspaceID == ws.ID {
				ac++
			}
		}
		rows = append(rows, wsRow{
			ID: ws.ID, Name: ws.Name, Status: ws.Status, Role: ws.Role,
			AgentCount: ac, DelegationCount: delegationsByWS[ws.ID],
		})
	}

	type audit struct {
		WorkspaceCount int               `json:"workspace_count"`
		AgentCount     int               `json:"agent_count"`
		ByStatus       map[string]int    `json:"by_status"`
		DelegationMap  map[string]int    `json:"delegations_by_workspace"`
		Rows           []wsRow           `json:"workspaces"`
		Agents         []client.Agent   `json:"agents"`
	}
	auditReport := audit{
		WorkspaceCount: len(workspaces),
		AgentCount:     len(agents),
		ByStatus:       byStatus,
		DelegationMap:  delegationsByWS,
		Rows:           rows,
		Agents:         agents,
	}

	if outputFormat == "json" {
		return printJSON(auditReport)
	}
	if outputFormat == "yaml" {
		return printYAML(auditReport)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintf(w, "=== Platform Audit (%d workspaces, %d agents) ===\n\n",
		len(workspaces), len(agents))
	fmt.Fprintln(w, "WORKSPACE\tSTATUS\tROLE\tAGENTS\tDELEGATIONS")
	for _, r := range rows {
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%d\n",
			r.Name, r.Status, r.Role, r.AgentCount, r.DelegationCount)
	}
	return w.Flush()
}

// ===========================================================================
// mol platform health
// ===========================================================================
var platformHealthCmd = &cobra.Command{
	Use:   "health",
	Short: "Check platform health and version",
	RunE:  runPlatformHealth,
}

func runPlatformHealth(cmd *cobra.Command, _ []string) error {
	cl := client.New(apiURL)
	h, err := cl.Health()
	if err != nil {
		// Fall back to raw check if /health 404s on older platforms.
		body, hErr := platformRawHealth(cl.BaseURL)
		if hErr != nil {
			return fmt.Errorf("platform health: %w (and /health fallback also failed: %v)", err, hErr)
		}
		fmt.Printf("Platform reachable at %s — raw status: %s\n", cl.BaseURL, string(body))
		return nil
	}
	if outputFormat == "json" {
		return printJSON(h)
	}
	if outputFormat == "yaml" {
		return printYAML(h)
	}
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	kv(w, "Status", h.Status)
	kv(w, "Version", h.Version)
	kv(w, "Uptime", h.Uptime)
	kv(w, "Database", h.Database)
	return w.Flush()
}

func platformRawHealth(baseURL string) ([]byte, error) {
	req, _ := http.NewRequest("GET", baseURL+"/health", nil)
	resp, err := httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}