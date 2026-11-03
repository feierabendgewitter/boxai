package fsscan

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/feierabendgewitter/boxai/internal/adapter/out/execx"
	"github.com/feierabendgewitter/boxai/internal/adapter/out/gitcli"
	"github.com/feierabendgewitter/boxai/internal/domain/workspace"
)

// copyFixture kopiert testdata/tree. Git versioniert keine `.git`-Ordner, deshalb
// heißen sie im Fixture `dot-git` und `dot-git-file` (für Worktrees).
func copyFixture(t *testing.T) string {
	t.Helper()
	dest := t.TempDir()
	err := filepath.WalkDir("testdata/tree", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel("testdata/tree", path)
		segments := strings.Split(rel, string(filepath.Separator))
		for i, segment := range segments {
			if segment == "dot-git" || segment == "dot-git-file" {
				segments[i] = ".git"
			}
		}
		target := filepath.Join(dest, filepath.Join(segments...))
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, content, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	return dest
}

func names(repos []workspace.RepoRef) []string {
	out := []string{}
	for _, r := range repos {
		out = append(out, r.Name)
	}
	slices.Sort(out)
	return out
}

func TestScanFixtureTree(t *testing.T) {
	root := copyFixture(t)
	repos, err := New().Scan(context.Background(), root, 3)
	if err != nil {
		t.Fatal(err)
	}
	got := names(repos)
	want := []string{"backend", "frontend", "nested"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestScanRespectsDepth(t *testing.T) {
	root := copyFixture(t)
	repos, err := New().Scan(context.Background(), root, 1)
	if err != nil {
		t.Fatal(err)
	}
	if got := names(repos); !slices.Equal(got, []string{"backend", "frontend"}) {
		t.Fatalf("got %v", got)
	}
}

func TestScanStopsAtRepoBoundary(t *testing.T) {
	root := copyFixture(t)
	repos, err := New().Scan(context.Background(), root, 5)
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(names(repos), "inside-repo") {
		t.Fatal("Repo innerhalb eines Repos gefunden")
	}
}

func TestScanCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := New().Scan(ctx, copyFixture(t), 3); err == nil {
		t.Fatal("abgebrochener Kontext ignoriert")
	}
}

func TestScanMissingRoot(t *testing.T) {
	if _, err := New().Scan(context.Background(), filepath.Join(t.TempDir(), "fehlt"), 3); err == nil {
		t.Fatal("fehlendes Verzeichnis akzeptiert")
	}
}

// Integrationstest gegen echte Repos, inklusive echtem Worktree und Bare-Repo.
func TestScanRealGitRepos(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	git := gitcli.New(&execx.Exec{})
	ctx := context.Background()
	run := func(args ...string) {
		t.Helper()
		if _, err := (&execx.Exec{}).Run(ctx, execx.Cmd{Name: "git", Args: args}); err != nil {
			t.Fatal(err)
		}
	}
	main := filepath.Join(root, "main-repo")
	run("init", "-q", "-b", "main", main)
	run("-C", main, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-q", "--allow-empty", "-m", "init")
	run("init", "-q", "--bare", filepath.Join(root, "bare.git"))
	wt := workspace.Worktree{RepoPath: main, Path: filepath.Join(root, "wt"), Branch: "x", Base: "main", CreateBranch: true}
	if err := git.AddWorktree(ctx, wt); err != nil {
		t.Fatal(err)
	}
	repos, err := New().Scan(ctx, root, 3)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(names(repos), ","); got != "main-repo" {
		t.Fatalf("got %s", got)
	}
}
