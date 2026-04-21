// Package cmd implements the CLI command tree.
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// ---------------------------------------------------------------------------
// Config command group
// ---------------------------------------------------------------------------

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "View and manage CLI and workspace configuration",
	Long: `mol config list      — list all config keys (from file + env)
mol config get <key> — print a single config value
mol config set <key> <value> — write a key to the config file
mol config init      — scaffold a default mol.yaml in the current directory
mol config view      — print the current config file with sources annotated`,
}

func init() {
	configCmd.AddCommand(
		configListCmd, configGetCmd, configSetCmd, configInitCmd, configViewCmd,
	)
}

// ===========================================================================
// mol config list
// ===========================================================================
var configListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all known config keys and their effective values",
	RunE:  runConfigList,
}

func runConfigList(cmd *cobra.Command, _ []string) error {
	settings := viper.AllSettings()
	if len(settings) == 0 {
		fmt.Println("No config keys set. Use `mol config set <key> <value>` or set env vars.")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "KEY\tVALUE\tSOURCE")
	for k, v := range settings {
		source := "default"
		if viper.InConfig(k) {
			source = "file"
		}
		if strings.HasPrefix(k, "MOLECULE_") || strings.HasPrefix(k, "MOL_") {
			source = "env"
		}
		fmt.Fprintf(w, "%s\t%v\t%s\n", k, v, source)
	}
	return w.Flush()
}

// ===========================================================================
// mol config get
// ===========================================================================
var configGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Print the effective value of a config key",
	Args:  cobra.ExactArgs(1),
	RunE:  runConfigGet,
}

func runConfigGet(cmd *cobra.Command, args []string) error {
	if !viper.IsSet(args[0]) {
		return fmt.Errorf("config get: key %q not set (check env var MOLECULE_%s)", args[0], args[0])
	}
	fmt.Println(viper.GetString(args[0]))
	return nil
}

// ===========================================================================
// mol config set
// ===========================================================================
var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Write a config key to the config file (~/.config/mol.yaml)",
	Args:  cobra.ExactArgs(2),
	RunE:  runConfigSet,
}

func runConfigSet(cmd *cobra.Command, args []string) error {
	key, value := args[0], args[1]
	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = "."
	}
	configFile := filepath.Join(configDir, "mol.yaml")

	v := viper.New()
	v.SetConfigFile(configFile)
	_ = v.ReadInConfig() // ignore not-found
	v.Set(key, value)
	if err := v.WriteConfig(); err != nil {
		if err2 := v.SafeWriteConfig(); err2 != nil {
			return fmt.Errorf("config set: write %s: %w (tried WriteConfig then SafeWriteConfig)", configFile, err)
		}
	}
	fmt.Printf("Set %s=%q in %s\n", key, value, v.ConfigFileUsed())
	return nil
}

// ===========================================================================
// mol config init
// ===========================================================================
var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Scaffold a default mol.yaml in the current directory",
	RunE:  runConfigInit,
}

func runConfigInit(cmd *cobra.Command, _ []string) error {
	const defaultConfig = `# mol CLI config — https://github.com/Molecule-AI/molecule-cli
#
# All values can be overridden by environment variables:
#   MOLECULE_API_URL, MOLECULE_RUNTIME_URL, MOL_OUTPUT, MOL_VERBOSE, etc.

# Platform API base URL (env: MOLECULE_API_URL)
# api_url: http://localhost:8080

# Output format: table | json | yaml  (env: MOL_OUTPUT)
# output: table

# Verbose logging: true | false  (env: MOL_VERBOSE)
# verbose: false
`
	if _, err := os.Stat("mol.yaml"); err == nil {
		return fmt.Errorf("config init: mol.yaml already exists (not overwriting)")
	}
	if err := os.WriteFile("mol.yaml", []byte(defaultConfig), 0o644); err != nil {
		return fmt.Errorf("config init: write mol.yaml: %w", err)
	}
	fmt.Println("Scaffolded mol.yaml — edit it and run mol --config mol.yaml, or move it to ~/.config/mol.yaml")
	return nil
}

// ===========================================================================
// mol config view
// ===========================================================================
var configViewCmd = &cobra.Command{
	Use:   "view",
	Short: "Print the current config file with sources annotated",
	RunE:  runConfigView,
}

func runConfigView(cmd *cobra.Command, _ []string) error {
	if viper.ConfigFileUsed() == "" {
		fmt.Println("No config file in use. Set one with --config or mol config init.")
		fmt.Println("\nActive env vars starting with MOLECULE_ or MOL_:")
		for _, env := range os.Environ() {
			if strings.HasPrefix(env, "MOLECULE_") || strings.HasPrefix(env, "MOL_") {
				fmt.Println("  ", env)
			}
		}
		return nil
	}
	data, err := os.ReadFile(viper.ConfigFileUsed())
	if err != nil {
		return fmt.Errorf("config view: read %s: %w", viper.ConfigFileUsed(), err)
	}
	fmt.Printf("# Config file: %s\n\n", viper.ConfigFileUsed())
	fmt.Print(string(data))
	return nil
}