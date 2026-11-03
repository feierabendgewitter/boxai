package cli

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/feierabendgewitter/boxai/internal/app/port"
)

func newScanCmd(services func() Services) *cobra.Command {
	var req port.ScanRequest
	cmd := &cobra.Command{
		Use:   "scan",
		Short: "Listet Git-Repos unter einem Verzeichnis",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			repos, err := services().Scan.Scan(cmd.Context(), req)
			if err != nil {
				return err
			}
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tBRANCH\tSTATUS\tPFAD")
			for _, r := range repos {
				status := "sauber"
				if r.Dirty {
					status = "geändert"
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", r.Name, r.CurrentBranch, status, r.Path)
			}
			return w.Flush()
		},
	}
	cmd.Flags().StringVar(&req.Dir, "dir", ".", "Startverzeichnis")
	cmd.Flags().IntVar(&req.Depth, "depth", 3, "maximale Suchtiefe")
	return cmd
}
