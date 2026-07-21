package cmd

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

const version = "0.2.5"

// banner is printed on every command invocation
const banner = `
   █████╗  ██╗   ██╗ ██╗   ██╗ ██╗ ███████╗
  ██╔══██╗ ██║   ██║ ██║   ██║ ██║ ██╔════╝
  ███████║ ██║   ██║ ██║   ██║ ██║ ███████╗
  ██╔══██║ ██║   ██║ ╚██╗ ██╔╝ ██║ ╚════██║
  ██║  ██║ ╚██████╔╝  ╚████╔╝  ██║ ███████║
  ╚═╝  ╚═╝  ╚═════╝    ╚═══╝   ╚═╝ ╚══════╝
`

var (
	// global flags — available on every subcommand
	cfgFile    string
	policiesDir string
	verbose    bool

	// colour helpers — automatically disabled if not a TTY
	bold    = color.New(color.Bold)
	green   = color.New(color.FgGreen, color.Bold)
	red     = color.New(color.FgRed, color.Bold)
	yellow  = color.New(color.FgYellow)
	cyan    = color.New(color.FgCyan)
	dimmed  = color.New(color.Faint)
)

// rootCmd is the base command — `auvis` with no subcommand
// prints version + help.
var rootCmd = &cobra.Command{
	Use:   "auvis",
	Short: "AUVIS — Data Governance Gateway",
	Long: fmt.Sprintf(`%s
AUVIS is a data governance gateway.
It enforces field-level masking, row filtering, and PII interception
policies on every piece of data moving through your systems.

Version: %s

Quick start:
  auvis init       scaffold auvis.yaml and policies/ in current directory
  auvis validate   check your policies for errors
  auvis dev        start the gateway with live policy reloading
  auvis status     show gateway health and active policies
  auvis logs       tail and filter the audit log

Documentation: https://auvis.io/docs
`, banner, version),
	// Running bare `auvis` with no subcommand prints help, not an error
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

// Execute is called by main.go — entry point for the entire CLI
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		red.Fprintf(os.Stderr, "\nerror: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	// Global flags — inherited by all subcommands
	rootCmd.PersistentFlags().StringVarP(
		&cfgFile,
		"config", "c",
		"auvis.yaml",
		"path to auvis.yaml config file",
	)
	rootCmd.PersistentFlags().StringVar(
		&policiesDir,
		"policies",
		"./policies",
		"path to policies directory",
	)
	rootCmd.PersistentFlags().BoolVarP(
		&verbose,
		"verbose", "v",
		false,
		"enable verbose output",
	)

	// Version flag — `auvis --version`
	rootCmd.Version = version
	rootCmd.SetVersionTemplate(fmt.Sprintf(
		"auvis version %s\n", version,
	))

	// Register all subcommands
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(validateCmd)
	rootCmd.AddCommand(devCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(logsCmd)
	rootCmd.AddCommand(versionCmd)
	
	rootCmd.AddCommand(policiesCmd)
	policiesCmd.AddCommand(policiesListCmd)
}

// printSuccess prints a green checkmark line — used across all commands
func printSuccess(format string, a ...any) {
	green.Fprint(os.Stdout, "✓ ")
	fmt.Fprintf(os.Stdout, format+"\n", a...)
}

// printError prints a red cross line — used across all commands
func printError(format string, a ...any) {
	red.Fprint(os.Stderr, "✗ ")
	fmt.Fprintf(os.Stderr, format+"\n", a...)
}

// printWarning prints a yellow warning line
func printWarning(format string, a ...any) {
	yellow.Fprint(os.Stdout, "⚠ ")
	fmt.Fprintf(os.Stdout, format+"\n", a...)
}

// printInfo prints a cyan info line
func printInfo(format string, a ...any) {
	cyan.Fprint(os.Stdout, "→ ")
	fmt.Fprintf(os.Stdout, format+"\n", a...)
}

// printDimmed prints a faint line for secondary information
func printDimmed(format string, a ...any) {
	dimmed.Fprintf(os.Stdout, "  "+format+"\n", a...)
}

// separator prints a full-width divider line
func separator() {
	dimmed.Println("────────────────────────────────────────────────────")
}