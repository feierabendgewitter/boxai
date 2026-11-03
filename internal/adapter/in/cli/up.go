package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/feierabendgewitter/boxai/internal/app/port"
)

func newUpCmd(services func() Services, rt *Runtime) *cobra.Command {
	var req port.CreateRequest
	cmd := &cobra.Command{
		Use:   "up",
		Short: "Legt Worktrees und Sandbox an und startet den Agent",
		Long:  "Ohne Flags fragt boxai alles interaktiv ab; mit Flags oder --config läuft es nicht-interaktiv.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			svc := services()
			req.DefaultRoot = svc.Defaults.Root
			req.CacheRoot = svc.Defaults.CacheRoot
			req.KitRoot = svc.Defaults.KitRoot
			req.DryRun = rt.DryRun
			res, err := svc.Create.Create(cmd.Context(), req)
			if err != nil {
				return err
			}
			if rt.DryRun || req.NoAttach {
				fmt.Fprintf(cmd.OutOrStdout(), "Workspace %s: %s (Sandbox %s)\n", res.Name, res.Path, res.SandboxName)
			}
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&req.Dir, "dir", ".", "Startverzeichnis für den Repo-Scan")
	f.IntVar(&req.Depth, "depth", 3, "maximale Suchtiefe")
	f.StringVar(&req.Root, "root", "", "Wurzel der Workspaces (Default ~/boxai-workspaces)")
	f.StringVar(&req.Name, "name", "", "Name des Workspaces")
	f.StringArrayVar(&req.Repos, "repo", nil, "Repo-Name oder Clone-URL (mehrfach)")
	f.StringVar(&req.Base, "base", "", "Base-Branch (Default: Default-Branch je Repo)")
	f.StringVar(&req.Branch, "branch", "", "neuer Branch (Default boxai/<name>)")
	f.StringVar(&req.Checkout, "checkout", "", "bestehenden Branch nutzen statt einen neuen anzulegen")
	f.StringVar(&req.Agent, "agent", "", "claude, copilot oder antigravity")
	f.StringSliceVar(&req.Profiles, "profile", nil, "Stack-Profile, z. B. kotlin-spring,react")
	f.StringVar(&req.Image, "image", "", "eigenes Image (ohne Registry-Domain: lokal geladen)")
	f.StringVar(&req.Dockerfile, "dockerfile", "", "Dockerfile, das gebaut und geladen wird")
	f.StringVar(&req.Secrets, "secrets", "", "Secret-Datei (.env oder YAML)")
	f.StringVar(&req.Config, "config", "", "Workspace-Datei, z. B. boxai.yaml")
	f.BoolVar(&req.NoDocker, "no-docker", false, "Image-Variante ohne Docker Engine")
	f.BoolVar(&req.KeepOnErr, "keep-on-error", false, "bei Fehlern nichts zurückrollen")
	f.BoolVar(&req.NoAttach, "no-attach", false, "alles anlegen, den Agent aber nicht starten")
	cmd.MarkFlagsMutuallyExclusive("image", "dockerfile")
	cmd.MarkFlagsMutuallyExclusive("branch", "checkout")
	return cmd
}
