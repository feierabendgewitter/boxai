package kitfs

import (
	"bytes"
	"context"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"gopkg.in/yaml.v3"

	"github.com/feierabendgewitter/boxai/internal/adapter/out/execx/execxtest"
	"github.com/feierabendgewitter/boxai/internal/domain/agent"
	"github.com/feierabendgewitter/boxai/internal/domain/profile"
	"github.com/feierabendgewitter/boxai/internal/domain/sandbox"
	"github.com/feierabendgewitter/boxai/internal/domain/workspace"
)

var update = flag.Bool("update", false, "Golden Files neu schreiben")

func golden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", "golden", name)
	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("%s weicht ab:\n%s", name, got)
	}
}

// buildKit durchläuft die Domäne wie der Use Case, damit das Golden File den echten Baum zeigt.
func buildKit(t *testing.T, kind agent.Kind) sandbox.KitSpec {
	t.Helper()
	def, err := agent.NewRegistry().Get(kind)
	if err != nil {
		t.Fatal(err)
	}
	var assets []agent.Asset
	for _, m := range def.Mappings {
		a := agent.Asset{Repo: "api", Mapping: m}
		if m.Kind == agent.Directories {
			a.Name = "lint"
			a.Files = []agent.AssetFile{{Rel: "SKILL.md", Content: "---\nname: lint\n---\nRegeln\n", Mode: 0o644}}
		} else {
			a.Name = "reviewer.md"
			a.Files = []agent.AssetFile{{Content: "# Reviewer\n", Mode: 0o644}}
		}
		assets = append(assets, a)
	}
	placed, _, err := def.Place(assets)
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := profile.Resolve([]string{profile.React}, nil)
	if err != nil {
		t.Fatal(err)
	}
	kit, _, err := sandbox.BuildKit(sandbox.KitInput{
		Workspace: workspace.Workspace{Name: "shop", Path: "/ws/shop"},
		Agent:     def,
		Assets:    placed,
		Config:    resolved.FilesFor([]string{"NPM_TOKEN"}),
		Context: sandbox.RenderContext(sandbox.ContextInput{
			WorkspaceName: "shop",
			Repos:         []sandbox.RepoContext{{Name: "api", Branch: "boxai/shop", OKFIndex: ".okf/index.md"}},
			Profiles:      resolved.Names, Hints: resolved.Hints, Agent: def,
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	return kit
}

func TestGoldenDescriptorPerAgent(t *testing.T) {
	for _, kind := range []agent.Kind{agent.Claude, agent.Copilot, agent.Antigravity} {
		t.Run(string(kind), func(t *testing.T) {
			data, err := Render(buildKit(t, kind))
			if err != nil {
				t.Fatal(err)
			}
			golden(t, string(kind)+".yaml", data)
		})
	}
}

// sbx verlangt die Syntax-Zeile am Anfang und akzeptiert nur bekannte Felder.
func TestDescriptorShape(t *testing.T) {
	data, err := Render(buildKit(t, agent.Claude))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(data), "# syntax=docker/sandbox-kit:3\n") {
		t.Fatal("Syntax-Zeile fehlt")
	}
	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	if doc["schemaVersion"] != "3" || doc["kind"] != "mixin" {
		t.Fatalf("%+v", doc)
	}
}

func TestReservedPathsAreNeverGenerated(t *testing.T) {
	reserved := []string{".claude.json", ".claude/settings.json", ".claude/.config.json", ".copilot/config.json"}
	for _, kind := range []agent.Kind{agent.Claude, agent.Copilot, agent.Antigravity} {
		for _, f := range buildKit(t, kind).Files {
			for _, r := range reserved {
				if f.Path == "/home/agent/"+r {
					t.Errorf("%s erzeugt %s", kind, f.Path)
				}
			}
		}
	}
}

func TestContentRoundTrips(t *testing.T) {
	const tricky = "zeile: 1\n# kein Kommentar\n${NPM_TOKEN}\n\"quote\" 'x'\n  eingerückt\n"
	kit := sandbox.KitSpec{Name: "k", Files: []sandbox.KitFile{{Path: "/home/agent/x", Content: tricky, Mode: 0o600}}}
	data, err := Render(kit)
	if err != nil {
		t.Fatal(err)
	}
	var doc descriptor
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	got := doc.Capabilities[0].Config.Files[0]
	if got.Content != tricky || got.Mode != "0600" {
		t.Fatalf("%+v", got)
	}
}

func TestEmptyKitHasNoInvalidLifecycle(t *testing.T) {
	data, err := Render(sandbox.KitSpec{Name: "k"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "lifecycle") {
		t.Fatal(string(data))
	}
}

func TestWriteReplacesAndValidates(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "kit")
	rec := &execxtest.Recorder{}
	w := New(rec, nil, "", false)
	ctx := context.Background()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "alt.yaml"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	kit := sandbox.KitSpec{Name: "boxai-shop", Files: []sandbox.KitFile{{Path: "/x", Mode: 0o644}}}
	if err := w.Write(ctx, kit, dir); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 || entries[0].Name() != "boxai-shop.yaml" {
		t.Fatalf("%v", entries)
	}
	if err := w.Validate(ctx, dir); err != nil {
		t.Fatal(err)
	}
	if got := rec.Lines()[0]; got != "sbx kit validate --no-input "+dir {
		t.Fatal(got)
	}
	if err := w.Remove(ctx, dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("Kit noch da")
	}
}

func TestWriteRootFilesReplacesStaleAssets(t *testing.T) {
	root := t.TempDir()
	stale := filepath.Join(root, ".claude", "agents", "alt.md")
	if err := os.MkdirAll(filepath.Dir(stale), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stale, []byte("alt"), 0o644); err != nil {
		t.Fatal(err)
	}
	w := New(&execxtest.Recorder{}, nil, "", false)
	kit := sandbox.KitSpec{
		Name:      "boxai-shop",
		RootDir:   root,
		RootFiles: []sandbox.KitFile{{Path: ".claude/skills/lint/SKILL.md", Content: "regeln", Mode: 0o644}},
	}
	if err := w.Write(context.Background(), kit, filepath.Join(t.TempDir(), "kit")); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(root, ".claude", "skills", "lint", "SKILL.md"))
	if err != nil || string(got) != "regeln" {
		t.Fatal(string(got), err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatal("alter Stand blieb liegen")
	}
}

func TestDryRunTouchesNothing(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, "kit")
	w := New(&execxtest.Recorder{}, fstest.MapFS{"agy/agy.yaml": {Data: []byte("x")}}, filepath.Join(parent, "builtin"), true)
	ctx := context.Background()
	if err := w.Write(ctx, sandbox.KitSpec{Name: "k"}, dir); err != nil {
		t.Fatal(err)
	}
	if err := w.Remove(ctx, parent); err != nil {
		t.Fatal(err)
	}
	if left, err := w.RemoveWorkspaceRoot(ctx, parent); err != nil || left != nil {
		t.Fatal(left, err)
	}
	got, err := w.Builtin(ctx, "agy")
	if err != nil || got != filepath.Join(parent, "builtin", "agy") {
		t.Fatal(got, err)
	}
	entries, _ := os.ReadDir(parent)
	if len(entries) != 0 {
		t.Fatalf("%v", entries)
	}
}

func TestRemoveWorkspaceRoot(t *testing.T) {
	ctx := context.Background()
	w := New(&execxtest.Recorder{}, nil, "", false)
	root := filepath.Join(t.TempDir(), "ws")
	mk := func(names ...string) {
		for _, n := range names {
			p := filepath.Join(root, n)
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(p, nil, 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	mk("AGENTS.md", "CLAUDE.md", ".agents/skills/a/SKILL.md", "notizen.txt")
	left, err := w.RemoveWorkspaceRoot(ctx, root)
	if err != nil || len(left) != 1 || left[0] != "notizen.txt" {
		t.Fatal(left, err)
	}
	if _, err := os.Stat(filepath.Join(root, "notizen.txt")); err != nil {
		t.Fatal("fremde Datei gelöscht")
	}
	if err := os.Remove(filepath.Join(root, "notizen.txt")); err != nil {
		t.Fatal(err)
	}
	mk("AGENTS.md")
	if left, err := w.RemoveWorkspaceRoot(ctx, root); err != nil || left != nil {
		t.Fatal(left, err)
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatal("leerer Ordner noch da")
	}
	if left, err := w.RemoveWorkspaceRoot(ctx, root); err != nil || left != nil {
		t.Fatal("zweiter Lauf:", left, err)
	}
}

func TestBuiltinExtractsAndRefreshes(t *testing.T) {
	root := t.TempDir()
	source := fstest.MapFS{"agy/agy.yaml": {Data: []byte("neu")}, "agy/sub/x.sh": {Data: []byte("s")}}
	w := New(&execxtest.Recorder{}, source, root, false)
	dir, err := w.Builtin(context.Background(), "agy")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "alt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Builtin(context.Background(), "agy"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "alt")); !os.IsNotExist(err) {
		t.Fatal("alte Datei überlebt")
	}
	if data, _ := os.ReadFile(filepath.Join(dir, "sub", "x.sh")); string(data) != "s" {
		t.Fatal(string(data))
	}
	if _, err := w.Builtin(context.Background(), "fehlt"); err == nil {
		t.Fatal("fehlender Kit akzeptiert")
	}
}
