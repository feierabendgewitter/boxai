package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/feierabendgewitter/boxai/internal/app/port"
	"github.com/feierabendgewitter/boxai/internal/domain/workspace"
)

type stubDoctor struct{ report port.DoctorReport }

func (s stubDoctor) Run(context.Context, port.DoctorRequest) port.DoctorReport { return s.report }

type stubScan struct{ repos []workspace.RepoRef }

func (s stubScan) Scan(context.Context, port.ScanRequest) ([]workspace.RepoRef, error) {
	return s.repos, nil
}

type stubCreate struct {
	got port.CreateRequest
	err error
}

func (s *stubCreate) Create(_ context.Context, req port.CreateRequest) (port.CreateResult, error) {
	s.got = req
	return port.CreateResult{Name: req.Name, Path: "/ws/x", SandboxName: "boxai-x"}, s.err
}

type stubRemove struct{ got port.RemoveRequest }

func (s *stubRemove) Remove(_ context.Context, req port.RemoveRequest) error {
	s.got = req
	return nil
}

type stubLifecycle struct {
	attached, stopped string
	attachErr         error
}

func (s *stubLifecycle) Attach(_ context.Context, name string) error {
	s.attached = name
	return s.attachErr
}
func (s *stubLifecycle) Stop(_ context.Context, name string) error { s.stopped = name; return nil }
func (s *stubLifecycle) List(context.Context) ([]port.WorkspaceInfo, error) {
	return []port.WorkspaceInfo{{
		State:         port.WorkspaceState{Name: "shop", Agent: "claude", Path: "/ws/shop"},
		SandboxStatus: "running", Repos: []string{"api", "ui"},
	}}, nil
}

func run(t *testing.T, svc Services, args ...string) (int, string, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code := Execute(context.Background(), args, strings.NewReader(""), &out, &errOut, func(Runtime, port.Prompter, port.Notifier) Services {
		return svc
	})
	return code, out.String(), errOut.String()
}

func TestDoctorPrintsChecksAndSetsExitCode(t *testing.T) {
	report := port.DoctorReport{Checks: []port.DoctorCheck{
		{Name: "git", OK: true, Required: true, Detail: "2.54.0"},
		{Name: "sbx", OK: false, Required: true, Detail: "nicht gefunden"},
		{Name: "docker", OK: false, Detail: "nicht gefunden"},
	}}
	code, out, _ := run(t, Services{Doctor: stubDoctor{report}}, "doctor")
	if code != 1 || !strings.Contains(out, "✗ sbx") || !strings.Contains(out, "✓ git") || !strings.Contains(out, "○ docker") {
		t.Fatal(code, out)
	}
}

func TestScan(t *testing.T) {
	svc := Services{Scan: stubScan{[]workspace.RepoRef{{Name: "api", CurrentBranch: "main", Path: "/a", Dirty: true}, {Name: "ui", Path: "/u"}}}}
	code, out, _ := run(t, svc, "scan")
	if code != 0 || !strings.Contains(out, "geändert") || !strings.Contains(out, "sauber") {
		t.Fatal(code, out)
	}
}

func TestUpPassesFlags(t *testing.T) {
	create := &stubCreate{}
	svc := Services{Create: create, Defaults: Defaults{Root: "/r", CacheRoot: "/c", KitRoot: "/k"}}
	code, out, _ := run(t, svc, "up", "--name", "shop", "--repo", "api", "--repo", "https://x.y/a/b.git",
		"--agent", "copilot", "--profile", "react,kotlin-spring", "--base", "dev", "--secrets", "/s", "--no-docker",
		"--keep-on-error", "--dry-run", "--config", "/b.yaml")
	if code != 0 {
		t.Fatal(code)
	}
	got := create.got
	if got.Name != "shop" || len(got.Repos) != 2 || got.Agent != "copilot" || len(got.Profiles) != 2 || got.Base != "dev" ||
		got.Secrets != "/s" || !got.NoDocker || !got.KeepOnErr || !got.DryRun || got.Config != "/b.yaml" {
		t.Fatalf("%+v", got)
	}
	if got.DefaultRoot != "/r" || got.CacheRoot != "/c" || got.KitRoot != "/k" || got.Dir != "." || got.Depth != 3 {
		t.Fatalf("%+v", got)
	}
	if !strings.Contains(out, "boxai-x") {
		t.Fatal(out)
	}
}

func TestUpMutuallyExclusiveFlags(t *testing.T) {
	for _, args := range [][]string{{"up", "--image", "a", "--dockerfile", "b"}, {"up", "--branch", "a", "--checkout", "b"}} {
		if code, _, errOut := run(t, Services{Create: &stubCreate{}}, args...); code != 1 || errOut == "" {
			t.Errorf("%v: %d", args, code)
		}
	}
}

func TestExitCodes(t *testing.T) {
	create := &stubCreate{err: &port.AgentExit{Code: 42}}
	code, _, errOut := run(t, Services{Create: create}, "up")
	if code != 42 || errOut != "" {
		t.Fatal(code, errOut)
	}
	create.err = errors.New("kaputt")
	code, _, errOut = run(t, Services{Create: create}, "up")
	if code != 1 || !strings.Contains(errOut, "kaputt") {
		t.Fatal(code, errOut)
	}
}

func TestLifecycleCommands(t *testing.T) {
	life := &stubLifecycle{}
	remove := &stubRemove{}
	svc := Services{Lifecycle: life, Remove: remove}
	if code, out, _ := run(t, svc, "list"); code != 0 || !strings.Contains(out, "api,ui") || !strings.Contains(out, "running") {
		t.Fatal(code, out)
	}
	if code, _, _ := run(t, svc, "attach", "shop"); code != 0 || life.attached != "shop" {
		t.Fatal(code, life)
	}
	if code, _, _ := run(t, svc, "stop", "shop"); code != 0 || life.stopped != "shop" {
		t.Fatal(code, life)
	}
	if code, _, _ := run(t, svc, "rm", "shop", "--delete-branches", "--force"); code != 0 || remove.got.Name != "shop" || !remove.got.DeleteBranches || !remove.got.Force {
		t.Fatal(code, remove)
	}
	life.attachErr = &port.AgentExit{Code: 3}
	if code, _, _ := run(t, svc, "attach", "shop"); code != 3 {
		t.Fatal(code)
	}
	if code, _, _ := run(t, svc, "attach"); code != 1 {
		t.Fatal("fehlendes Argument akzeptiert")
	}
}

func TestInitWritesExamplesWithoutOverwriting(t *testing.T) {
	dir := t.TempDir()
	if code, out, _ := run(t, Services{}, "init", "--dir", dir); code != 0 || !strings.Contains(out, "boxai.yaml geschrieben") {
		t.Fatal(code, out)
	}
	path := filepath.Join(dir, "boxai.yaml")
	if err := os.WriteFile(path, []byte("eigen"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, out, _ := run(t, Services{}, "init", "--dir", dir); code != 0 || !strings.Contains(out, "bleibt unverändert") {
		t.Fatal(code, out)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "eigen" {
		t.Fatal("Datei überschrieben")
	}
	secrets, _ := os.ReadFile(filepath.Join(dir, "secrets.example.yaml"))
	if !strings.Contains(string(secrets), "${env:NEXUS_TOKEN}") {
		t.Fatal(string(secrets))
	}
	if code, _, _ := run(t, Services{}, "init", "--dir", filepath.Join(dir, "fehlt")); code != 1 {
		t.Fatal("fehlendes Ziel akzeptiert")
	}
}

func TestPrompterWithoutTerminalExplainsFlags(t *testing.T) {
	p := &Prompter{Stdin: strings.NewReader("")}
	ctx := context.Background()
	if _, err := p.SelectRepos(ctx, nil); !errors.Is(err, ErrNotInteractive) {
		t.Fatal(err)
	}
	if _, err := p.SelectAgent(ctx, nil); !errors.Is(err, ErrNotInteractive) {
		t.Fatal(err)
	}
	if _, err := p.InputName(ctx); !errors.Is(err, ErrNotInteractive) {
		t.Fatal(err)
	}
}

func TestRepoLabel(t *testing.T) {
	label := RepoLabel(workspace.RepoRef{Name: "api", CurrentBranch: "main", Path: "/a", Dirty: true})
	for _, want := range []string{"api", "main", "/a", "geändert"} {
		if !strings.Contains(label, want) {
			t.Errorf("%q fehlt in %q", want, label)
		}
	}
}

func TestNotifier(t *testing.T) {
	var buf bytes.Buffer
	n := &Notifier{Out: &buf}
	n.Info("a")
	n.Warn("b")
	if buf.String() != "a\nWarnung: b\n" {
		t.Fatal(buf.String())
	}
}
