// Package cmd implements the CLI command tree.
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/Molecule-AI/molecule-cli/internal/client"
	"github.com/spf13/cobra"
)

// ---------------------------------------------------------------------------
// Workspace command group
// ---------------------------------------------------------------------------

var workspaceCmd = &cobra.Command{
	Use:   "workspace",
	Short: "Manage Molecule AI workspaces",
	Long:  `List, inspect, create, delete, restart, audit, and delegate to workspaces.`,
}

func init() {
	workspaceCmd.AddCommand(
		workspaceListCmd, workspaceCreateCmd, workspaceInspectCmd,
		workspaceDeleteCmd, workspaceRestartCmd, workspaceAuditCmd, workspaceDelegateCmd,
	)
}

// ===========================================================================
// mol workspace list
// ===========================================================================
var workspaceListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all workspaces",
	RunE:  runWorkspaceList,
}

func runWorkspaceList(cmd *cobra.Command, _ []string) error {
	cl := client.New(apiURL)
	ws, err := cl.ListWorkspaces()
	if err != nil {
		return fmt.Errorf("workspace list: %w", err)
	}
	if outputFormat == "json" || outputFormat == "yaml" {
		return printJSON(ws)
	}
	if len(ws) == 0 {
		fmt.Println("No workspaces found.")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tSTATUS\tROLE\tRUNTIME\tCREATED AT")
	for _, s := range ws {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			s.ID, s.Name, s.Status, s.Role, s.Runtime, s.CreatedAt)
	}
	return w.Flush()
}

// ===========================================================================
// mol workspace create
// ===========================================================================
var createFlags struct {
	name         string
	role         string
	runtime      string
	template     string
	parentID     string
	workspaceDir string
	tier         int
}

var workspaceCreateCmd = &cobra.Command{
	Use:   "create --name <name> [flags]",
	Short: "Create a new workspace",
	RunE:  runWorkspaceCreate,
}

func init() {
	f := workspaceCreateCmd.Flags()
	f.StringVarP(&createFlags.name, "name", "n", "", "Workspace name (required)")
	f.StringVar(&createFlags.role, "role", "", "Role (e.g. pm, report, researcher)")
	f.StringVar(&createFlags.runtime, "runtime", "", "Runtime (e.g. claude-code, deepagents)")
	f.StringVar(&createFlags.template, "template", "", "Template name or ID")
	f.StringVar(&createFlags.parentID, "parent-id", "", "Parent workspace ID")
	f.StringVar(&createFlags.workspaceDir, "workspace-dir", "", "Workspace directory path")
	f.IntVar(&createFlags.tier, "tier", 0, "Tier value")
	workspaceCreateCmd.MarkFlagRequired("name")
}

func runWorkspaceCreate(cmd *cobra.Command, _ []string) error {
	cl := client.New(apiURL)
	req := client.CreateWorkspaceRequest{Name: createFlags.name}
	if createFlags.role != "" {
		req.Role = createFlags.role
	}
	if createFlags.runtime != "" {
		req.Runtime = createFlags.runtime
	}
	if createFlags.template != "" {
		req.Template = createFlags.template
	}
	if createFlags.parentID != "" {
		req.ParentID = createFlags.parentID
	}
	if createFlags.workspaceDir != "" {
		req.WorkspaceDir = createFlags.workspaceDir
	}
	if createFlags.tier > 0 {
		req.Tier = createFlags.tier
	}
	ws, err := cl.CreateWorkspace(req)
	if err != nil {
		return fmt.Errorf("workspace create: %w", err)
	}
	if outputFormat == "json" || outputFormat == "yaml" {
		return printJSON(ws)
	}
	fmt.Printf("Workspace created: %s (%s)\n", ws.Name, ws.ID)
	return nil
}

// ===========================================================================
// mol workspace inspect
// ===========================================================================
var workspaceInspectCmd = &cobra.Command{
	Use:   "inspect <workspace-id>",
	Short: "Show full details for a workspace",
	Args:  cobra.ExactArgs(1),
	RunE:  runWorkspaceInspect,
}

func runWorkspaceInspect(cmd *cobra.Command, args []string) error {
	cl := client.New(apiURL)
	ws, err := cl.GetWorkspace(args[0])
	if err != nil {
		return fmt.Errorf("workspace inspect: %w", err)
	}
	if outputFormat == "json" || outputFormat == "yaml" {
		return printJSON(ws)
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	kv(w, "ID", ws.ID)
	kv(w, "Name", ws.Name)
	kv(w, "Status", ws.Status)
	kv(w, "Role", ws.Role)
	kv(w, "Runtime", ws.Runtime)
	kv(w, "Tier", fmt.Sprintf("%d", ws.Tier))
	kv(w, "ParentID", ws.ParentID)
	kv(w, "WorkspaceDir", ws.WorkspaceDir)
	kv(w, "CreatedAt", ws.CreatedAt)
	if ws.Canvas != nil {
		kv(w, "Canvas", fmt.Sprintf("(%.0f, %.0f)", ws.Canvas.X, ws.Canvas.Y))
	}
	return w.Flush()
}

// ===========================================================================
// mol workspace delete
// ===========================================================================
var workspaceDeleteCmd = &cobra.Command{
	Use:   "delete <workspace-id>",
	Short: "Delete a workspace (irreversible)",
	Args:  cobra.ExactArgs(1),
	RunE:  runWorkspaceDelete,
}

func runWorkspaceDelete(cmd *cobra.Command, args []string) error {
	cl := client.New(apiURL)
	if err := cl.DeleteWorkspace(args[0]); err != nil {
		return fmt.Errorf("workspace delete: %w", err)
	}
	fmt.Printf("Workspace %q deleted.\n", args[0])
	return nil
}

// ===========================================================================
// mol workspace restart
// ===========================================================================
var workspaceRestartCmd = &cobra.Command{
	Use:   "restart <workspace-id>",
	Short: "Restart a workspace",
	Args:  cobra.ExactArgs(1),
	RunE:  runWorkspaceRestart,
}

func runWorkspaceRestart(cmd *cobra.Command, args []string) error {
	cl := client.New(apiURL)
	if err := cl.RestartWorkspace(args[0]); err != nil {
		return fmt.Errorf("workspace restart: %w", err)
	}
	fmt.Printf("Restart triggered for workspace %q.\n", args[0])
	return nil
}

// ===========================================================================
// mol workspace audit
// ===========================================================================
var workspaceAuditCmd = &cobra.Command{
	Use:   "audit",
	Short: "Full workspace + agent audit report",
	RunE:  runWorkspaceAudit,
}

func runWorkspaceAudit(cmd *cobra.Command, _ []string) error {
	cl := client.New(apiURL)
	workspaces, agents, err := cl.AuditWorkspaces()
	if err != nil {
		return fmt.Errorf("workspace audit: %w", err)
	}
	type auditReport struct {
		Workspaces  int               `json:"workspaces"`
		Agents      int               `json:"agents"`
		ByStatus    map[string]int    `json:"by_status"`
		Items       []client.Workspace `json:"workspaces_list"`
		AgentList   []client.Agent     `json:"agents_list"`
	}
	byStatus := map[string]int{}
	for _, ws := range workspaces {
		byStatus[ws.Status]++
	}
	report := auditReport{
		Workspaces: len(workspaces),
		Agents:     len(agents),
		ByStatus:   byStatus,
		Items:      workspaces,
		AgentList:  agents,
	}
	if outputFormat == "json" || outputFormat == "yaml" {
		return printJSON(report)
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "WORKSPACES\t")
	fmt.Fprintln(w, "ID\tNAME\tSTATUS\tROLE\tRUNTIME")
	for _, ws := range workspaces {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			ws.ID, ws.Name, ws.Status, ws.Role, ws.Runtime)
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "AGENTS\t")
	fmt.Fprintln(w, "ID\tNAME\tWORKSPACE\tSTATUS\tMODEL")
	for _, a := range agents {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			a.ID, a.Name, a.WorkspaceID, a.Status, a.Model)
	}
	return w.Flush()
}

// ===========================================================================
// mol workspace delegate
// ===========================================================================
var workspaceDelegateCmd = &cobra.Command{
	Use:   "delegate <workspace-id> <target-workspace-id> <task>",
	Short: "Delegate a task to another workspace (non-blocking)",
	Args:  cobra.ExactArgs(3),
	RunE:  runWorkspaceDelegate,
}

func runWorkspaceDelegate(cmd *cobra.Command, args []string) error {
	workspaceID, targetID, task := args[0], args[1], args[2]
	cl := client.New(apiURL)

	type delReq struct {
		TargetID string `json:"target_id"`
		Task     string `json:"task"`
	}
	type delResp struct {
		DelegationID string `json:"delegation_id,omitempty"`
		Status       string `json:"status,omitempty"`
	}
	encoded, _ := json.Marshal(delReq{TargetID: targetID, Task: task})
	body, err := runHTTP("POST", cl.BaseURL+"/workspaces/"+workspaceID+"/delegate", encoded)
	if err != nil {
		return fmt.Errorf("workspace delegate: %w", err)
	}
	var resp delResp
	if err := json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("workspace delegate: parse response: %w", err)
	}
	if resp.DelegationID != "" {
		fmt.Printf("Delegation queued: %s (status: %s)\n", resp.DelegationID, resp.Status)
	} else {
		fmt.Printf("Delegation sent to %q.\n", targetID)
	}
	_ = workspaceID
	return nil
}