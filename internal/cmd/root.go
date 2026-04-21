// Package cmd implements the CLI command tree.
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// Version is set at build time via -ldflags.
var Version = "dev"

// apiURL is the platform API base URL, configurable via --api-url flag or
// MOLECULE_API_URL env var.
var apiURL string

func init() {
	// Bind MOLECULE_API_URL env var so flag → env var → default chaining works.
	rootCmd.PersistentFlags().StringVar(&apiURL, "api-url",
		envOr("MOLECULE_API_URL", "http://localhost:8080"),
		"Base URL of the Molecule AI platform API")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false,
		"Enable verbose (DEBUG-level) output")
	rootCmd.PersistentFlags().StringVarP(&outputFormat, "output", "o", "table",
		"Output format: table, json, yaml")
}

// rootCmd is the top-level molecule command.
var rootCmd = &cobra.Command{
	Use:   "molecule",
	Short: "molecule — manage Molecule AI workspaces and agents",
	Long: `molecule is the CLI for the Molecule AI agent platform.

Manage workspaces, inspect agents, configure deployments, and interact
with the platform control plane.

Start by listing available workspaces:

    molecule workspace list`,
	Version: Version,
	PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
		// Lazy-validate that the API URL is reachable.
		if apiURL == "" {
			return fmt.Errorf("--api-url or MOLECULE_API_URL must be set")
		}
		return nil
	},
}

// verbose controls debug log output.
var verbose bool

// outputFormat controls the output format for list/inspect commands.
var outputFormat string

// Execute runs the CLI.
func Execute() error {
	return rootCmd.Execute()
}

// envOr returns the value of env var key, or fallback if unset/empty.
func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func init() {
	// Register subcommand trees.
	rootCmd.AddCommand(workspaceCmd)
}
