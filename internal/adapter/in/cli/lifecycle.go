package cli

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/feierabendgewitter/boxai/internal/app/port"
)

func newListCmd(services func() Services) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Listet Workspaces und den Status ihrer Sandboxes",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			infos, err := services().Lifecycle.List(cmd.Context())
			if err != nil {
				return err
			}
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tAGENT\tSANDBOX\tREPOS\tPFAD")
			for _, i := range infos {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", i.State.Name, i.State.Agent, i.SandboxStatus, strings.Join(i.Repos, ","), i.State.Path)
			}
			return w.Flush()
		},
	}
}

func newAttachCmd(services func() Services) *cobra.Command {
	return &cobra.Command{
		Use:   "attach <name>",
		Short: "Startet den Agent eines Workspaces erneut",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return services().Lifecycle.Attach(cmd.Context(), args[0])
		},
	}
}

func newStopCmd(services func() Services) *cobra.Command {
	return &cobra.Command{
		Use:   "stop <name>",
		Short: "Hält die Sandbox eines Workspaces an",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return services().Lifecycle.Stop(cmd.Context(), args[0])
		},
	}
}

func newRmCmd(services func() Services) *cobra.Command {
	var req port.RemoveRequest
	cmd := &cobra.Command{
		Use:   "rm <name>",
		Short: "Entfernt Sandbox und Worktrees; Branches bleiben ohne --delete-branches erhalten",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			req.Name = args[0]
			return services().Remove.Remove(cmd.Context(), req)
		},
	}
	cmd.Flags().BoolVar(&req.DeleteBranches, "delete-branches", false, "von boxai angelegte Branches löschen")
	cmd.Flags().BoolVar(&req.Force, "force", false, "auch bei uncommitteten Änderungen entfernen")
	return cmd
}
