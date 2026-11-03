package dockerbuild

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/feierabendgewitter/boxai/internal/adapter/out/execx/execxtest"
)

const templateList = `REPOSITORY                           TAG              IMAGE ID       FLAVOR           CREATED
docker.io/docker/sandbox-templates   copilot-docker   83feb0d8dad0   copilot-docker   3mo
docker.io/library/boxai-base-claude  latest           9bed33b208b6   claude-code      2mo
ghcr.io/acme/box                     1.2              9bed33b208b7   claude           2mo
`

func TestBuildAndLoad(t *testing.T) {
	rec := &execxtest.Recorder{}
	ref, err := New(rec).BuildAndLoad(context.Background(), "/proj/images/Dockerfile", "boxai-custom-x:latest")
	if err != nil || ref != "boxai-custom-x:latest" {
		t.Fatal(ref, err)
	}
	lines := rec.Lines()
	if len(lines) != 3 {
		t.Fatal(lines)
	}
	if lines[0] != "docker build -f /proj/images/Dockerfile -t boxai-custom-x:latest /proj/images" {
		t.Fatal(lines[0])
	}
	if !strings.HasPrefix(lines[1], "docker image save boxai-custom-x:latest -o ") || !strings.HasPrefix(lines[2], "sbx template load ") {
		t.Fatal(lines)
	}
}

func TestBuildStopsAtFirstError(t *testing.T) {
	boom := errors.New("boom")
	for _, prefix := range []string{"docker build", "docker image save", "sbx template load"} {
		rec := &execxtest.Recorder{Responses: map[string]execxtest.Response{prefix: {Err: boom}}}
		if _, err := New(rec).BuildAndLoad(context.Background(), "/d/Dockerfile", "t"); !errors.Is(err, boom) {
			t.Errorf("%s: %v", prefix, err)
		}
	}
}

func TestAvailable(t *testing.T) {
	rec := &execxtest.Recorder{Responses: map[string]execxtest.Response{"sbx template ls": {Stdout: templateList}}}
	b := New(rec)
	tests := map[string]bool{
		"boxai-base-claude:latest":                true,
		"boxai-base-claude":                       true,
		"docker/sandbox-templates:copilot-docker": true,
		"ghcr.io/acme/box:1.2":                    true,
		"ghcr.io/acme/box:1.3":                    false,
		"boxai-base-copilot:latest":               false,
	}
	for ref, want := range tests {
		got, err := b.Available(context.Background(), ref)
		if err != nil || got != want {
			t.Errorf("%s: %v %v", ref, got, err)
		}
	}
	if !rec.Cmds[0].ReadOnly {
		t.Fatal("muss im Dry-Run laufen")
	}
	rec.Responses["sbx template ls"] = execxtest.Response{Err: errors.New("boom")}
	if _, err := b.Available(context.Background(), "x"); err == nil {
		t.Fatal("Fehler verschluckt")
	}
}

func TestCanonicalRegistryWithPort(t *testing.T) {
	if got := canonical("reg:5000/a/b"); got != "reg:5000/a/b:latest" {
		t.Fatal(got)
	}
	if got := canonical("localhost/a:1"); got != "localhost/a:1" {
		t.Fatal(got)
	}
}

func TestAgentLabel(t *testing.T) {
	rec := &execxtest.Recorder{Responses: map[string]execxtest.Response{"docker image inspect": {Stdout: "claude\n"}}}
	b := New(rec)
	if got, err := b.AgentLabel(context.Background(), "x"); err != nil || got != "claude" {
		t.Fatal(got, err)
	}
	rec.Responses["docker image inspect"] = execxtest.Response{Stdout: "<no value>\n"}
	if got, err := b.AgentLabel(context.Background(), "x"); err != nil || got != "" {
		t.Fatal(got, err)
	}
	rec.Responses["docker image inspect"] = execxtest.Response{Err: errors.New("boom")}
	if _, err := b.AgentLabel(context.Background(), "x"); err == nil {
		t.Fatal("Fehler verschluckt")
	}
}
