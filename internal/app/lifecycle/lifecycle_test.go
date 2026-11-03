package lifecycle

import (
	"context"
	"errors"
	"testing"

	"github.com/feierabendgewitter/boxai/internal/app/port"
	"github.com/feierabendgewitter/boxai/internal/app/port/porttest"
	"github.com/feierabendgewitter/boxai/internal/domain/sandbox"
	"github.com/feierabendgewitter/boxai/internal/domain/workspace"
)

func setup() (*Service, *porttest.Runtime, *porttest.State) {
	runtime, state := porttest.NewRuntime(), porttest.NewState()
	state.States["shop"] = port.WorkspaceState{Name: "shop", SandboxName: "boxai-shop", Worktrees: []workspace.Worktree{{RepoName: "api"}}}
	state.States["old"] = port.WorkspaceState{Name: "old", SandboxName: "boxai-old"}
	runtime.Sandboxes["boxai-shop"] = sandbox.Info{Name: "boxai-shop", Status: "running"}
	runtime.Sandboxes["copilot-fremd"] = sandbox.Info{Name: "copilot-fremd", Status: "stopped"}
	return New(runtime, state), runtime, state
}

func TestAttachAndStopUseSandboxName(t *testing.T) {
	svc, runtime, _ := setup()
	ctx := context.Background()
	if err := svc.Attach(ctx, "shop"); err != nil {
		t.Fatal(err)
	}
	if err := svc.Stop(ctx, "shop"); err != nil {
		t.Fatal(err)
	}
	if runtime.Attached[0] != "boxai-shop" {
		t.Fatal(runtime.Attached)
	}
	if err := svc.Attach(ctx, "fehlt"); !errors.Is(err, port.ErrStateNotFound) {
		t.Fatal(err)
	}
	if err := svc.Stop(ctx, "fehlt"); !errors.Is(err, port.ErrStateNotFound) {
		t.Fatal(err)
	}
}

func TestListJoinsStatusAndHidesForeignSandboxes(t *testing.T) {
	svc, runtime, state := setup()
	infos, err := svc.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(infos) != 2 {
		t.Fatalf("%+v", infos)
	}
	if infos[0].State.Name != "old" || infos[0].SandboxStatus != StatusMissing {
		t.Fatalf("%+v", infos[0])
	}
	if infos[1].SandboxStatus != "running" || infos[1].Repos[0] != "api" {
		t.Fatalf("%+v", infos[1])
	}
	boom := errors.New("boom")
	runtime.FailOn["List"] = boom
	if _, err := svc.List(context.Background()); !errors.Is(err, boom) {
		t.Fatal(err)
	}
	state.FailOn["List"] = boom
	if _, err := svc.List(context.Background()); !errors.Is(err, boom) {
		t.Fatal(err)
	}
}
