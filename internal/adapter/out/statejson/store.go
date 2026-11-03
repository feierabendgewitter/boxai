// Package statejson implementiert port.StateStore mit JSON-Dateien.
package statejson

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/feierabendgewitter/boxai/internal/app/port"
)

// Store implementiert port.StateStore unter einem Verzeichnis.
type Store struct {
	dir string
	// dryRun verhindert jedes Schreiben.
	dryRun bool
}

// New erzeugt den Store.
func New(dir string, dryRun bool) *Store { return &Store{dir: dir, dryRun: dryRun} }

func (s *Store) path(name string) string { return filepath.Join(s.dir, name+".json") }

// Save schreibt atomar, damit ein Abbruch keine halbe Datei hinterlässt.
func (s *Store) Save(_ context.Context, st port.WorkspaceState) error {
	if s.dryRun {
		return nil
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(s.dir, st.Name+".*.tmp")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), s.path(st.Name))
}

// Load implementiert port.StateStore.
func (s *Store) Load(_ context.Context, name string) (port.WorkspaceState, error) {
	data, err := os.ReadFile(s.path(name))
	if errors.Is(err, os.ErrNotExist) {
		return port.WorkspaceState{}, fmt.Errorf("%w: %s", port.ErrStateNotFound, name)
	}
	if err != nil {
		return port.WorkspaceState{}, err
	}
	var st port.WorkspaceState
	if err := json.Unmarshal(data, &st); err != nil {
		return port.WorkspaceState{}, fmt.Errorf("state %s: %w", name, err)
	}
	return st, nil
}

// List liefert alle Zustände sortiert nach Namen.
func (s *Store) List(ctx context.Context) ([]port.WorkspaceState, error) {
	entries, err := os.ReadDir(s.dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var states []port.WorkspaceState
	for _, e := range entries {
		name, ok := strings.CutSuffix(e.Name(), ".json")
		if !ok {
			continue
		}
		st, err := s.Load(ctx, name)
		if err != nil {
			return nil, err
		}
		states = append(states, st)
	}
	slices.SortFunc(states, func(a, b port.WorkspaceState) int { return strings.Compare(a.Name, b.Name) })
	return states, nil
}

// Delete implementiert port.StateStore; ein fehlender State ist kein Fehler.
func (s *Store) Delete(_ context.Context, name string) error {
	if s.dryRun {
		return nil
	}
	err := os.Remove(s.path(name))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
