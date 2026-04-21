// Package cmd implements the CLI command tree.
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Version is set at build time via -ldflags.
var Version = "dev"

// Global flags.
var (
	verbose      bool
	outputFormat string
	configPath   string
	apiURL       string
)

// rootCmd is the top-level molecule command.
var rootCmd = &cobra.Command{
	Use:     "mol",
	Version: Version,
	Short:   "mol — Molecule AI platform CLI",
	Long: `mol is the CLI for the Molecule AI agent platform.

Manage workspaces, inspect agents, audit the platform, and configure
agent behaviour from the terminal.

Quick start:
  mol workspace list
  mol agent list
  mol platform health`,
	SilenceUsage:   true,
	SilenceErrors:  true,
}

func init() {
	rootCmd.PersistentFlags().StringVar(&apiURL, "api-url",
		envOr("MOLECULE_API_URL", "http://localhost:8080"),
		"Platform API base URL (env: MOLECULE_API_URL)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false,
		"Enable verbose (DEBUG-level) output to stderr")
	rootCmd.PersistentFlags().StringVarP(&outputFormat, "output", "o", "table",
		"Output format: table | json | yaml")
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "",
		"Path to config file (default ~/.config/mol.yaml or ./mol.yaml)")
	rootCmd.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		return &exitError{code: 2, msg: err.Error()}
	})
	rootCmd.SetErr(os.Stderr)
}

// Execute runs the CLI.
func Execute() error {
	// Load config file.
	if configPath != "" {
		viper.SetConfigFile(configPath)
	} else {
		viper.SetConfigName("mol")
		viper.AddConfigPath("$HOME/.config")
		viper.AddConfigPath(".")
	}
	viper.AutomaticEnv()
	_ = viper.ReadInConfig() // ignore not-found; env vars win

	return rootCmd.Execute()
}

// envOr returns the value of env var key, or fallback if unset/empty.
func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// init registers all subcommand trees.
func init() {
	rootCmd.AddCommand(workspaceCmd)
	rootCmd.AddCommand(agentCmd)
	rootCmd.AddCommand(platformCmd)
	rootCmd.AddCommand(configCmd)
}

// exitError wraps a user-facing error with a specific exit code.
type exitError struct{ code int; msg string }

func (e *exitError) Error() string { return e.msg }

// handleErr converts an error to the right exit code.
func handleErr(err error) error {
	if err == nil {
		return nil
	}
	if ee, ok := err.(*exitError); ok {
		fmt.Fprintf(os.Stderr, "%s\n", ee.msg)
		os.Exit(ee.code)
	}
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(1)
	return nil
}

// printJSON writes v as JSON to stdout.
func printJSON(v interface{}) error {
	return json.NewEncoder(os.Stdout).Encode(v)
}

// kv writes a key-value pair to the tabwriter (only if v is non-empty).
func kv(w *tabwriter.Writer, k, v string) {
	if v == "" {
		return
	}
	fmt.Fprintf(w, "%s:\t%s\n", k, v)
}

func versionInfo() string {
	return fmt.Sprintf("mol %s (go %s)", Version, runtime.Version())
}