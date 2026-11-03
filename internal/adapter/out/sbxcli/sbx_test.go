package sbxcli

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/feierabendgewitter/boxai/internal/adapter/out/execx"
	"github.com/feierabendgewitter/boxai/internal/adapter/out/execx/execxtest"
	"github.com/feierabendgewitter/boxai/internal/app/port"
	"github.com/feierabendgewitter/boxai/internal/domain/profile"
	"github.com/feierabendgewitter/boxai/internal/domain/sandbox"
	"github.com/feierabendgewitter/boxai/internal/domain/secret"
)

func assertLines(t *testing.T, rec *execxtest.Recorder, want ...string) {
	t.Helper()
	if got := rec.Lines(); !slices.Equal(got, want) {
		t.Fatalf("\ngot:  %q\nwant: %q", got, want)
	}
}

func TestCreateCommandLine(t *testing.T) {
	rec := &execxtest.Recorder{}
	err := New(rec).Create(context.Background(), sandbox.SandboxSpec{
		Name: "boxai-shop", Agent: "claude", Workspace: "/ws/shop",
		Mounts: []sandbox.Mount{{Path: "/src/api/.git"}, {Path: "/docs", ReadOnly: true}},
		KitDir: "/kits/shop", Template: "boxai-react-claude:latest", LocalTemplate: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	assertLines(t, rec, "sbx create --no-input --no-progress --name boxai-shop --skills off --kit /kits/shop --template boxai-react-claude:latest --pull never claude /ws/shop /src/api/.git /docs:ro")
}

func TestCreateMinimal(t *testing.T) {
	rec := &execxtest.Recorder{}
	if err := New(rec).Create(context.Background(), sandbox.SandboxSpec{Name: "boxai-x", Agent: "copilot", Workspace: "/ws/x"}); err != nil {
		t.Fatal(err)
	}
	assertLines(t, rec, "sbx create --no-input --no-progress --name boxai-x --skills off copilot /ws/x")
}

func TestEnvSecretsStayOutOfArguments(t *testing.T) {
	rec := &execxtest.Recorder{}
	err := New(rec).Create(context.Background(), sandbox.SandboxSpec{
		Name: "boxai-x", Agent: "claude", Workspace: "/ws/x",
		EnvSecrets: []secret.SecretSpec{{Name: "AWS_KEY", Value: "geheim"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	cmd := rec.Cmds[0]
	if strings.Contains(strings.Join(cmd.Args, " "), "geheim") {
		t.Fatal("Wert steht im Argument")
	}
	if !slices.Contains(cmd.Args, "AWS_KEY") || cmd.Env[0] != "AWS_KEY=geheim" || cmd.Secrets[0] != "geheim" {
		t.Fatalf("%+v", cmd)
	}
	if strings.Contains(rec.Lines()[0], "geheim") {
		t.Fatal(rec.Lines()[0])
	}
}

func TestAttachIsInteractiveAndMapsExitCode(t *testing.T) {
	rec := &execxtest.Recorder{Responses: map[string]execxtest.Response{
		"sbx run": {Err: &execx.ExitError{Code: 7}},
	}}
	err := New(rec).Attach(context.Background(), "boxai-x")
	var exit *port.AgentExit
	if !errors.As(err, &exit) || exit.Code != 7 {
		t.Fatal(err)
	}
	if !rec.Cmds[0].Interactive {
		t.Fatal("nicht interaktiv")
	}
	assertLines(t, rec, "sbx run --name boxai-x")

	boom := errors.New("boom")
	rec.Responses["sbx run"] = execxtest.Response{Err: boom}
	if err := New(rec).Attach(context.Background(), "boxai-x"); !errors.Is(err, boom) {
		t.Fatal(err)
	}
}

func TestStopAndRemove(t *testing.T) {
	rec := &execxtest.Recorder{}
	c := New(rec)
	ctx := context.Background()
	if err := c.Stop(ctx, "boxai-x"); err != nil {
		t.Fatal(err)
	}
	if err := c.Remove(ctx, "boxai-x"); err != nil {
		t.Fatal(err)
	}
	assertLines(t, rec, "sbx stop boxai-x", "sbx rm --force boxai-x", "sbx template ls")
}

func TestRemoveDeletesOnlyOwnKitImages(t *testing.T) {
	rec := &execxtest.Recorder{Responses: map[string]execxtest.Response{"sbx template ls": {Stdout: `REPOSITORY  TAG  IMAGE ID  FLAVOR  CREATED
docker.io/library/sbx-kit-src  boxai-foo-0eaab94f583d  7253033da215    2m
docker.io/library/sbx-kit-src  boxai-foo-bar-6be4d2d3bd00  9a8354d6abd8    4m
docker.io/library/sbx-kit-src  fremd-e6520cf8e5df  08497a57dac6    5m
docker.io/library/boxai-foo  0eaab94f583d  017481601084  claude  1m
docker.io/library/boxai-custom-foo  latest  017481601085  claude  1m
docker.io/library/boxai-custom-foo-bar  latest  017481601086  claude  1m
`}}}
	if err := New(rec).Remove(context.Background(), "boxai-foo"); err != nil {
		t.Fatal(err)
	}
	assertLines(t, rec,
		"sbx rm --force boxai-foo",
		"sbx template ls",
		"sbx template rm --force docker.io/library/sbx-kit-src:boxai-foo-0eaab94f583d",
		"sbx template rm --force docker.io/library/boxai-custom-foo:latest",
	)
	boom := errors.New("boom")
	rec.Responses["sbx template rm"] = execxtest.Response{Err: boom}
	if err := New(rec).Remove(context.Background(), "boxai-foo"); !errors.Is(err, boom) {
		t.Fatal(err)
	}
	rec.Responses["sbx template ls"] = execxtest.Response{Err: boom}
	if err := New(rec).Remove(context.Background(), "boxai-foo"); !errors.Is(err, boom) {
		t.Fatal(err)
	}
}

func TestRemoveMissingSandboxIsFine(t *testing.T) {
	rec := &execxtest.Recorder{Responses: map[string]execxtest.Response{
		"sbx rm": {Err: &execx.ExitError{Code: 1, Stderr: "error: sandbox 'x' not found"}},
	}}
	if err := New(rec).Remove(context.Background(), "x"); err != nil {
		t.Fatal(err)
	}
	rec.Responses["sbx rm"] = execxtest.Response{Err: &execx.ExitError{Code: 1, Stderr: "daemon down"}}
	if err := New(rec).Remove(context.Background(), "x"); err == nil {
		t.Fatal("anderer Fehler verschluckt")
	}
}

func TestListParsesJSON(t *testing.T) {
	rec := &execxtest.Recorder{Responses: map[string]execxtest.Response{
		"sbx ls": {Stdout: `[{"name":"boxai-a","agent":"claude","status":"running","mounts":["/x:/x"]},{"name":"copilot-b","agent":"copilot","status":"stopped"}]`},
	}}
	infos, err := New(rec).List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(infos) != 2 || infos[0] != (sandbox.Info{Name: "boxai-a", Agent: "claude", Status: "running"}) {
		t.Fatalf("%+v", infos)
	}
	if !rec.Cmds[0].ReadOnly {
		t.Fatal("sbx ls muss auch im Dry-Run laufen")
	}
	rec.Responses["sbx ls"] = execxtest.Response{Stdout: "kaputt"}
	if _, err := New(rec).List(context.Background()); err == nil {
		t.Fatal("kaputtes JSON akzeptiert")
	}
	boom := errors.New("boom")
	rec.Responses["sbx ls"] = execxtest.Response{Err: boom}
	if _, err := New(rec).List(context.Background()); !errors.Is(err, boom) {
		t.Fatal(err)
	}
}

func TestAllowNetworkIsScopedToSandbox(t *testing.T) {
	rec := &execxtest.Recorder{}
	c := New(rec)
	if err := c.AllowNetwork(context.Background(), "boxai-x", nil); err != nil || len(rec.Cmds) != 0 {
		t.Fatal(err, rec.Cmds)
	}
	err := c.AllowNetwork(context.Background(), "boxai-x", []profile.NetworkRule{{Host: "github.com"}, {Host: "**.amazonaws.com"}})
	if err != nil {
		t.Fatal(err)
	}
	assertLines(t, rec, "sbx policy allow network --sandbox boxai-x 'github.com,**.amazonaws.com'")
}

func TestPutSendsValueOnlyViaStdin(t *testing.T) {
	rec := &execxtest.Recorder{}
	err := New(rec).Put(context.Background(), "boxai-x", secret.SecretSpec{Name: "T", Hosts: []string{"a.example", "*.b.example"}, Value: "geheim"})
	if err != nil {
		t.Fatal(err)
	}
	cmd := rec.Cmds[0]
	if cmd.Stdin != "geheim" || strings.Contains(strings.Join(cmd.Args, " "), "geheim") || cmd.ReadOnly {
		t.Fatalf("%+v", cmd)
	}
	assertLines(t, rec, "sbx secret set-custom --sandbox boxai-x --env T --value-stdin --host a.example --host '*.b.example' < [stdin]")
}

func TestDeleteAllRemovesOnlyOwnScope(t *testing.T) {
	rec := &execxtest.Recorder{Responses: map[string]execxtest.Response{
		"sbx secret ls": {Stdout: `{"secrets":[],"custom_secrets":[
			{"scope":"boxai-x","placeholder":"sbx-cs-1","secret":"fake-v****"},
			{"scope":"global","placeholder":"sbx-cs-2"}
		]}`},
	}}
	if err := New(rec).DeleteAll(context.Background(), "boxai-x"); err != nil {
		t.Fatal(err)
	}
	assertLines(t, rec,
		"sbx secret ls --sandbox boxai-x --json",
		"sbx secret rm --placeholder sbx-cs-1 --sandbox boxai-x --force",
	)
}

func TestDeleteAllErrors(t *testing.T) {
	rec := &execxtest.Recorder{Responses: map[string]execxtest.Response{"sbx secret ls": {Stdout: "fake-v**** kaputt"}}}
	err := New(rec).DeleteAll(context.Background(), "x")
	if err == nil || strings.Contains(err.Error(), "fake-v") {
		t.Fatal(err)
	}
	rec.Responses["sbx secret ls"] = execxtest.Response{Err: &execx.ExitError{Code: 1, Stderr: "sandbox not found"}}
	if err := New(rec).DeleteAll(context.Background(), "x"); err != nil {
		t.Fatal(err)
	}
	rec.Responses["sbx secret ls"] = execxtest.Response{Stdout: `{"custom_secrets":[{"scope":"x","placeholder":"p"}]}`}
	rec.Responses["sbx secret rm"] = execxtest.Response{Err: errors.New("boom")}
	if err := New(rec).DeleteAll(context.Background(), "x"); err == nil {
		t.Fatal("Fehler verschluckt")
	}
}
