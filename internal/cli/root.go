// Package cli implements the command-line interface for stk.
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/stefanaki/stk/internal/git"
	"github.com/stefanaki/stk/internal/stack"
)

var (
	cfgFile string
	verbose bool

	// Shared instances
	g       *git.Git
	manager *stack.Manager
)

// rootCmd represents the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:   "stk",
	Short: "A CLI tool for managing stacked branches",
	Long: `stk is a command-line tool for managing stacked branches (stacked diffs).

It helps you maintain a chain of dependent branches where each branch
builds on top of the previous one. This is useful for breaking large
features into smaller, reviewable pull requests while keeping them in sync.

Example workflow:
  stk init my-feature              # Start a new stack
  stk branch auth-models           # Create first branch in stack  
  # ... make changes, commit ...
  stk branch auth-api              # Create next branch
  # ... make changes, commit ...
  stk rebase                       # Rebase entire stack after base updates
  stk sync                         # Push all branches`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip initialization for commands that don't need git
		if cmd.Name() == "help" || cmd.Name() == "version" || cmd.Name() == "completion" {
			return nil
		}

		// Initialize git wrapper
		g = git.New()

		// Check if we're in a git repository
		if !g.IsInsideWorkTree() {
			return fmt.Errorf("not a git repository (or any parent up to mount point /)")
		}

		// Get git directory and initialize manager
		gitDir, err := g.GitDir()
		if err != nil {
			return fmt.Errorf("failed to find git directory: %w", err)
		}

		manager = stack.NewManager(gitDir)
		return nil
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.stk.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	// Bind flags to viper
	viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory
		home, err := os.UserHomeDir()
		if err != nil {
			return
		}

		// Search for config in home directory
		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".stk")
	}

	// Read environment variables
	viper.SetEnvPrefix("STK")
	viper.AutomaticEnv()

	// Read config file (ignore errors if not found)
	_ = viper.ReadInConfig()
}

// Git returns the shared git instance.
func Git() *git.Git {
	return g
}

// Manager returns the shared stack manager.
func Manager() *stack.Manager {
	return manager
}

// RequireStack loads the current stack or exits with an error.
func RequireStack() *stack.Stack {
	s, err := manager.Current()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
	return s
}

// RequireCleanTree ensures the working tree is clean or exits.
func RequireCleanTree() {
	if err := g.EnsureClean(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
