package cli

import (
	"fmt"

	"github.com/calmcacil/CalmsToolkit/internal/app"
	"github.com/spf13/cobra"
)

var completionShells = []string{"bash", "zsh", "fish"}

func fixedCompletions(values ...string) cobra.CompletionFunc {
	return func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return values, cobra.ShellCompDirectiveNoFileComp
	}
}

func newCompletionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "completion <shell>",
		Short:                 "Generate a shell completion script",
		Long:                  "Generate a completion script for bash, zsh, or fish and write it to stdout.",
		Args:                  cobra.ExactArgs(1),
		ValidArgs:             completionShells,
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			root := cmd.Root()
			switch args[0] {
			case "bash":
				return root.GenBashCompletionV2(cmd.OutOrStdout(), true)
			case "zsh":
				return root.GenZshCompletion(cmd.OutOrStdout())
			case "fish":
				return root.GenFishCompletion(cmd.OutOrStdout(), true)
			default:
				return app.Error(app.ExitUsage, fmt.Errorf("unsupported shell %q (choose bash, zsh, or fish)", args[0]))
			}
		},
	}
	cmd.ValidArgsFunction = func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return completionShells, cobra.ShellCompDirectiveNoFileComp
	}
	return cmd
}
