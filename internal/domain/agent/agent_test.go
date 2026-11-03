package agent

import (
	"strings"
	"testing"
)

func md(repo, name string) Asset {
	return Asset{
		Repo:    repo,
		Mapping: AssetMapping{Source: ".claude/agents", Kind: MarkdownFiles, Target: ".claude/agents"},
		Name:    name,
		Files:   []AssetFile{{Content: repo, Mode: 0o644}},
	}
}

func TestRegistryGet(t *testing.T) {
	r := NewRegistry()
	for _, k := range []Kind{Claude, Copilot, Antigravity} {
		def, err := r.Get(k)
		if err != nil || def.Kind != k || len(def.Mappings) == 0 {
			t.Errorf("%s: %+v, %v", k, def, err)
		}
	}
	if _, err := r.Get("cursor"); err == nil || !strings.Contains(err.Error(), "antigravity, claude, copilot") {
		t.Fatal(err)
	}
	if def, _ := r.Get(Antigravity); !def.Experimental || def.BuiltinKit == "" || def.SbxAgent != "" {
		t.Fatalf("%+v", def)
	}
}

func TestPlaceWithoutConflict(t *testing.T) {
	def, _ := NewRegistry().Get(Claude)
	placed, conflicts, err := def.Place([]Asset{md("api", "a.md"), md("ui", "b.md")})
	if err != nil || len(conflicts) != 0 || len(placed) != 2 {
		t.Fatalf("%+v %+v %v", placed, conflicts, err)
	}
	if placed[0].Path != ".claude/agents/a.md" {
		t.Fatal(placed[0].Path)
	}
}

func TestPlaceConflictPrefixesAllRepos(t *testing.T) {
	def, _ := NewRegistry().Get(Claude)
	placed, conflicts, err := def.Place([]Asset{md("api", "a.md"), md("ui", "a.md")})
	if err != nil {
		t.Fatal(err)
	}
	if placed[0].Path != ".claude/agents/api-a.md" || placed[1].Path != ".claude/agents/ui-a.md" {
		t.Fatalf("%+v", placed)
	}
	if len(conflicts) != 1 || len(conflicts[0].Repos) != 2 || !strings.Contains(conflicts[0].Resolution, "<repo>-a.md") {
		t.Fatalf("%+v", conflicts)
	}
}

func TestPlaceSkillDirectoryConflict(t *testing.T) {
	def, _ := NewRegistry().Get(Copilot)
	skill := func(repo, source string) Asset {
		return Asset{
			Repo:    repo,
			Mapping: AssetMapping{Source: source, Kind: Directories, Target: ".copilot/skills"},
			Name:    "lint",
			Files:   []AssetFile{{Rel: "SKILL.md", Content: source}, {Rel: "ref/a.md", Content: "x"}},
		}
	}
	// Dasselbe Repo liefert den Skill doppelt: Das zuerst gemappte gilt.
	placed, conflicts, err := def.Place([]Asset{skill("api", ".github/skills"), skill("api", ".claude/skills")})
	if err != nil {
		t.Fatal(err)
	}
	if len(placed) != 2 || placed[0].Content != ".github/skills" || placed[0].Path != ".copilot/skills/lint/SKILL.md" {
		t.Fatalf("%+v", placed)
	}
	if len(conflicts) != 1 || !strings.Contains(conflicts[0].Resolution, "selben Repo") {
		t.Fatalf("%+v", conflicts)
	}
	// Zwei Repos: Der Ordner bekommt das Präfix.
	placed, _, err = def.Place([]Asset{skill("api", ".github/skills"), skill("ui", ".github/skills")})
	if err != nil || placed[2].Path != ".copilot/skills/ui-lint/SKILL.md" {
		t.Fatalf("%+v %v", placed, err)
	}
}

func TestPlaceRejectsReservedPaths(t *testing.T) {
	def, _ := NewRegistry().Get(Claude)
	a := Asset{Repo: "api", Mapping: AssetMapping{Target: ".claude"}, Name: "settings.json", Files: []AssetFile{{}}}
	if _, _, err := def.Place([]Asset{a}); err == nil || !strings.Contains(err.Error(), "reservierten") {
		t.Fatal(err)
	}
}

func TestWorkspaceScopeIgnoresHomeReservation(t *testing.T) {
	def, _ := NewRegistry().Get(Claude)
	a := Asset{Repo: "api", Mapping: AssetMapping{Target: ".claude", Scope: Workspace}, Name: "settings.json", Files: []AssetFile{{}}}
	if _, _, err := def.Place([]Asset{a}); err != nil {
		t.Fatal(err)
	}
}

func TestExtend(t *testing.T) {
	r := NewRegistry()
	extra := AssetMapping{Source: ".team/agents", Kind: MarkdownFiles, Target: ".claude/agents"}
	if err := r.Extend(Claude, []AssetMapping{extra}); err != nil {
		t.Fatal(err)
	}
	def, _ := r.Get(Claude)
	if def.Mappings[len(def.Mappings)-1] != extra {
		t.Fatalf("%+v", def.Mappings)
	}
	bad := []AssetMapping{
		{Source: "../x", Target: "y"},
		{Source: "x", Target: "/abs"},
		{Source: "", Target: "y"},
		{Source: "x", Target: ".claude/settings.json"},
	}
	for _, m := range bad {
		if err := r.Extend(Claude, []AssetMapping{m}); err == nil {
			t.Errorf("%+v müsste abgelehnt werden", m)
		}
	}
	if err := r.Extend("cursor", nil); err == nil {
		t.Error("unbekannter Agent akzeptiert")
	}
}
