package statejson

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/feierabendgewitter/boxai/internal/app/port"
	"github.com/feierabendgewitter/boxai/internal/domain/workspace"
)

func state(name string) port.WorkspaceState {
	return port.WorkspaceState{
		Name: name, Path: "/ws/" + name, SandboxName: "boxai-" + name, Agent: "claude",
		Status:          port.StatusReady,
		Worktrees:       []workspace.Worktree{{RepoName: "api", RepoPath: "/src/api", Path: "/ws/x/api", Branch: "b"}},
		CreatedBranches: map[string]string{"/src/api": "b"},
		CreatedAt:       time.Unix(100, 0).UTC(),
	}
}

func TestRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := New(filepath.Join(t.TempDir(), "state"), false)
	want := state("shop")
	if err := s.Save(ctx, want); err != nil {
		t.Fatal(err)
	}
	got, err := s.Load(ctx, "shop")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != want.Name || got.Worktrees[0].Branch != "b" || got.CreatedBranches["/src/api"] != "b" || !got.CreatedAt.Equal(want.CreatedAt) {
		t.Fatalf("%+v", got)
	}
}

func TestFileAndDirectoryArePrivate(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state")
	if err := New(dir, false).Save(context.Background(), state("shop")); err != nil {
		t.Fatal(err)
	}
	file, err := os.Stat(filepath.Join(dir, "shop.json"))
	if err != nil {
		t.Fatal(err)
	}
	if perm := file.Mode().Perm(); perm != 0o600 {
		t.Fatalf("Datei %o", perm)
	}
	folder, _ := os.Stat(dir)
	if perm := folder.Mode().Perm(); perm != 0o700 {
		t.Fatalf("Ordner %o", perm)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("temporäre Datei übrig: %v", entries)
	}
}

func TestLoadMissing(t *testing.T) {
	if _, err := New(t.TempDir(), false).Load(context.Background(), "fehlt"); !errors.Is(err, port.ErrStateNotFound) {
		t.Fatal(err)
	}
}

func TestLoadCorrupt(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "shop.json"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := New(dir, false).Load(context.Background(), "shop"); err == nil || errors.Is(err, port.ErrStateNotFound) {
		t.Fatal(err)
	}
}

func TestListSortedAndDelete(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s := New(dir, false)
	for _, n := range []string{"zz", "aa"} {
		if err := s.Save(ctx, state(n)); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "notizen.txt"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	list, err := s.List(ctx)
	if err != nil || len(list) != 2 || list[0].Name != "aa" {
		t.Fatalf("%+v %v", list, err)
	}
	if err := s.Delete(ctx, "aa"); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(ctx, "aa"); err != nil {
		t.Fatal("zweites Löschen muss gehen:", err)
	}
	if list, _ := s.List(ctx); len(list) != 1 {
		t.Fatalf("%+v", list)
	}
}

func TestListMissingDirectory(t *testing.T) {
	list, err := New(filepath.Join(t.TempDir(), "fehlt"), false).List(context.Background())
	if err != nil || list != nil {
		t.Fatal(list, err)
	}
}

func TestDryRunWritesNothing(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state")
	s := New(dir, true)
	if err := s.Save(context.Background(), state("shop")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("Dry-Run hat geschrieben")
	}
	if err := s.Delete(context.Background(), "shop"); err != nil {
		t.Fatal(err)
	}
}
