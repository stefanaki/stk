package cli

import (
	"os"

	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate shell completion scripts",
	Long: `Generate shell completion scripts for stk.

To load completions:

Bash:
  $ source <(stk completion bash)
  # To load completions for each session, execute once:
  # Linux:
  $ stk completion bash > /etc/bash_completion.d/stk
  # macOS:
  $ stk completion bash > $(brew --prefix)/etc/bash_completion.d/stk

Zsh:
  # If shell completion is not already enabled in your environment,
  # you will need to enable it. You can execute the following once:
  $ echo "autoload -U compinit; compinit" >> ~/.zshrc

  # To load completions for each session, execute once:
  $ stk completion zsh > "${fpath[1]}/_stk"
  # You will need to start a new shell for this setup to take effect.

Fish:
  $ stk completion fish | source
  # To load completions for each session, execute once:
  $ stk completion fish > ~/.config/fish/completions/stk.fish

PowerShell:
  PS> stk completion powershell | Out-String | Invoke-Expression
  # To load completions for every new session, run:
  PS> stk completion powershell > stk.ps1
  # and source this file from your PowerShell profile.
`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
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
		return nil
	},
}

func init() {
	rootCmd.AddCommand(completionCmd)
}
