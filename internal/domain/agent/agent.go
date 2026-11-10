// Package agent enthält die Agent-Registry und die Regeln für Agent- und Skill-Dateien.
package agent

import (
	"fmt"
	"path"
	"slices"
	"strings"
)

// Kind benennt einen unterstützten Agent.
type Kind string

// Die mitgelieferten Agents.
const (
	Claude      Kind = "claude"
	Copilot     Kind = "copilot"
	Antigravity Kind = "antigravity"
)

// AssetKind unterscheidet einzelne Dateien von Skill-Ordnern.
type AssetKind int

// Asset-Arten.
const (
	MarkdownFiles AssetKind = iota // alle *.md direkt im Quellordner
	Directories                    // jeder Unterordner ist ein Asset
)

// Scope legt fest, wohin ein Asset in der Sandbox kommt.
type Scope int

// Ziel-Scopes.
const (
	Home      Scope = iota // unter /home/agent
	Workspace              // unter dem Workspace-Ordner
)

// AssetMapping bildet einen Repo-Ordner auf ein Ziel in der Sandbox ab.
type AssetMapping struct {
	Source string
	Kind   AssetKind
	Target string
	Scope  Scope
}

// Definition beschreibt, wie boxai einen Agent startet.
type Definition struct {
	Kind Kind
	// SbxAgent ist das erste Argument von `sbx create`; leer bei BuiltinKit.
	SbxAgent string
	// BuiltinKit benennt einen mitgelieferten Kit, der den Agent stellt.
	BuiltinKit string
	// ImageVariant ist die Variante von docker/sandbox-templates.
	ImageVariant string
	Experimental bool
	Mappings     []AssetMapping
	// Reserved sind von sbx verwaltete Pfade relativ zum Home.
	Reserved []string
	// WritesClaudeMD ergänzt die Root-AGENTS.md um eine CLAUDE.md.
	WritesClaudeMD bool
}

// Registry hält alle bekannten Agents.
type Registry struct {
	defs map[Kind]Definition
}

// NewRegistry liefert die mitgelieferten Agents.
func NewRegistry() *Registry {
	return &Registry{defs: map[Kind]Definition{
		Claude: {
			Kind:           Claude,
			SbxAgent:       "claude",
			ImageVariant:   "claude-code",
			WritesClaudeMD: true,
			Reserved:       []string{".claude.json", ".claude/settings.json", ".claude/.config.json"},
			Mappings: []AssetMapping{
				{Source: ".claude/agents", Kind: MarkdownFiles, Target: ".claude/agents", Scope: Workspace},
				{Source: ".claude/skills", Kind: Directories, Target: ".claude/skills", Scope: Workspace},
				{Source: ".claude/commands", Kind: MarkdownFiles, Target: ".claude/commands", Scope: Workspace},
			},
		},
		Copilot: {
			Kind:         Copilot,
			SbxAgent:     "copilot",
			ImageVariant: "copilot",
			Reserved:     []string{".copilot/config.json"},
			Mappings: []AssetMapping{
				{Source: ".github/agents", Kind: MarkdownFiles, Target: ".copilot/agents"},
				{Source: ".github/skills", Kind: Directories, Target: ".copilot/skills"},
				{Source: ".claude/skills", Kind: Directories, Target: ".copilot/skills"},
			},
		},
		Antigravity: {
			Kind:         Antigravity,
			BuiltinKit:   "antigravity",
			ImageVariant: "shell",
			Experimental: true,
			Mappings: []AssetMapping{
				{Source: ".agents/skills", Kind: Directories, Target: ".agents/skills", Scope: Workspace},
			},
		},
	}}
}

// Get liefert die Definition eines Agents.
func (r *Registry) Get(kind Kind) (Definition, error) {
	def, ok := r.defs[kind]
	if !ok {
		return Definition{}, fmt.Errorf("unbekannter Agent %q (verfügbar: %s)", kind, strings.Join(kindStrings(r.Kinds()), ", "))
	}
	return def, nil
}

// Kinds liefert alle Agents sortiert.
func (r *Registry) Kinds() []Kind {
	kinds := make([]Kind, 0, len(r.defs))
	for k := range r.defs {
		kinds = append(kinds, k)
	}
	slices.Sort(kinds)
	return kinds
}

func kindStrings(kinds []Kind) []string {
	out := make([]string, len(kinds))
	for i, k := range kinds {
		out[i] = string(k)
	}
	return out
}

// Extend ergänzt Mappings eines Agents, etwa aus ~/.boxai/config.yaml.
func (r *Registry) Extend(kind Kind, mappings []AssetMapping) error {
	def, err := r.Get(kind)
	if err != nil {
		return err
	}
	for _, m := range mappings {
		if err := validateMapping(m); err != nil {
			return err
		}
		if m.Scope == Home && def.IsReserved(m.Target) {
			return fmt.Errorf("mapping-Ziel %q ist von sbx reserviert", m.Target)
		}
	}
	def.Mappings = append(slices.Clone(def.Mappings), mappings...)
	r.defs[kind] = def
	return nil
}

func validateMapping(m AssetMapping) error {
	for _, p := range []string{m.Source, m.Target} {
		clean := path.Clean(p)
		if p == "" || path.IsAbs(p) || clean == ".." || strings.HasPrefix(clean, "../") {
			return fmt.Errorf("ungültiger Mapping-Pfad %q: relativ und innerhalb des Ordners erforderlich", p)
		}
	}
	return nil
}

// IsReserved meldet, ob ein Pfad relativ zum Home von sbx verwaltet wird.
func (d Definition) IsReserved(rel string) bool {
	return slices.Contains(d.Reserved, path.Clean(rel))
}

// AssetFile ist eine Datei eines Assets.
type AssetFile struct {
	// Rel ist der Pfad relativ zum Asset; bei MarkdownFiles leer.
	Rel     string
	Content string
	Mode    uint32
}

// Asset ist eine Agent-, Skill- oder Command-Datei aus einem Repo.
type Asset struct {
	Repo    string
	Mapping AssetMapping
	Name    string
	Files   []AssetFile
}

// PlacedFile ist eine Datei mit Zielpfad relativ zum Scope.
type PlacedFile struct {
	Scope   Scope
	Path    string
	Content string
	Mode    uint32
}

// Conflict meldet eine Namenskollision und wie sie aufgelöst wurde.
type Conflict struct {
	Target     string
	Name       string
	Repos      []string
	Resolution string
}

// Place wendet die Konfliktregel an und berechnet die Zielpfade.
func (d Definition) Place(assets []Asset) ([]PlacedFile, []Conflict, error) {
	type key struct {
		scope        Scope
		target, name string
	}
	byKey := map[key][]Asset{}
	var order []key
	for _, a := range assets {
		k := key{a.Mapping.Scope, a.Mapping.Target, a.Name}
		if _, seen := byKey[k]; !seen {
			order = append(order, k)
		}
		byKey[k] = append(byKey[k], a)
	}

	var placed []PlacedFile
	var conflicts []Conflict
	for _, k := range order {
		group, dropped := dedupeRepos(byKey[k])
		for _, repo := range dropped {
			conflicts = append(conflicts, Conflict{
				Target: k.target, Name: k.name, Repos: []string{repo},
				Resolution: "doppelt im selben Repo, das zuerst gemappte gilt",
			})
		}
		rename := len(group) > 1
		var repos []string
		for _, a := range group {
			repos = append(repos, a.Repo)
			name := a.Name
			if rename {
				name = a.Repo + "-" + a.Name
			}
			files, err := d.placeAsset(a, name)
			if err != nil {
				return nil, nil, err
			}
			placed = append(placed, files...)
		}
		if rename {
			conflicts = append(conflicts, Conflict{
				Target: k.target, Name: k.name, Repos: repos,
				Resolution: "abgelegt als <repo>-" + k.name,
			})
		}
	}
	return placed, conflicts, nil
}

func dedupeRepos(group []Asset) (unique []Asset, dropped []string) {
	seen := map[string]bool{}
	for _, a := range group {
		if seen[a.Repo] {
			dropped = append(dropped, a.Repo)
			continue
		}
		seen[a.Repo] = true
		unique = append(unique, a)
	}
	return unique, dropped
}

func (d Definition) placeAsset(a Asset, name string) ([]PlacedFile, error) {
	out := make([]PlacedFile, 0, len(a.Files))
	for _, f := range a.Files {
		target := path.Join(a.Mapping.Target, name, f.Rel)
		if a.Mapping.Scope == Home && d.IsReserved(target) {
			return nil, fmt.Errorf("asset %s aus %s zielt auf den reservierten Pfad %s", a.Name, a.Repo, target)
		}
		out = append(out, PlacedFile{Scope: a.Mapping.Scope, Path: target, Content: f.Content, Mode: f.Mode})
	}
	return out, nil
}
