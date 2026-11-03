// Package fsscan implementiert port.RepoScanner über filepath.WalkDir.
package fsscan

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/feierabendgewitter/boxai/internal/domain/workspace"
)

var skipped = []string{"node_modules", "build", "target", "vendor"}

// Scanner implementiert port.RepoScanner.
type Scanner struct{}

// New erzeugt den Scanner.
func New() *Scanner { return &Scanner{} }

// Scan findet Verzeichnisse mit `.git`-Ordner bis zur Tiefe depth. Bestehende
// Worktrees (`.git` ist eine Datei) und Bare-Repos erscheinen nicht.
func (s *Scanner) Scan(ctx context.Context, root string, depth int) ([]workspace.RepoRef, error) {
	root, err := Normalize(root)
	if err != nil {
		return nil, err
	}
	var repos []workspace.RepoRef
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			// Unlesbare Ordner sollen den Scan nicht beenden.
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		if path != root && shouldSkip(d.Name()) {
			return filepath.SkipDir
		}
		switch classify(path) {
		case kindRepo:
			repos = append(repos, workspace.RepoRef{Name: filepath.Base(path), Path: path})
			return filepath.SkipDir
		case kindWorktree, kindBare:
			return filepath.SkipDir
		}
		if levels(root, path) >= depth {
			return filepath.SkipDir
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return repos, nil
}

// Normalize löst Symlinks auf, damit Pfade mit den absoluten Pfaden in
// `.git`-Dateien übereinstimmen.
func Normalize(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(abs)
}

func shouldSkip(name string) bool {
	return strings.HasPrefix(name, ".") || slices.Contains(skipped, name)
}

func levels(root, path string) int {
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == "." {
		return 0
	}
	return strings.Count(rel, string(filepath.Separator)) + 1
}

type repoKind int

const (
	kindNone repoKind = iota
	kindRepo
	kindWorktree
	kindBare
)

func classify(dir string) repoKind {
	if info, err := os.Lstat(filepath.Join(dir, ".git")); err == nil {
		if info.IsDir() {
			return kindRepo
		}
		return kindWorktree
	}
	for _, marker := range []string{"HEAD", "objects", "refs"} {
		if _, err := os.Stat(filepath.Join(dir, marker)); err != nil {
			return kindNone
		}
	}
	return kindBare
}
