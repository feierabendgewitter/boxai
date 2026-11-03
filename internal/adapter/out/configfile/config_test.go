package configfile

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/feierabendgewitter/boxai/internal/domain/agent"
)

func newLoader(t *testing.T, env map[string]string) (*Loader, *[]string, string) {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var seen []string
	l := New("/home/j", filepath.Join(dir, "global.yaml"), func(k string) (string, bool) {
		v, ok := env[k]
		return v, ok
	}, func(v string) { seen = append(seen, v) })
	return l, &seen, dir
}

func write(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// Das Beispiel entspricht dem Format aus dem Plan.
func TestLoadWorkspaceConfigPlanExample(t *testing.T) {
	l, _, dir := newLoader(t, nil)
	path := write(t, dir, "boxai.yaml", `name: shop-feature-x
root: ~/boxai-workspaces
agent: claude
profiles: [kotlin-spring, react]
image: ""
dockerfile: ""
branch: feature/x
base: main
repos:
  - path: ../shop-backend
  - path: ../shop-frontend
    base: develop
  - url: git@gitlab.com:team/shared-lib.git
secrets: ./secrets.yaml
network:
  allow: ["nexus.firma.de:443"]
`)
	cfg, err := l.LoadWorkspaceConfig(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Name != "shop-feature-x" || cfg.Root != "/home/j/boxai-workspaces" || cfg.Agent != "claude" || len(cfg.Profiles) != 2 {
		t.Fatalf("%+v", cfg)
	}
	if cfg.Repos[0].Path != filepath.Join(filepath.Dir(dir), "shop-backend") {
		t.Fatal(cfg.Repos[0].Path)
	}
	if cfg.Repos[1].Base != "develop" || cfg.Repos[2].URL == "" {
		t.Fatalf("%+v", cfg.Repos)
	}
	if cfg.Secrets != filepath.Join(dir, "secrets.yaml") || cfg.NetworkAllow[0] != "nexus.firma.de:443" {
		t.Fatalf("%+v", cfg)
	}
}

func TestSecretsAsMapping(t *testing.T) {
	l, _, dir := newLoader(t, nil)
	path := write(t, dir, "b.yaml", "secrets:\n  file: s.env\n  mode: env\n")
	cfg, err := l.LoadWorkspaceConfig(context.Background(), path)
	if err != nil || cfg.Secrets != filepath.Join(dir, "s.env") || cfg.SecretsMode != "env" {
		t.Fatalf("%+v %v", cfg, err)
	}
}

func TestInvalidConfigs(t *testing.T) {
	tests := map[string]string{
		"unbekanntes Feld": "nmae: x\n",
		"Repo ohne Quelle": "repos:\n  - base: main\n",
		"Repo doppelt":     "repos:\n  - {path: a, url: b}\n",
		"Mapping-Art":      "agents:\n  claude:\n    mappings:\n      - {source: a, kind: x, target: b}\n",
		"Mapping-Scope":    "agents:\n  claude:\n    mappings:\n      - {source: a, kind: files, target: b, scope: x}\n",
	}
	for name, content := range tests {
		t.Run(name, func(t *testing.T) {
			l, _, dir := newLoader(t, nil)
			if _, err := l.LoadWorkspaceConfig(context.Background(), write(t, dir, "b.yaml", content)); err == nil {
				t.Fatal("Fehler erwartet")
			}
		})
	}
}

func TestGlobalConfig(t *testing.T) {
	l, _, dir := newLoader(t, nil)
	cfg, err := l.LoadGlobalConfig(context.Background())
	if err != nil || cfg.Name != "" {
		t.Fatalf("Fehlende Datei: %+v %v", cfg, err)
	}
	write(t, dir, "global.yaml", `agent: copilot
agents:
  claude:
    mappings:
      - {source: .team/agents, kind: files, target: .claude/agents}
      - {source: .team/skills, kind: dirs, target: .agents/skills, scope: workspace}
`)
	cfg, err = l.LoadGlobalConfig(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	got := cfg.Mappings[agent.Claude]
	if cfg.Agent != "copilot" || len(got) != 2 || got[0].Kind != agent.MarkdownFiles || got[1].Kind != agent.Directories || got[1].Scope != agent.Workspace {
		t.Fatalf("%+v", cfg)
	}
}

func TestSecretsYAMLPlanExample(t *testing.T) {
	l, seen, dir := newLoader(t, map[string]string{"NEXUS_TOKEN": "nexus-wert"})
	secretFile := write(t, dir, "npm", "npm-wert\n")
	path := write(t, dir, "secrets.yaml", `secrets:
  - name: MAVEN_TOKEN
    hosts: ["nexus.firma.de"]
    value: ${env:NEXUS_TOKEN}
  - name: NPM_TOKEN
    hosts: ["registry.npmjs.org"]
    value: ${file:`+secretFile+`}
  - name: PLAIN
    hosts: ["a.example"]
    value: direkt
`)
	specs, err := l.LoadSecrets(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if len(specs) != 3 || specs[0].Value != "nexus-wert" || specs[1].Value != "npm-wert" || specs[2].Value != "direkt" {
		t.Fatal(specs)
	}
	if specs[0].Hosts[0] != "nexus.firma.de" {
		t.Fatal(specs[0].Hosts)
	}
	if strings.Join(*seen, ",") != "nexus-wert,npm-wert,direkt" {
		t.Fatal("Redaction-Hook nicht aufgerufen")
	}
}

func TestSecretsEnvPlanExample(t *testing.T) {
	l, _, dir := newLoader(t, map[string]string{"NEXUS_TOKEN": "nexus-wert"})
	path := write(t, dir, "secrets.env", `# Format: NAME=wert  plus Host-Zuordnung als Kommentar-Direktive
# @hosts nexus.firma.de
MAVEN_TOKEN=${NEXUS_TOKEN}
# @hosts registry.npmjs.org, npm.example
export NPM_TOKEN="xyz"

OHNE_HOSTS='abc'
`)
	specs, err := l.LoadSecrets(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if specs[0].Name != "MAVEN_TOKEN" || specs[0].Value != "nexus-wert" || specs[0].Hosts[0] != "nexus.firma.de" {
		t.Fatal(specs[0])
	}
	if specs[1].Value != "xyz" || len(specs[1].Hosts) != 2 {
		t.Fatal(specs[1])
	}
	if specs[2].Value != "abc" || len(specs[2].Hosts) != 0 {
		t.Fatal(specs[2])
	}
}

func TestSecretErrorsNeverContainValues(t *testing.T) {
	const value = "topsecret-123"
	tests := map[string]struct{ name, content string }{
		"Env fehlt":    {"s.yaml", "secrets:\n  - {name: A, hosts: [h], value: '${env:FEHLT}'}\n"},
		"Datei fehlt":  {"s.yaml", "secrets:\n  - {name: A, hosts: [h], value: '${file:/fehlt/x}'}\n"},
		"Referenz":     {"s.yaml", "secrets:\n  - {name: A, hosts: [h], value: '${vault:x}'}\n"},
		"Env-Zeile":    {"s.env", value + "\n"},
		"YAML-Fehler":  {"s.yaml", "secrets:\n  - {name: A, value: " + value + "\n"},
		"Unbek. Feld":  {"s.yaml", "secrets:\n  - {name: A, valeu: " + value + "}\n"},
		"Bare in YAML": {"s.yaml", "secrets:\n  - {name: A, hosts: [h], value: '${NEXUS}'}\n"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			l, _, dir := newLoader(t, nil)
			_, err := l.LoadSecrets(context.Background(), write(t, dir, tt.name, tt.content))
			if err == nil {
				t.Fatal("Fehler erwartet")
			}
			if strings.Contains(err.Error(), value) {
				t.Fatalf("Fehlertext verrät den Wert: %v", err)
			}
		})
	}
}

func TestNormalizePath(t *testing.T) {
	l, _, dir := newLoader(t, nil)
	real := filepath.Join(dir, "real")
	if err := os.Mkdir(real, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if got, err := l.NormalizePath(ctx, filepath.Join(link, "noch", "nicht")); err != nil || got != filepath.Join(real, "noch", "nicht") {
		t.Fatal(got, err)
	}
	if got, err := l.NormalizePath(ctx, "/fehlt-boxai/x"); err != nil || got != "/fehlt-boxai/x" {
		t.Fatal(got, err)
	}
	if got, _ := l.NormalizePath(ctx, "~/x"); !strings.HasSuffix(got, "/home/j/x") {
		t.Fatal(got)
	}
}
