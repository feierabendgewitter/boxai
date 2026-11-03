package removeworkspace

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/feierabendgewitter/boxai/internal/app/port"
	"github.com/feierabendgewitter/boxai/internal/app/port/porttest"
	"github.com/feierabendgewitter/boxai/internal/domain/sandbox"
	"github.com/feierabendgewitter/boxai/internal/domain/workspace"
)

type env struct {
	journal  *porttest.Journal
	git      *porttest.Git
	runtime  *porttest.Runtime
	secrets  *porttest.Secrets
	kits     *porttest.Kits
	state    *porttest.State
	notifier *porttest.Notifier
	svc      *Service
}

func newEnv() *env {
	j := &porttest.Journal{}
	e := &env{journal: j, git: porttest.NewGit(), runtime: porttest.NewRuntime(), secrets: porttest.NewSecrets(), kits: porttest.NewKits(), state: porttest.NewState(), notifier: &porttest.Notifier{}}
	e.git.Journal, e.runtime.Journal, e.secrets.Journal, e.kits.Journal, e.state.Journal = j, j, j, j, j
	wts := []workspace.Worktree{
		{RepoName: "api", RepoPath: "/src/api", Path: "/ws/shop/api", Branch: "boxai/shop"},
		{RepoName: "ui", RepoPath: "/src/ui", Path: "/ws/shop/ui", Branch: "develop"},
	}
	e.state.States["shop"] = port.WorkspaceState{
		Name: "shop", Path: "/ws/shop", SandboxName: "boxai-shop", KitDir: "/kits/shop", Worktrees: wts,
		CreatedBranches: map[string]string{"/src/api": "boxai/shop"},
	}
	e.runtime.Sandboxes["boxai-shop"] = sandbox.Info{Name: "boxai-shop"}
	e.git.Branches["/src/api@boxai/shop"] = true
	e.git.Branches["/src/ui@develop"] = true
	e.svc = New(e.git, e.runtime, e.secrets, e.kits, e.state, e.notifier)
	return e
}

func TestRemoveCleansEverythingButKeepsBranches(t *testing.T) {
	e := newEnv()
	if err := e.svc.Remove(context.Background(), port.RemoveRequest{Name: "shop"}); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"sandbox remove boxai-shop", "secret delete-all boxai-shop",
		"worktree remove api force=false", "worktree remove ui force=false",
		"kit remove", "workspace-root remove", "state delete shop",
	}
	if !slices.Equal(e.journal.Entries, want) {
		t.Fatalf("%v", e.journal.Entries)
	}
	if !e.git.Branches["/src/api@boxai/shop"] {
		t.Fatal("Branch ohne --delete-branches gelöscht")
	}
	if len(e.git.Pruned) != 2 || e.kits.RootsRemoved[0] != "/ws/shop" {
		t.Fatalf("%v %v", e.git.Pruned, e.kits.RootsRemoved)
	}
}

func TestDeleteBranchesOnlyTouchesOwnBranches(t *testing.T) {
	e := newEnv()
	if err := e.svc.Remove(context.Background(), port.RemoveRequest{Name: "shop", DeleteBranches: true}); err != nil {
		t.Fatal(err)
	}
	if e.git.Branches["/src/api@boxai/shop"] {
		t.Fatal("eigener Branch noch da")
	}
	if !e.git.Branches["/src/ui@develop"] {
		t.Fatal("--checkout-Branch gelöscht")
	}
}

func TestDirtyWorktreeAbortsBeforeAnyChange(t *testing.T) {
	e := newEnv()
	e.git.Dirty["/ws/shop/ui"] = true
	err := e.svc.Remove(context.Background(), port.RemoveRequest{Name: "shop"})
	if err == nil || !strings.Contains(err.Error(), "ui") || !strings.Contains(err.Error(), "--force") {
		t.Fatal(err)
	}
	if len(e.journal.Entries) != 0 {
		t.Fatalf("%v", e.journal.Entries)
	}
}

func TestForceSkipsDirtyCheck(t *testing.T) {
	e := newEnv()
	e.git.Dirty["/ws/shop/ui"] = true
	e.git.FailOn["IsDirty"] = errors.New("worktree fehlt")
	if err := e.svc.Remove(context.Background(), port.RemoveRequest{Name: "shop", Force: true}); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(e.journal.Entries, "worktree remove ui force=true") {
		t.Fatalf("%v", e.journal.Entries)
	}
}

func TestLeftoversAreReported(t *testing.T) {
	e := newEnv()
	e.kits.RootLeftovers = []string{"notizen.txt"}
	if err := e.svc.Remove(context.Background(), port.RemoveRequest{Name: "shop"}); err != nil {
		t.Fatal(err)
	}
	if len(e.notifier.Warns) != 1 || !strings.Contains(e.notifier.Warns[0], "notizen.txt") {
		t.Fatal(e.notifier.Warns)
	}
}

func TestUnknownWorkspace(t *testing.T) {
	if err := newEnv().svc.Remove(context.Background(), port.RemoveRequest{Name: "fehlt"}); !errors.Is(err, port.ErrStateNotFound) {
		t.Fatal(err)
	}
}

func TestFailuresKeepStateForRetry(t *testing.T) {
	boom := errors.New("boom")
	tests := map[string]func(*env){
		"Sandbox":   func(e *env) { e.runtime.FailOn["Remove"] = boom },
		"Secrets":   func(e *env) { e.secrets.FailOn["DeleteAll"] = boom },
		"Worktree":  func(e *env) { e.git.FailOn["RemoveWorktree"] = boom },
		"Prune":     func(e *env) { e.git.FailOn["PruneWorktrees"] = boom },
		"Kit":       func(e *env) { e.kits.FailOn["Remove"] = boom },
		"Root":      func(e *env) { e.kits.FailOn["RemoveWorkspaceRoot"] = boom },
		"IsDirty":   func(e *env) { e.git.FailOn["IsDirty"] = boom },
		"DelBranch": func(e *env) { e.git.FailOn["DeleteBranch"] = boom },
	}
	for name, edit := range tests {
		t.Run(name, func(t *testing.T) {
			e := newEnv()
			edit(e)
			err := e.svc.Remove(context.Background(), port.RemoveRequest{Name: "shop", DeleteBranches: true})
			if !errors.Is(err, boom) {
				t.Fatal(err)
			}
			if _, ok := e.state.States["shop"]; !ok {
				t.Fatal("State gelöscht, kein erneuter Versuch möglich")
			}
		})
	}
}
