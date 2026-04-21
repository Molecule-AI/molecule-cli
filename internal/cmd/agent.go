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
// Agent command group
// ---------------------------------------------------------------------------

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Inspect and interact with agents",
	Long:  `List agents, inspect individual agents, send messages, and discover peers.`,
}

func init() {
	agentCmd.AddCommand(
		agentListCmd, agentInspectCmd, agentSendCmd, agentPeersCmd,
	)
}

// ===========================================================================
// mol agent list
// ===========================================================================
var agentListCmd = &cobra.Command{
	Use:   "list [workspace-id]",
	Short: "List all agents (optionally filtered to one workspace)",
	Args:  cobra.RangeArgs(0, 1),
	RunE:  runAgentList,
}

func runAgentList(cmd *cobra.Command, args []string) error {
	cl := client.New(apiURL)
	var agents []client.Agent
	var err error
	if len(args) == 0 {
		agents, err = cl.ListAgents()
	} else {
		agents, err = cl.ListWorkspaceAgents(args[0])
	}
	if err != nil {
		return fmt.Errorf("agent list: %w", err)
	}
	if outputFormat == "json" || outputFormat == "yaml" {
		return printJSON(agents)
	}
	if len(agents) == 0 {
		fmt.Println("No agents found.")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tWORKSPACE\tSTATUS\tMODEL\tRUNTIME")
	for _, a := range agents {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			a.ID, a.Name, a.WorkspaceID, a.Status, a.Model, a.Runtime)
	}
	return w.Flush()
}

// ===========================================================================
// mol agent inspect
// ===========================================================================
var agentInspectCmd = &cobra.Command{
	Use:   "inspect <agent-id>",
	Short: "Show full details for an agent",
	Args:  cobra.ExactArgs(1),
	RunE:  runAgentInspect,
}

func runAgentInspect(cmd *cobra.Command, args []string) error {
	cl := client.New(apiURL)
	a, err := cl.GetAgent(args[0])
	if err != nil {
		return fmt.Errorf("agent inspect: %w", err)
	}
	if outputFormat == "json" || outputFormat == "yaml" {
		return printJSON(a)
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	kv(w, "ID", a.ID)
	kv(w, "Name", a.Name)
	kv(w, "WorkspaceID", a.WorkspaceID)
	kv(w, "Status", a.Status)
	kv(w, "Model", a.Model)
	kv(w, "Runtime", a.Runtime)
	kv(w, "CreatedAt", a.CreatedAt)
	return w.Flush()
}

// ===========================================================================
// mol agent send
// ===========================================================================
var agentSendCmd = &cobra.Command{
	Use:   "send <agent-id> <message>",
	Short: "Send a one-shot message to an agent via A2A",
	Args:  cobra.ExactArgs(2),
	RunE:  runAgentSend,
}

func runAgentSend(cmd *cobra.Command, args []string) error {
	agentID, message := args[0], args[1]
	cl := client.New(apiURL)

	a, err := cl.GetAgent(agentID)
	if err != nil {
		return fmt.Errorf("agent send: %w", err)
	}
	wsID := a.WorkspaceID
	if wsID == "" {
		return fmt.Errorf("agent send: workspace_id unknown for agent %q", agentID)
	}

	type a2aReq struct {
		AgentID string `json:"agent_id"`
		Message string `json:"message"`
	}
	type a2aResp struct {
		Result string `json:"result,omitempty"`
		Error  string `json:"error,omitempty"`
	}
	encoded, _ := json.Marshal(a2aReq{AgentID: agentID, Message: message})
	body, err := runHTTP("POST", cl.BaseURL+"/workspaces/"+wsID+"/a2a", encoded)
	if err != nil {
		return fmt.Errorf("agent send: %w", err)
	}
	var resp a2aResp
	if err := json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("agent send: parse response: %w", err)
	}
	if resp.Error != "" {
		return fmt.Errorf("agent send: platform error: %s", resp.Error)
	}
	fmt.Println(resp.Result)
	return nil
}

// ===========================================================================
// mol agent peers
// ===========================================================================
var agentPeersCmd = &cobra.Command{
	Use:   "peers <workspace-id>",
	Short: "List peer workspaces reachable from a workspace",
	Args:  cobra.ExactArgs(1),
	RunE:  runAgentPeers,
}

func runAgentPeers(cmd *cobra.Command, args []string) error {
	cl := client.New(apiURL)
	peers, err := cl.GetPeers(args[0])
	if err != nil {
		return fmt.Errorf("agent peers: %w", err)
	}
	if outputFormat == "json" || outputFormat == "yaml" {
		return printJSON(peers)
	}
	if len(peers) == 0 {
		fmt.Println("No peers found.")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tWORKSPACE\tSTATUS\tMODEL")
	for _, p := range peers {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			p.ID, p.Name, p.WorkspaceID, p.Status, p.Model)
	}
	return w.Flush()
}