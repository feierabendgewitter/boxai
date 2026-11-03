package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/feierabendgewitter/boxai/internal/app/port"
)

func newDoctorCmd(services func() Services) *cobra.Command {
	var req port.DoctorRequest
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Prüft git, sbx, sbx login und docker",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			report := services().Doctor.Run(cmd.Context(), req)
			for _, c := range report.Checks {
				mark := "✓"
				switch {
				case !c.OK && c.Required:
					mark = "✗"
				case !c.OK:
					mark = "○"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s %-34s %s\n", mark, c.Name, c.Detail)
			}
			if !report.Healthy() {
				return ErrUnhealthy
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&req.NeedDocker, "docker", false, "docker als Pflicht prüfen (für --dockerfile)")
	cmd.Flags().BoolVar(&req.Network, "network", false, "Netzwerk-Freigaben der Profile gegen die sbx-Policy prüfen")
	cmd.Flags().StringVar(&req.Sandbox, "sandbox", "", "--network im Kontext einer Sandbox prüfen, z. B. boxai-shop")
	return cmd
}
