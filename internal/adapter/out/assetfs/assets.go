// Package assetfs implementiert port.AssetSource über das Dateisystem.
package assetfs

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/feierabendgewitter/boxai/internal/app/port"
	"github.com/feierabendgewitter/boxai/internal/domain/agent"
)

// maxFileSize begrenzt Dateien, weil sie inline im Kit-Deskriptor landen.
const maxFileSize = 1 << 20

var instructionFiles = []string{"AGENTS.md", "CLAUDE.md", ".github/copilot-instructions.md"}

// Source implementiert port.AssetSource.
type Source struct{}

// New erzeugt den Adapter.
func New() *Source { return &Source{} }

// Collect liest die Assets aller Mappings. Fehlende Ordner sind normal.
func (s *Source) Collect(_ context.Context, repo, worktree string, mappings []agent.AssetMapping) (port.AssetResult, error) {
	var res port.AssetResult
	for _, m := range mappings {
		dir := filepath.Join(worktree, filepath.FromSlash(m.Source))
		entries, err := os.ReadDir(dir)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return res, err
		}
		for _, e := range entries {
			asset, skipped, ok, err := readEntry(repo, m, dir, e)
			if err != nil {
				return res, err
			}
			res.Skipped = append(res.Skipped, skipped...)
			if ok {
				res.Assets = append(res.Assets, asset)
			}
		}
	}
	return res, nil
}

func readEntry(repo string, m agent.AssetMapping, dir string, e os.DirEntry) (agent.Asset, []string, bool, error) {
	asset := agent.Asset{Repo: repo, Mapping: m, Name: e.Name()}
	path := filepath.Join(dir, e.Name())
	switch m.Kind {
	case agent.MarkdownFiles:
		if !e.Type().IsRegular() || !strings.HasSuffix(e.Name(), ".md") {
			return asset, nil, false, nil
		}
		content, mode, usable, err := readText(path)
		if err != nil {
			return asset, nil, false, err
		}
		if !usable {
			return asset, []string{filepath.Join(m.Source, e.Name())}, false, nil
		}
		asset.Files = []agent.AssetFile{{Content: content, Mode: mode}}
		return asset, nil, true, nil
	default:
		if !e.IsDir() {
			return asset, nil, false, nil
		}
		files, skipped, err := readTree(path, filepath.Join(m.Source, e.Name()))
		if err != nil {
			return asset, nil, false, err
		}
		asset.Files = files
		return asset, skipped, len(files) > 0, nil
	}
}

func readTree(root, display string) ([]agent.AssetFile, []string, error) {
	var files []agent.AssetFile
	var skipped []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if !d.Type().IsRegular() {
			skipped = append(skipped, filepath.Join(display, rel))
			return nil
		}
		content, mode, usable, err := readText(path)
		if err != nil {
			return err
		}
		if !usable {
			skipped = append(skipped, filepath.Join(display, rel))
			return nil
		}
		files = append(files, agent.AssetFile{Rel: filepath.ToSlash(rel), Content: content, Mode: mode})
		return nil
	})
	return files, skipped, err
}

// readText liefert false, wenn die Datei nicht als Text in den Deskriptor passt.
func readText(path string) (string, uint32, bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", 0, false, err
	}
	if info.Size() > maxFileSize {
		return "", 0, false, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", 0, false, err
	}
	if !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 {
		return "", 0, false, nil
	}
	return string(data), uint32(info.Mode().Perm()), true, nil
}

// RepoContext findet das OKF-Bundle und die Anweisungsdateien eines Worktrees.
func (s *Source) RepoContext(_ context.Context, worktree string) (string, []string, error) {
	var instructions []string
	for _, name := range instructionFiles {
		if _, err := os.Stat(filepath.Join(worktree, filepath.FromSlash(name))); err == nil {
			instructions = append(instructions, name)
		}
	}
	okf, err := findBundle(worktree)
	return okf, instructions, err
}

// findBundle prüft `.okf`, dann Ordner der ersten Ebene und unter docs/.
func findBundle(worktree string) (string, error) {
	candidates := []string{".okf"}
	for _, parent := range []string{"", "docs"} {
		entries, err := os.ReadDir(filepath.Join(worktree, parent))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		for _, e := range entries {
			if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
				candidates = append(candidates, filepath.ToSlash(filepath.Join(parent, e.Name())))
			}
		}
	}
	for _, dir := range candidates {
		if isBundle(filepath.Join(worktree, filepath.FromSlash(dir))) {
			return dir + "/index.md", nil
		}
	}
	return "", nil
}

// isBundle erkennt ein OKF-Bundle an index.md plus einem Konzept mit `type`-Frontmatter.
func isBundle(dir string) bool {
	if _, err := os.Stat(filepath.Join(dir, "index.md")); err != nil {
		return false
	}
	found := false
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || found {
			return filepath.SkipAll
		}
		name := d.Name()
		if d.IsDir() || !strings.HasSuffix(name, ".md") || slices.Contains([]string{"index.md", "log.md"}, name) {
			return nil
		}
		found = hasTypeFrontmatter(path)
		return nil
	})
	return found
}

func hasTypeFrontmatter(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	body, ok := strings.CutPrefix(string(data), "---\n")
	if !ok {
		return false
	}
	head, _, closed := strings.Cut(body, "\n---")
	if !closed {
		return false
	}
	for line := range strings.SplitSeq(head, "\n") {
		if value, ok := strings.CutPrefix(line, "type:"); ok && strings.TrimSpace(value) != "" {
			return true
		}
	}
	return false
}
