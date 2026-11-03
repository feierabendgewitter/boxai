package gitcli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/feierabendgewitter/boxai/internal/adapter/out/execx"
	"github.com/feierabendgewitter/boxai/internal/domain/workspace"
)

type fixture struct {
	t    *testing.T
	root string
	repo string
	git  *Git
	ctx  context.Context
}

func setup(t *testing.T) fixture {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	f := fixture{t: t, root: root, repo: filepath.Join(root, "repo"), git: New(&execx.Exec{}), ctx: context.Background()}
	f.run("init", "-q", "-b", "main", f.repo)
	f.commit("init")
	return f
}

func (f fixture) run(args ...string) string {
	f.t.Helper()
	res, err := (&execx.Exec{}).Run(f.ctx, execx.Cmd{Name: "git", Args: args})
	if err != nil {
		f.t.Fatal(err)
	}
	return strings.TrimSpace(res.Stdout)
}

func (f fixture) commit(msg string) {
	f.t.Helper()
	f.run("-C", f.repo, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-q", "--allow-empty", "-m", msg)
}

func (f fixture) worktree(name string) workspace.Worktree {
	return workspace.Worktree{RepoName: "repo", RepoPath: f.repo, Path: filepath.Join(f.root, name), Branch: "boxai/" + name, Base: "main", CreateBranch: true}
}

func TestBranches(t *testing.T) {
	f := setup(t)
	if got, err := f.git.CurrentBranch(f.ctx, f.repo); err != nil || got != "main" {
		t.Fatal(got, err)
	}
	if got, err := f.git.DefaultBranch(f.ctx, f.repo); err != nil || got != "main" {
		t.Fatal(got, err)
	}
	if ok, err := f.git.BranchExists(f.ctx, f.repo, "main"); err != nil || !ok {
		t.Fatal(ok, err)
	}
	if ok, err := f.git.BranchExists(f.ctx, f.repo, "fehlt"); err != nil || ok {
		t.Fatal(ok, err)
	}
}

func TestDefaultBranchFallsBackToCurrent(t *testing.T) {
	f := setup(t)
	f.run("-C", f.repo, "branch", "-m", "main", "trunk")
	if got, err := f.git.DefaultBranch(f.ctx, f.repo); err != nil || got != "trunk" {
		t.Fatal(got, err)
	}
}

func TestDefaultBranchUsesOriginHead(t *testing.T) {
	f := setup(t)
	f.run("-C", f.repo, "branch", "-m", "main", "develop")
	clone := filepath.Join(f.root, "clone")
	if err := f.git.Clone(f.ctx, f.repo, clone); err != nil {
		t.Fatal(err)
	}
	if got, err := f.git.DefaultBranch(f.ctx, clone); err != nil || got != "develop" {
		t.Fatal(got, err)
	}
	if err := f.git.Fetch(f.ctx, clone); err != nil {
		t.Fatal(err)
	}
}

func TestDetachedHead(t *testing.T) {
	f := setup(t)
	f.run("-C", f.repo, "checkout", "-q", "--detach")
	got, err := f.git.CurrentBranch(f.ctx, f.repo)
	if err != nil || got == "" || got == "main" {
		t.Fatal(got, err)
	}
}

func TestIsDirty(t *testing.T) {
	f := setup(t)
	if dirty, err := f.git.IsDirty(f.ctx, f.repo); err != nil || dirty {
		t.Fatal(dirty, err)
	}
	if err := os.WriteFile(filepath.Join(f.repo, "x"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if dirty, err := f.git.IsDirty(f.ctx, f.repo); err != nil || !dirty {
		t.Fatal(dirty, err)
	}
}

func TestWorktreeLifecycle(t *testing.T) {
	f := setup(t)
	wt := f.worktree("wt")
	if err := f.git.AddWorktree(f.ctx, wt); err != nil {
		t.Fatal(err)
	}
	if got, _ := f.git.CurrentBranch(f.ctx, wt.Path); got != "boxai/wt" {
		t.Fatal(got)
	}

	// CommonDir zeigt von Worktree und Repo auf denselben Ordner.
	fromWorktree, err := f.git.CommonDir(f.ctx, wt.Path)
	if err != nil {
		t.Fatal(err)
	}
	fromRepo, err := f.git.CommonDir(f.ctx, f.repo)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(f.repo, ".git"); fromWorktree != want || fromRepo != want {
		t.Fatalf("%s, %s", fromWorktree, fromRepo)
	}

	// Dirty-Worktree: Ohne force bricht git ab.
	if err := os.WriteFile(filepath.Join(wt.Path, "x"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := f.git.RemoveWorktree(f.ctx, wt, false); err == nil {
		t.Fatal("Dirty-Worktree ohne force entfernt")
	}
	if err := f.git.RemoveWorktree(f.ctx, wt, true); err != nil {
		t.Fatal(err)
	}
	if err := f.git.PruneWorktrees(f.ctx, f.repo); err != nil {
		t.Fatal(err)
	}
	if err := f.git.DeleteBranch(f.ctx, f.repo, wt.Branch); err != nil {
		t.Fatal(err)
	}
	if ok, _ := f.git.BranchExists(f.ctx, f.repo, wt.Branch); ok {
		t.Fatal("Branch existiert noch")
	}
}

func TestAddWorktreeCheckoutExistingBranch(t *testing.T) {
	f := setup(t)
	f.run("-C", f.repo, "branch", "develop")
	wt := workspace.Worktree{RepoPath: f.repo, Path: filepath.Join(f.root, "dev"), Branch: "develop"}
	if err := f.git.AddWorktree(f.ctx, wt); err != nil {
		t.Fatal(err)
	}
	if got, _ := f.git.CurrentBranch(f.ctx, wt.Path); got != "develop" {
		t.Fatal(got)
	}
}

// Der Worktree-Fix hängt daran, dass die .git-Datei auf einen absoluten Pfad zeigt.
func TestWorktreeGitFilePointsIntoCommonDir(t *testing.T) {
	f := setup(t)
	wt := f.worktree("wt")
	if err := f.git.AddWorktree(f.ctx, wt); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(wt.Path, ".git"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), filepath.Join(f.repo, ".git")) {
		t.Fatal(string(content))
	}
}

func TestErrorsFromMissingRepo(t *testing.T) {
	f := setup(t)
	missing := filepath.Join(f.root, "fehlt")
	if _, err := f.git.CommonDir(f.ctx, missing); err == nil {
		t.Error("CommonDir ohne Repo")
	}
	if _, err := f.git.IsDirty(f.ctx, missing); err == nil {
		t.Error("IsDirty ohne Repo")
	}
}
