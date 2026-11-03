package sandbox

import (
	"strings"
	"testing"

	"github.com/feierabendgewitter/boxai/internal/domain/agent"
	"github.com/feierabendgewitter/boxai/internal/domain/profile"
	"github.com/feierabendgewitter/boxai/internal/domain/workspace"
)

func definition(t *testing.T, k agent.Kind) agent.Definition {
	t.Helper()
	def, err := agent.NewRegistry().Get(k)
	if err != nil {
		t.Fatal(err)
	}
	return def
}

var ws = workspace.Workspace{Name: "shop", Root: "/ws", Path: "/ws/shop"}

func TestBuildKitPlacesFiles(t *testing.T) {
	kit, notes, err := BuildKit(KitInput{
		Workspace: ws,
		Agent:     definition(t, agent.Claude),
		Assets: []agent.PlacedFile{
			{Path: ".claude/agents/a.md", Content: "a", Mode: 0o600},
			{Scope: agent.Workspace, Path: ".agents/skills/x/SKILL.md", Content: "x"},
		},
		Config:  []profile.ConfigFile{{Path: ".npmrc", Content: "n", Mode: 0o600}},
		Context: []ContextFile{{Name: "AGENTS.md", Content: "c"}},
	})
	if err != nil || len(notes) != 0 {
		t.Fatal(err, notes)
	}
	if kit.Name != "boxai-shop" || kit.Empty() {
		t.Fatalf("%+v", kit)
	}
	want := map[string]uint32{
		"/home/agent/.claude/agents/a.md":    0o600,
		"/ws/shop/.agents/skills/x/SKILL.md": 0o644,
		"/home/agent/.npmrc":                 0o600,
		"/ws/shop/AGENTS.md":                 0o644,
	}
	if len(kit.Files) != len(want) {
		t.Fatalf("%+v", kit.Files)
	}
	for _, f := range kit.Files {
		if mode, ok := want[f.Path]; !ok || mode != f.Mode {
			t.Errorf("%s mode %o", f.Path, f.Mode)
		}
	}
}

func TestBuildKitAssetWinsOverProfileFile(t *testing.T) {
	kit, notes, err := BuildKit(KitInput{
		Workspace: ws,
		Agent:     definition(t, agent.Claude),
		Assets:    []agent.PlacedFile{{Path: ".npmrc", Content: "asset"}},
		Config:    []profile.ConfigFile{{Path: ".npmrc", Content: "profil"}},
	})
	if err != nil || len(kit.Files) != 1 || kit.Files[0].Content != "asset" || len(notes) != 1 {
		t.Fatalf("%+v %v %v", kit, notes, err)
	}
}

func TestBuildKitRejectsReservedConfig(t *testing.T) {
	_, _, err := BuildKit(KitInput{
		Workspace: ws,
		Agent:     definition(t, agent.Claude),
		Config:    []profile.ConfigFile{{Path: ".claude/settings.json"}},
	})
	if err == nil || !strings.Contains(err.Error(), "reservierten") {
		t.Fatal(err)
	}
}

func TestEmptyKit(t *testing.T) {
	kit, _, err := BuildKit(KitInput{Workspace: ws, Agent: definition(t, agent.Copilot)})
	if err != nil || !kit.Empty() {
		t.Fatalf("%+v %v", kit, err)
	}
}

func TestRenderContextClaude(t *testing.T) {
	files := RenderContext(ContextInput{
		WorkspaceName: "shop",
		Repos: []RepoContext{
			{Name: "api", Branch: "boxai/shop", OKFIndex: ".okf/index.md", Instructions: []string{"AGENTS.md", "CLAUDE.md"}},
			{Name: "ui", Branch: "boxai/shop"},
		},
		Profiles: []string{"react"},
		Hints:    []string{"npm ci"},
		Agent:    definition(t, agent.Claude),
	})
	if len(files) != 2 || files[0].Name != "AGENTS.md" || files[1].Name != "CLAUDE.md" {
		t.Fatalf("%+v", files)
	}
	md := files[0].Content
	for _, want := range []string{"`api/.okf/index.md`", "## Zuerst lesen", "`api/AGENTS.md`, `api/CLAUDE.md`", "| `ui` | `boxai/shop` | – |", "Profile: react", "`npm ci`"} {
		if !strings.Contains(md, want) {
			t.Errorf("%q fehlt in:\n%s", want, md)
		}
	}
	if !strings.Contains(files[1].Content, "AGENTS.md") {
		t.Error(files[1].Content)
	}
}

func TestRenderContextWithoutOptionalSections(t *testing.T) {
	files := RenderContext(ContextInput{
		WorkspaceName: "shop",
		Repos:         []RepoContext{{Name: "api", Branch: "x"}},
		Agent:         definition(t, agent.Copilot),
	})
	if len(files) != 1 {
		t.Fatalf("%+v", files)
	}
	for _, unwanted := range []string{"Zuerst lesen", "Stack", "Build-Kommandos"} {
		if strings.Contains(files[0].Content, unwanted) {
			t.Errorf("%q sollte fehlen", unwanted)
		}
	}
}
