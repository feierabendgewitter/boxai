// Package sandbox enthält das formatfreie Modell von Sandbox und Kit.
package sandbox

import (
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/feierabendgewitter/boxai/internal/domain/agent"
	"github.com/feierabendgewitter/boxai/internal/domain/profile"
	"github.com/feierabendgewitter/boxai/internal/domain/secret"
	"github.com/feierabendgewitter/boxai/internal/domain/workspace"
)

// HomeDir ist das Home des Sandbox-Nutzers.
const HomeDir = "/home/agent"

// Mount ist ein zusätzlicher Workspace unter seinem Host-Pfad.
type Mount struct {
	Path     string
	ReadOnly bool
}

// SandboxSpec beschreibt eine zu erzeugende Sandbox.
type SandboxSpec struct {
	Name string
	// Agent ist ein Built-in-Name oder ein Kit-Verzeichnis.
	Agent     string
	Workspace string
	Mounts    []Mount
	// KitDir ist der generierte Mixin; leer, wenn es nichts zu kopieren gibt.
	KitDir string
	// Template ist das Image; leer lässt sbx das Agent-Image wählen.
	Template string
	// LocalTemplate setzt `--pull never` für lokal geladene Images.
	LocalTemplate bool
	// EnvSecrets sind Secrets im Fallback-Modus secrets.mode: env.
	EnvSecrets []secret.SecretSpec
}

// Info ist der Laufzeitstatus einer Sandbox.
type Info struct {
	Name   string
	Agent  string
	Status string
}

// KitFile ist eine Datei mit absolutem Zielpfad in der Sandbox.
type KitFile struct {
	Path    string
	Content string
	Mode    uint32
}

// KitSpec ist der generierte Mixin als reines Modell.
type KitSpec struct {
	Name        string
	Description string
	Files       []KitFile
	// RootDir ist der Workspace-Ordner auf dem Host (in der Sandbox unter demselben Pfad).
	RootDir string
	// RootFiles liegen direkt im Workspace-Ordner statt im Deskriptor, weil sbx den
	// Deskriptor auf 512 KiB begrenzt. Path ist relativ zu RootDir.
	RootFiles []KitFile
}

// Empty meldet, ob der Kit nichts beitragen würde.
func (k KitSpec) Empty() bool { return len(k.Files) == 0 && len(k.RootFiles) == 0 }

// KitInput enthält alle Bausteine für BuildKit.
type KitInput struct {
	Workspace workspace.Workspace
	Agent     agent.Definition
	Assets    []agent.PlacedFile
	Config    []profile.ConfigFile
	Context   []ContextFile
}

// ContextFile ist eine generierte Datei im Workspace-Root.
type ContextFile struct {
	Name    string
	Content string
}

// BuildKit baut den Mixin. Assets aus Repos haben Vorrang vor Profil-Dateien, generierter
// Kontext vor beidem. Reservierte Pfade werden nie erzeugt.
func BuildKit(in KitInput) (KitSpec, []string, error) {
	kit := KitSpec{
		Name:        workspace.SandboxName(in.Workspace.Name),
		Description: "boxai-Workspace " + in.Workspace.Name,
		RootDir:     in.Workspace.Path,
	}
	var notes []string
	index := map[string]bool{}
	add := func(file KitFile) bool {
		if index[file.Path] {
			notes = append(notes, "übersprungen, Ziel schon belegt: "+file.Path)
			return false
		}
		index[file.Path] = true
		kit.Files = append(kit.Files, file)
		return true
	}

	for _, c := range in.Context {
		add(KitFile{Path: path.Join(in.Workspace.Path, c.Name), Content: c.Content, Mode: 0o644})
	}
	for _, a := range in.Assets {
		if a.Scope == agent.Workspace {
			abs := path.Join(in.Workspace.Path, a.Path)
			if add(KitFile{Path: abs, Content: a.Content, Mode: normalizeMode(a.Mode)}) {
				// Nicht in den Deskriptor, sondern direkt in den gemounteten Workspace.
				kit.Files = kit.Files[:len(kit.Files)-1]
				kit.RootFiles = append(kit.RootFiles, KitFile{Path: a.Path, Content: a.Content, Mode: normalizeMode(a.Mode)})
			}
			continue
		}
		add(KitFile{Path: path.Join(HomeDir, a.Path), Content: a.Content, Mode: normalizeMode(a.Mode)})
	}
	for _, f := range in.Config {
		if in.Agent.IsReserved(f.Path) {
			return KitSpec{}, nil, fmt.Errorf("profil-Datei %s liegt auf einem reservierten Pfad", f.Path)
		}
		add(KitFile{Path: path.Join(HomeDir, f.Path), Content: f.Content, Mode: normalizeMode(f.Mode)})
	}
	slices.SortFunc(kit.Files, func(a, b KitFile) int { return strings.Compare(a.Path, b.Path) })
	slices.SortFunc(kit.RootFiles, func(a, b KitFile) int { return strings.Compare(a.Path, b.Path) })
	return kit, notes, nil
}

func normalizeMode(mode uint32) uint32 {
	if mode == 0 {
		return 0o644
	}
	return mode
}

// RepoContext beschreibt ein Repo für die generierte AGENTS.md.
type RepoContext struct {
	Name   string
	Branch string
	// OKFIndex ist der Pfad zum OKF-Bundle relativ zum Repo, falls vorhanden.
	OKFIndex string
	// Instructions listet vorhandene Agent-Dateien im Repo.
	Instructions []string
}

// ContextInput enthält den Inhalt der generierten Root-Dateien.
type ContextInput struct {
	WorkspaceName string
	Repos         []RepoContext
	Profiles      []string
	Hints         []string
	Agent         agent.Definition
}

// RenderContext erzeugt die Root-AGENTS.md und für Claude zusätzlich eine CLAUDE.md.
func RenderContext(in ContextInput) []ContextFile {
	var b strings.Builder
	fmt.Fprintf(&b, "# Workspace %s\n\nDieser Ordner bündelt mehrere Git-Worktrees. Jedes Unterverzeichnis ist ein eigenes Repo mit eigener Historie; committe im jeweiligen Repo.\n\n", in.WorkspaceName)

	var okf []string
	for _, r := range in.Repos {
		if r.OKFIndex != "" {
			okf = append(okf, fmt.Sprintf("- `%s/%s`", r.Name, r.OKFIndex))
		}
	}
	if len(okf) > 0 {
		b.WriteString("## Zuerst lesen\n\nProjektkontext als OKF-Bundle; lies diese Dateien, bevor du im jeweiligen Repo arbeitest:\n\n")
		b.WriteString(strings.Join(okf, "\n"))
		b.WriteString("\n\n")
	}

	b.WriteString("## Repos\n\n| Repo | Branch | Anweisungen |\n| --- | --- | --- |\n")
	for _, r := range in.Repos {
		instructions := "–"
		if len(r.Instructions) > 0 {
			quoted := make([]string, len(r.Instructions))
			for i, f := range r.Instructions {
				quoted[i] = "`" + r.Name + "/" + f + "`"
			}
			instructions = strings.Join(quoted, ", ")
		}
		fmt.Fprintf(&b, "| `%s` | `%s` | %s |\n", r.Name, r.Branch, instructions)
	}
	if len(in.Profiles) > 0 {
		fmt.Fprintf(&b, "\n## Stack\n\nProfile: %s\n", strings.Join(in.Profiles, ", "))
	}
	if len(in.Hints) > 0 {
		b.WriteString("\n## Build-Kommandos\n\nIm jeweils passenden Repo:\n\n")
		for _, h := range in.Hints {
			fmt.Fprintf(&b, "- `%s`\n", h)
		}
	}

	files := []ContextFile{{Name: "AGENTS.md", Content: b.String()}}
	if in.Agent.WritesClaudeMD {
		files = append(files, ContextFile{Name: "CLAUDE.md", Content: "Lies zuerst [AGENTS.md](AGENTS.md).\n"})
	}
	return files
}
