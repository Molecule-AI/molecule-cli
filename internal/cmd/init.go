package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// ---------------------------------------------------------------------------
// molecule init — bootstrap workspace setup
// ---------------------------------------------------------------------------

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Bootstrap workspace and scaffold a molecule.yaml config file",
	Long: `Scaffold a default molecule.yaml in the current directory.

This is the primary entry point for new users. Run once in a project
to create a configuration file that can be checked into version control.

All values can be overridden by environment variables
(MOLECULE_API_URL, MOLECULE_RUNTIME_URL, etc.).

After init, run 'molecule --config molecule.yaml workspace list' to verify your setup.`,
	RunE: runInit,
}

func runInit(cmd *cobra.Command, _ []string) error {
	cfgPath := "molecule.yaml"

	if _, err := os.Stat(cfgPath); err == nil {
		return fmt.Errorf("init: %s already exists — not overwriting (use --force to replace)", cfgPath)
	}

	content := `# molecule CLI configuration — https://github.com/Molecule-AI/molecule-cli
#
# All values can be overridden by environment variables:
#   MOLECULE_API_URL, MOLECULE_RUNTIME_URL, MOL_OUTPUT, MOL_VERBOSE, etc.
#
# Environment variables always take precedence over this file.

# Platform API base URL (env: MOLECULE_API_URL)
# api_url: https://api.molecule.ai

# Workspace runtime URL for dev/proxy mode (env: MOLECULE_RUNTIME_URL)
# runtime_url: https://runtime.molecule.ai

# Output format: table | json | yaml  (env: MOL_OUTPUT)
# output: table

# Verbose logging: true | false  (env: MOL_VERBOSE)
# verbose: false
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("init: write %s: %w", cfgPath, err)
	}

	absPath, _ := filepath.Abs(cfgPath)
	fmt.Printf("Scaffolded %s\n", absPath)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Edit molecule.yaml with your platform URL")
	fmt.Println("  2. Run molecule --config molecule.yaml workspace list")
	fmt.Println("  3. For full reference: molecule --help")
	return nil
}
