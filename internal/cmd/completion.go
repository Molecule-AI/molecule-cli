package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

// completionCmd represents the shell completion subcommand.
// Cobra v1.10+ generates completions for bash, zsh, fish, and PowerShell.
var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate shell completion scripts",
	Long: `Generate shell completion scripts for molecule.

Cobra automatically generates completions for:

  Bash — completions for bash (~/.bashrc or ~/.bash_completion)
  Zsh  — completions for zsh (usually ~/.zshrc)
  Fish — completions for fish (~/.config/fish/completions)
  PowerShell — completions for PowerShell (profile)

Examples:

  # Bash (add to ~/.bashrc or ~/.bash_completion)
  source <(molecule completion bash)

  # Zsh (add to ~/.zshrc)
  autoload -U compinit && compinit
  autoload -Uz bashcompinit && bashcompinit
  source <(molecule completion zsh)
  compdef _molecule molecule

  # Fish
  molecule completion fish | source

  # PowerShell (add to $PROFILE)
  molecule completion powershell | Out-String | Invoke-Expression
`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.ExactValidArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			return rootCmd.GenBashCompletion(os.Stdout)
		case "zsh":
			return rootCmd.GenZshCompletion(os.Stdout)
		case "fish":
			return rootCmd.GenFishCompletion(os.Stdout, true)
		case "powershell":
			return rootCmd.GenPowerShellCompletionWithDesc(os.Stdout)
		}
		return nil // unreachable
	},
}

func init() {
	rootCmd.AddCommand(completionCmd)
}
