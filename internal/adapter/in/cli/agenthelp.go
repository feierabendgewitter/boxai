package cli

import (
	_ "embed" // bettet die Anleitung ein
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
)

//go:embed agenthelp.md
var agentHelpText string

func newAgentHelpCmd(root *cobra.Command) *cobra.Command {
	return &cobra.Command{
		Use:   "agent-help",
		Short: "Gibt eine Anleitung für KI-Agenten als Markdown aus",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()
			fmt.Fprintln(out, strings.TrimRight(agentHelpText, "\n"))
			fmt.Fprint(out, "\n## Kommandoreferenz\n")
			writeFlags(out, "Globale Flags", root.PersistentFlags().FlagUsages())
			for _, c := range root.Commands() {
				// Cobra ergänzt completion beim Start; es gehört nicht zur Bedienung.
				if c == cmd || !c.IsAvailableCommand() || c.Name() == "completion" {
					continue
				}
				fmt.Fprintf(out, "\n### `%s`\n\n%s\n", c.UseLine(), c.Short)
				writeFlags(out, "Flags", c.LocalNonPersistentFlags().FlagUsages())
			}
			return nil
		},
	}
}

func writeFlags(out io.Writer, title, usages string) {
	if usages == "" {
		return
	}
	fmt.Fprintf(out, "\n%s:\n\n```text\n%s```\n", title, usages)
}
