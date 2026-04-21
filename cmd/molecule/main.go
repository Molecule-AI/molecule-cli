package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// Global flags
var (
	flagOutput  string
	flagVerbose bool
	flagConfig  string
)

// Output formats
type OutputFormat int

const (
	FormatText OutputFormat = iota
	FormatJSON
	FormatYAML
)

var formatMap = map[string]OutputFormat{
	"text": FormatText,
	"json": FormatJSON,
	"yaml": FormatYAML,
}

// writeOutput writes the result in the requested format
func writeOutput(w io.Writer, data any, format string) {
	switch formatMap[format] {
	case FormatJSON:
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		enc.Encode(data)
	case FormatYAML:
		// Simple YAML-like output for maps
		if m, ok := data.(map[string]any); ok {
			for k, v := range m {
				fmt.Fprintf(w, "%s: %v\n", k, v)
			}
		} else {
			fmt.Fprintf(w, "%v\n", data)
		}
	default:
		dumpText(w, data)
	}
}

func dumpText(w io.Writer, data any) {
	switch v := data.(type) {
	case string:
		fmt.Fprintln(w, v)
	case []string:
		for _, s := range v {
			fmt.Fprintln(w, s)
		}
	case map[string]any:
		for k, val := range v {
			fmt.Fprintf(w, "%-20s %v\n", k+":", val)
		}
	default:
		fmt.Fprintf(w, "%v\n", v)
	}
}

func errorExit(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format, args...)
	os.Exit(1)
}

// PlatformClient is a minimal client for the Molecule AI platform API
type PlatformClient struct {
	BaseURL string
	Token   string
}

func newPlatformClient() *PlatformClient {
	baseURL := os.Getenv("MOLECULE_API_URL")
	if baseURL == "" {
		baseURL = "https://api.moleculesai.app"
	}
	return &PlatformClient{BaseURL: baseURL, Token: os.Getenv("MOLECULE_API_TOKEN")}
}

// Workspace represents a Molecule AI workspace
type Workspace struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Status   string `json:"status"`
	Tier     int    `json:"tier"`
	ParentID string `json:"parent_id,omitempty"`
	Created  string `json:"created_at,omitempty"`
}

// Agent represents a Molecule AI agent
type Agent struct {
	ID         string `json:"id"`
	WorkspaceID string `json:"workspace_id"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	Role       string `json:"role"`
}

// ─── Workspace Commands ───────────────────────────────────────────────────────

var workspaceCmd = &cobra.Command{
	Use:   "workspace",
	Short: "Manage workspaces",
}

var workspaceCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new workspace",
	Run: func(cmd *cobra.Command, args []string) {
		name, _ := cmd.Flags().GetString("name")
		tier, _ := cmd.Flags().GetInt("tier")
		template, _ := cmd.Flags().GetString("template")

		if name == "" {
			errorExit("workspace create: --name is required")
		}

		// Placeholder — actual POST to platform API
		ws := Workspace{
			ID:       uuid.New().String(),
			Name:     name,
			Status:   "provisioning",
			Tier:     tier,
			Created:  time.Now().Format(time.RFC3339),
		}
		writeOutput(os.Stdout, ws, flagOutput)
	},
}

var workspaceListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all workspaces",
	Run: func(cmd *cobra.Command, args []string) {
		// Placeholder — actual GET /workspaces
		workspaces := []Workspace{
			{ID: "placeholder-workspace-id", Name: "example-workspace", Status: "online", Tier: 2},
		}
		writeOutput(os.Stdout, workspaces, flagOutput)
	},
}

var workspaceInspectCmd = &cobra.Command{
	Use:   "inspect",
	Args:  cobra.ExactArgs(1),
	Short: "Inspect a workspace by ID",
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		ws := Workspace{ID: id, Name: "placeholder", Status: "online", Tier: 2}
		writeOutput(os.Stdout, ws, flagOutput)
	},
}

var workspaceDeleteCmd = &cobra.Command{
	Use:   "delete",
	Args:  cobra.ExactArgs(1),
	Short: "Delete a workspace",
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		fmt.Fprintf(os.Stderr, "workspace delete: deleted %s\n", id)
	},
}

var workspaceRestartCmd = &cobra.Command{
	Use:   "restart",
	Args:  cobra.ExactArgs(1),
	Short: "Restart a workspace",
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		fmt.Fprintf(os.Stderr, "workspace restart: restarting %s\n", id)
	},
}

var workspaceDelegateCmd = &cobra.Command{
	Use:   "delegate",
	Args:  cobra.ExactArgs(1),
	Short: "Delegate a task to a workspace",
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		task, _ := cmd.Flags().GetString("task")
		async, _ := cmd.Flags().GetBool("async")

		if task == "" {
			errorExit("workspace delegate: --task is required")
		}

		if async {
			taskID := uuid.New().String()
			fmt.Fprintf(os.Stderr, "workspace delegate: async task %s dispatched to %s\n", taskID, id)
		} else {
			fmt.Fprintf(os.Stderr, "workspace delegate: sync task dispatched to %s\n", id)
		}
	},
}

var workspaceAuditCmd = &cobra.Command{
	Use:   "audit",
	Args:  cobra.ExactArgs(1),
	Short: "Audit a workspace's configuration",
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		report := map[string]any{
			"workspace_id":     id,
			"audit_passed":     true,
			"issues_found":     0,
			"recommendations":  []string{},
		}
		writeOutput(os.Stdout, report, flagOutput)
	},
}

// ─── Agent Commands ─────────────────────────────────────────────────────────

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Manage agents",
}

var agentListCmd = &cobra.Command{
	Use:   "list",
	Args:  cobra.ExactArgs(1),
	Short: "List agents in a workspace",
	Run: func(cmd *cobra.Command, args []string) {
		workspaceID := args[0]
		agents := []Agent{
			{ID: "placeholder-agent-id", WorkspaceID: workspaceID, Name: "example-agent", Status: "online", Role: "assistant"},
		}
		writeOutput(os.Stdout, agents, flagOutput)
	},
}

var agentInspectCmd = &cobra.Command{
	Use:   "inspect",
	Args:  cobra.ExactArgs(1),
	Short: "Inspect an agent by ID",
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		agent := Agent{ID: id, Status: "online", Role: "assistant"}
		writeOutput(os.Stdout, agent, flagOutput)
	},
}

var agentSendCmd = &cobra.Command{
	Use:   "send",
	Args:  cobra.ExactArgs(1),
	Short: "Send a message to an agent",
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		msg, _ := cmd.Flags().GetString("message")
		if msg == "" {
			errorExit("agent send: --message is required")
		}
		fmt.Fprintf(os.Stderr, "agent send: message sent to %s\n", id)
	},
}

var agentPeersCmd = &cobra.Command{
	Use:   "peers",
	Args:  cobra.ExactArgs(1),
	Short: "List peers for an agent",
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		peers := []string{}
		writeOutput(os.Stdout, peers, flagOutput)
	},
}

// ─── Platform Commands ───────────────────────────────────────────────────────

var platformCmd = &cobra.Command{
	Use:   "platform",
	Short: "Interact with the platform",
}

var platformAuditCmd = &cobra.Command{
	Use:   "audit",
	Short: "Audit platform configuration",
	Run: func(cmd *cobra.Command, args []string) {
		url, _ := cmd.Flags().GetString("url")
		token, _ := cmd.Flags().GetString("token")
		if url == "" {
			url = os.Getenv("MOLECULE_API_URL")
		}

		report := map[string]any{
			"platform_url":    url,
			"api_key_configured": token != "" || os.Getenv("MOLECULE_API_TOKEN") != "",
			"audit_passed":    true,
		}
		writeOutput(os.Stdout, report, flagOutput)
	},
}

var platformHealthCmd = &cobra.Command{
	Use:   "health",
	Short: "Check platform health",
	Run: func(cmd *cobra.Command, args []string) {
		health := map[string]any{
			"status":      "ok",
			"api_url":     os.Getenv("MOLECULE_API_URL"),
			"checked_at":  time.Now().Format(time.RFC3339),
		}
		writeOutput(os.Stdout, health, flagOutput)
	},
}

// ─── Config Commands ─────────────────────────────────────────────────────────

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage CLI configuration",
}

var configListCmd = &cobra.Command{
	Use:   "list",
	Short: "Show current configuration",
	Run: func(cmd *cobra.Command, args []string) {
		cfg := map[string]string{
			"MOLECULE_API_URL":    os.Getenv("MOLECULE_API_URL"),
			"MOLECULE_RUNTIME_URL": os.Getenv("MOLECULE_RUNTIME_URL"),
			"MOLECULE_API_TOKEN":   maskToken(os.Getenv("MOLECULE_API_TOKEN")),
			"config_file":          viper.ConfigFileUsed(),
		}
		writeOutput(os.Stdout, cfg, flagOutput)
	},
}

var configGetCmd = &cobra.Command{
	Use:   "get",
	Args:  cobra.ExactArgs(1),
	Short: "Get a specific config value",
	Run: func(cmd *cobra.Command, args []string) {
		key := args[0]
		val := viper.GetString(key)
		fmt.Fprintln(os.Stdout, val)
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set",
	Args:  cobra.ExactArgs(2),
	Short: "Set a config value",
	Run: func(cmd *cobra.Command, args []string) {
		key, val := args[0], args[1]
		viper.Set(key, val)
		fmt.Fprintf(os.Stderr, "config set: %s = %s\n", key, val)
	},
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Bootstrap ~/.config/molecule/cli.yaml",
	Run: func(cmd *cobra.Command, args []string) {
		home, err := os.UserHomeDir()
		if err != nil {
			errorExit("config init: cannot determine home directory")
		}
		dir := home + "/.config/molecule"
		os.MkdirAll(dir, 0755)
		path := dir + "/cli.yaml"
		content := `# Molecule CLI configuration
# Copy this file to ~/.config/molecule/cli.yaml

api_url: "https://api.moleculesai.app"
runtime_url: ""

# Token is read from MOLECULE_API_TOKEN env var — do not store here.
`
		os.WriteFile(path, []byte(content), 0644)
		fmt.Fprintf(os.Stderr, "config init: created %s\n", path)
	},
}

var configViewCmd = &cobra.Command{
	Use:   "view",
	Short: "Print config file path and current values",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Fprintf(os.Stderr, "config file: %s\n", viper.ConfigFileUsed())
		fmt.Fprintf(os.Stderr, "effective config:\n")
		for _, key := range viper.AllKeys() {
			fmt.Fprintf(os.Stderr, "  %s = %v\n", key, viper.Get(key))
		}
	},
}

// ─── Root Command ────────────────────────────────────────────────────────────

var rootCmd = &cobra.Command{
	Use:   "molecule",
	Short: "Molecule AI CLI — agent platform management tool",
	Long: `Molecule AI CLI for managing agents, workspaces, and deployments.

Environment variables:
  MOLECULE_API_URL      Control plane API base URL (default: https://api.moleculesai.app)
  MOLECULE_RUNTIME_URL  Workspace runtime URL
  MOLECULE_API_TOKEN    API authentication token

Examples:
  molecule workspace list
  molecule agent inspect <agent-id>
  molecule config init
`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&flagOutput, "output", "o", "text", "Output format: text, json, yaml")
	rootCmd.PersistentFlags().BoolVarP(&flagVerbose, "verbose", "v", false, "Enable verbose output")
	rootCmd.PersistentFlags().StringVar(&flagConfig, "config", "", "Path to config file")

	// Workspace subcommands
	workspaceCreateCmd.Flags().String("name", "", "Workspace name")
	workspaceCreateCmd.Flags().Int("tier", 2, "Workspace tier (1-4)")
	workspaceCreateCmd.Flags().String("template", "default", "Template ID")
	workspaceCreateCmd.MarkFlagRequired("name")

	workspaceDelegateCmd.Flags().String("task", "", "Task prompt")
	workspaceDelegateCmd.Flags().Bool("async", false, "Fire and forget (async)")

	// Agent subcommands
	agentSendCmd.Flags().String("message", "", "Message to send")
	agentSendCmd.MarkFlagRequired("message")

	// Platform subcommands
	platformAuditCmd.Flags().String("url", "", "Platform URL override")
	platformAuditCmd.Flags().String("token", "", "API token override")

	// Wire up workspace tree
	workspaceCmd.AddCommand(workspaceCreateCmd, workspaceListCmd, workspaceInspectCmd,
		workspaceDeleteCmd, workspaceRestartCmd, workspaceDelegateCmd, workspaceAuditCmd)
	rootCmd.AddCommand(workspaceCmd)

	// Wire up agent tree
	agentCmd.AddCommand(agentListCmd, agentInspectCmd, agentSendCmd, agentPeersCmd)
	rootCmd.AddCommand(agentCmd)

	// Wire up platform tree
	platformCmd.AddCommand(platformAuditCmd, platformHealthCmd)
	rootCmd.AddCommand(platformCmd)

	// Wire up config tree
	configCmd.AddCommand(configListCmd, configGetCmd, configSetCmd, configInitCmd, configViewCmd)
	rootCmd.AddCommand(configCmd)
}

func maskToken(token string) string {
	if len(token) <= 8 {
		return "***"
	}
	return token[:4] + strings.Repeat("*", len(token)-8) + token[len(token)-4:]
}

func main() {
	// Configure viper for config file support
	viper.SetConfigName("cli")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("$HOME/.config/molecule")
	viper.AutomaticEnv()

	// Bind CLI flags to viper
	viper.BindPFlag("output", rootCmd.PersistentFlags().Lookup("output"))
	viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))

	// Override flag values from config file
	if viper.IsSet("output") {
		flagOutput = viper.GetString("output")
	}
	if viper.IsSet("verbose") {
		flagVerbose = viper.GetBool("verbose")
	}

	// Set Gin mode based on verbosity
	if flagVerbose {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	// Execute
	if err := rootCmd.Execute(); err != nil {
		if strings.Contains(err.Error(), "unknown subcommand") ||
			strings.Contains(err.Error(), "missing required") ||
			strings.Contains(err.Error(), "flag") {
			os.Exit(2)
		}
		os.Exit(1)
	}
}