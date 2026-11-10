// Package kitfs implementiert port.KitWriter: Es übersetzt sandbox.KitSpec in einen
// deklarativen v3-Mixin und stellt mitgelieferte Kits bereit.
package kitfs

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/feierabendgewitter/boxai/internal/adapter/out/execx"
	"github.com/feierabendgewitter/boxai/internal/domain/sandbox"
)

const syntaxLine = "# syntax=docker/sandbox-kit:3\n"

// generatedRootEntries sind die Einträge, die boxai im Workspace-Root erzeugt.
var generatedRootEntries = []string{"AGENTS.md", "CLAUDE.md", ".agents", ".claude"}

// Writer implementiert port.KitWriter.
type Writer struct {
	runner  execx.Runner
	builtin fs.FS
	root    string
	dryRun  bool
}

// New erzeugt den Adapter. root enthält die entpackten Built-in-Kits.
func New(runner execx.Runner, builtin fs.FS, root string, dryRun bool) *Writer {
	return &Writer{runner: runner, builtin: builtin, root: root, dryRun: dryRun}
}

type descriptor struct {
	SchemaVersion string       `yaml:"schemaVersion"`
	Kind          string       `yaml:"kind"`
	Description   string       `yaml:"description"`
	Capabilities  []capability `yaml:"capabilities"`
}

type capability struct {
	Type   string          `yaml:"type"`
	Config lifecycleConfig `yaml:"config"`
}

type lifecycleConfig struct {
	Files []descriptorFile `yaml:"files"`
}

type descriptorFile struct {
	Path    string `yaml:"path"`
	Content string `yaml:"content"`
	Mode    string `yaml:"mode"`
}

// Render erzeugt den Deskriptor. sbx verlangt die Syntax-Zeile in Zeile 1.
func Render(kit sandbox.KitSpec) ([]byte, error) {
	doc := descriptor{SchemaVersion: "3", Kind: "mixin", Description: kit.Description}
	if len(kit.Files) > 0 {
		files := make([]descriptorFile, len(kit.Files))
		for i, f := range kit.Files {
			files[i] = descriptorFile{Path: f.Path, Content: f.Content, Mode: fmt.Sprintf("%04o", f.Mode)}
		}
		doc.Capabilities = []capability{{
			Type:   "com.docker.sandbox/lifecycle@1",
			Config: lifecycleConfig{Files: files},
		}}
	}
	var buf bytes.Buffer
	buf.WriteString(syntaxLine)
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Write implementiert port.KitWriter und ersetzt einen vorhandenen Kit vollständig.
func (w *Writer) Write(_ context.Context, kit sandbox.KitSpec, dir string) error {
	data, err := Render(kit)
	if err != nil || w.dryRun {
		return err
	}
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, kit.Name+".yaml"), data, 0o600); err != nil {
		return err
	}
	return writeRootFiles(kit)
}

// writeRootFiles legt Workspace-Assets direkt im Workspace-Ordner ab. Der Ordner wird
// von boxai verwaltet; Reste eines früheren Laufs werden vorher entfernt.
func writeRootFiles(kit sandbox.KitSpec) error {
	if len(kit.RootFiles) == 0 {
		return nil
	}
	if kit.RootDir == "" {
		return errors.New("workspace-ordner für Workspace-Assets fehlt")
	}
	for _, name := range []string{".agents", ".claude"} {
		if err := os.RemoveAll(filepath.Join(kit.RootDir, name)); err != nil {
			return err
		}
	}
	for _, f := range kit.RootFiles {
		target := filepath.Join(kit.RootDir, filepath.FromSlash(path.Clean(f.Path)))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, []byte(f.Content), fs.FileMode(f.Mode)); err != nil {
			return err
		}
	}
	return nil
}

// Validate implementiert port.KitWriter. `--offline` lehnt v3-Quell-Kits ab, deshalb
// läuft die Prüfung über den Daemon.
func (w *Writer) Validate(ctx context.Context, dir string) error {
	_, err := w.runner.Run(ctx, execx.Cmd{Name: "sbx", Args: []string{"kit", "validate", "--no-input", dir}})
	return err
}

// Remove implementiert port.KitWriter.
func (w *Writer) Remove(_ context.Context, dir string) error {
	if w.dryRun {
		return nil
	}
	return os.RemoveAll(dir)
}

// RemoveWorkspaceRoot implementiert port.KitWriter.
func (w *Writer) RemoveWorkspaceRoot(_ context.Context, root string) ([]string, error) {
	if w.dryRun {
		return nil, nil
	}
	for _, name := range generatedRootEntries {
		if err := os.RemoveAll(filepath.Join(root, name)); err != nil {
			return nil, err
		}
	}
	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, os.Remove(root)
	}
	leftovers := make([]string, len(entries))
	for i, e := range entries {
		leftovers[i] = e.Name()
	}
	return leftovers, nil
}

// Builtin implementiert port.KitWriter und entpackt den Kit bei jedem Aufruf neu,
// damit ein Update von boxai auch den Kit aktualisiert.
func (w *Writer) Builtin(_ context.Context, name string) (string, error) {
	sub, err := fs.Sub(w.builtin, name)
	if err != nil {
		return "", err
	}
	if _, err := fs.Stat(sub, "."); err != nil {
		return "", fmt.Errorf("mitgelieferter Kit %s fehlt: %w", name, err)
	}
	dest := filepath.Join(w.root, name)
	if w.dryRun {
		return dest, nil
	}
	if err := os.RemoveAll(dest); err != nil {
		return "", err
	}
	err = fs.WalkDir(sub, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		target := filepath.Join(dest, filepath.FromSlash(path.Clean(p)))
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := fs.ReadFile(sub, p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	return dest, err
}
