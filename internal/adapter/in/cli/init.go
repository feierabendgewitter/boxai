package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

const exampleConfig = `# Workspace-Datei für boxai up --config boxai.yaml
name: mein-feature
# root: ~/boxai-workspaces
agent: claude            # claude | copilot | antigravity
profiles: [kotlin-spring, react]
# image: ""              # überschreibt das Profil-Image
# dockerfile: ""         # wird gebaut und geladen
branch: feature/mein-feature
# base: main             # sonst Default-Branch je Repo
repos:
  - path: ../backend
  - path: ../frontend
    # base: develop      # überschreibt base pro Repo
  # - url: git@gitlab.com:team/shared-lib.git
# secrets: ./secrets.yaml
network:
  allow: []              # z. B. nexus.firma.de:443
`

const exampleSecrets = `# Kopieren nach secrets.yaml. Werte nie im Klartext committen, sondern referenzieren.
secrets:
  - name: MAVEN_TOKEN            # Env-Variable in der Sandbox (enthält nur einen Platzhalter)
    hosts: ["nexus.firma.de"]    # Nur hier setzt der Proxy den echten Wert ein
    value: ${env:NEXUS_TOKEN}
  - name: NPM_TOKEN
    hosts: ["registry.npmjs.org"]
    value: ${file:~/.secrets/npm}
  - name: GITLAB_TOKEN
    hosts: ["gitlab.com"]
    value: ${env:GITLAB_TOKEN}
`

func newInitCmd() *cobra.Command {
	var dir string
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Schreibt eine Beispiel-boxai.yaml und secrets.example.yaml",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			files := []struct{ name, content string }{
				{"boxai.yaml", exampleConfig},
				{"secrets.example.yaml", exampleSecrets},
			}
			for _, f := range files {
				path := filepath.Join(dir, f.name)
				file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
				if errors.Is(err, os.ErrExist) {
					fmt.Fprintf(cmd.OutOrStdout(), "%s existiert, bleibt unverändert\n", path)
					continue
				}
				if err != nil {
					return err
				}
				_, writeErr := file.WriteString(f.content)
				if err := errors.Join(writeErr, file.Close()); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s geschrieben\n", path)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dir, "dir", ".", "Zielverzeichnis")
	return cmd
}
