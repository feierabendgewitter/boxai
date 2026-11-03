package assetfs

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/feierabendgewitter/boxai/internal/domain/agent"
)

func tree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, content := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func mappings(t *testing.T, k agent.Kind) []agent.AssetMapping {
	t.Helper()
	def, err := agent.NewRegistry().Get(k)
	if err != nil {
		t.Fatal(err)
	}
	return def.Mappings
}

func TestCollectClaude(t *testing.T) {
	root := tree(t, map[string]string{
		".claude/agents/reviewer.md":       "agent",
		".claude/agents/notes.txt":         "keine Markdown-Datei",
		".claude/commands/deploy.md":       "cmd",
		".claude/skills/lint/SKILL.md":     "skill",
		".claude/skills/lint/ref/rules.md": "regeln",
		".claude/skills/lint/logo.png":     "\x00\x01binär",
		".claude/skills/leer/logo.png":     "\x00",
		".claude/skills/lose.md":           "kein Ordner",
		".claude/settings.json":            "{}",
	})
	res, err := New().Collect(context.Background(), "api", root, mappings(t, agent.Claude))
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]agent.Asset{}
	for _, a := range res.Assets {
		byName[a.Name] = a
		if a.Repo != "api" {
			t.Errorf("%+v", a)
		}
	}
	if len(res.Assets) != 3 {
		t.Fatalf("%+v", res.Assets)
	}
	if byName["reviewer.md"].Files[0].Content != "agent" {
		t.Fatal(byName["reviewer.md"])
	}
	lint := byName["lint"]
	rels := []string{}
	for _, f := range lint.Files {
		rels = append(rels, f.Rel)
	}
	slices.Sort(rels)
	if !slices.Equal(rels, []string{"SKILL.md", "ref/rules.md"}) {
		t.Fatal(rels)
	}
	if !slices.Contains(res.Skipped, ".claude/skills/lint/logo.png") {
		t.Fatal(res.Skipped)
	}
}

func TestCollectMissingDirectoriesAreFine(t *testing.T) {
	res, err := New().Collect(context.Background(), "api", t.TempDir(), mappings(t, agent.Copilot))
	if err != nil || len(res.Assets) != 0 {
		t.Fatal(res, err)
	}
}

func TestOversizedAndSymlinkedFilesAreSkipped(t *testing.T) {
	root := tree(t, map[string]string{
		".agents/skills/a/SKILL.md": "ok",
		".agents/skills/a/big.md":   strings.Repeat("x", maxFileSize+1),
	})
	if err := os.Symlink("/etc/hosts", filepath.Join(root, ".agents/skills/a/link.md")); err != nil {
		t.Fatal(err)
	}
	res, err := New().Collect(context.Background(), "api", root, mappings(t, agent.Antigravity))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Assets) != 1 || len(res.Assets[0].Files) != 1 {
		t.Fatalf("%+v", res.Assets)
	}
	if len(res.Skipped) != 2 {
		t.Fatal(res.Skipped)
	}
}

func TestMarkdownBinarySkipped(t *testing.T) {
	root := tree(t, map[string]string{".claude/agents/x.md": "\xff\xfe"})
	res, err := New().Collect(context.Background(), "api", root, mappings(t, agent.Claude))
	if err != nil || len(res.Assets) != 0 || len(res.Skipped) != 1 {
		t.Fatal(res, err)
	}
}

const concept = "---\ntype: Decision\ntitle: X\n---\n# X\n"

func TestRepoContext(t *testing.T) {
	tests := []struct {
		name  string
		files map[string]string
		okf   string
		instr []string
	}{
		{"nichts", map[string]string{"README.md": "x"}, "", nil},
		{"Punkt-OKF", map[string]string{".okf/index.md": "", ".okf/a/b.md": concept}, ".okf/index.md", nil},
		{"Ordner unter docs", map[string]string{"docs/wissen/index.md": "", "docs/wissen/c.md": concept}, "docs/wissen/index.md", nil},
		{"Erste Ebene", map[string]string{"okf/index.md": "", "okf/c.md": concept}, "okf/index.md", nil},
		{"index ohne Konzept", map[string]string{"docs/index.md": "", "docs/x/readme.md": "# ohne Frontmatter"}, "", nil},
		{"Konzept ohne type", map[string]string{"site/index.md": "", "site/x.md": "---\ntitle: x\n---\n"}, "", nil},
		{"Frontmatter offen", map[string]string{"site/index.md": "", "site/x.md": "---\ntype: x\n"}, "", nil},
		{"Nur log", map[string]string{"site/index.md": "", "site/log.md": concept}, "", nil},
		{"Anweisungen", map[string]string{"AGENTS.md": "", "CLAUDE.md": "", ".github/copilot-instructions.md": ""}, "", []string{"AGENTS.md", "CLAUDE.md", ".github/copilot-instructions.md"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			okf, instr, err := New().RepoContext(context.Background(), tree(t, tt.files))
			if err != nil {
				t.Fatal(err)
			}
			if okf != tt.okf || !slices.Equal(instr, tt.instr) {
				t.Fatalf("%q %v", okf, instr)
			}
		})
	}
}
