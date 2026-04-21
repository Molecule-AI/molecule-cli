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

var workspaceCmd = &cobra.Command{
	Use:   "workspace",
	Short: "Manage Molecule AI workspaces",
	Long:  `List, inspect, create, and delete workspaces on the Molecule AI platform.`,
}

func init() {
	workspaceCmd.AddCommand(workspaceListCmd, workspaceGetCmd)
}

// workspaceListCmd implements: molecule workspace list
var workspaceListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all workspaces",
	RunE:  runWorkspaceList,
}

func runWorkspaceList(cmd *cobra.Command, _ []string) error {
	cl := client.New(apiURL)
	workspaces, err := cl.ListWorkspaces()
	if err != nil {
		return fmt.Errorf("list workspaces: %w", err)
	}

	switch outputFormat {
	case "json":
		return printJSON(workspaces)
	case "yaml":
		return printYAML(workspaces)
	default: // table
		return printWorkspaceTable(workspaces)
	}
}

func printWorkspaceTable(workspaces []client.Workspace) error {
	if len(workspaces) == 0 {
		fmt.Println("No workspaces found.")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tSTATUS\tROLE\tRUNTIME\tCREATED AT")
	for _, ws := range workspaces {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			ws.ID, ws.Name, ws.Status, ws.Role, ws.Runtime, ws.CreatedAt)
	}
	return w.Flush()
}

// workspaceGetCmd implements: molecule workspace get <id>
var workspaceGetCmd = &cobra.Command{
	Use:   "get <workspace-id>",
	Short: "Show details for a workspace",
	Args:  cobra.ExactArgs(1),
	RunE:  runWorkspaceGet,
}

func runWorkspaceGet(cmd *cobra.Command, args []string) error {
	cl := client.New(apiURL)
	ws, err := cl.GetWorkspace(args[0])
	if err != nil {
		return fmt.Errorf("get workspace: %w", err)
	}

	switch outputFormat {
	case "json":
		return printJSON(ws)
	case "yaml":
		return printYAML(ws)
	default: // table
		w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintf(w, "ID:\t%s\n", ws.ID)
		fmt.Fprintf(w, "Name:\t%s\n", ws.Name)
		fmt.Fprintf(w, "Status:\t%s\n", ws.Status)
		fmt.Fprintf(w, "Role:\t%s\n", ws.Role)
		fmt.Fprintf(w, "Runtime:\t%s\n", ws.Runtime)
		fmt.Fprintf(w, "WorkspaceDir:\t%s\n", ws.WorkspaceDir)
		fmt.Fprintf(w, "CreatedAt:\t%s\n", ws.CreatedAt)
		if ws.Canvas != nil {
			fmt.Fprintf(w, "Canvas:\t(%.0f, %.0f)\n", ws.Canvas.X, ws.Canvas.Y)
		}
		return w.Flush()
	}
}

func printJSON(v interface{}) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func printYAML(v interface{}) error {
	// Simple YAML emitter without external deps: use json then prefix each line.
	// For production, replace with yaml.v3 Marshal.
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
