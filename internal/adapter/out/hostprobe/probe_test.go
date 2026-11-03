package hostprobe

import (
	"context"
	"errors"
	"testing"

	"github.com/feierabendgewitter/boxai/internal/adapter/out/execx/execxtest"
)

func found(string) (string, error)   { return "/bin/x", nil }
func missing(string) (string, error) { return "", errors.New("nope") }

func TestToolParsesVersion(t *testing.T) {
	r := &execxtest.Recorder{Responses: map[string]execxtest.Response{
		"sbx version":   {Stdout: "sbx version v0.51.0 7d55286\n"},
		"git --version": {Stdout: "git version 2.54.0 (Apple Git-157)\n"},
	}}
	p := New(r, found)
	if got := p.Tool(context.Background(), "sbx"); !got.Found || got.Version != "0.51.0" {
		t.Fatalf("%+v", got)
	}
	if got := p.Tool(context.Background(), "git"); got.Version != "2.54.0" {
		t.Fatalf("%+v", got)
	}
}

func TestToolMissing(t *testing.T) {
	if got := New(&execxtest.Recorder{}, missing).Tool(context.Background(), "sbx"); got.Found {
		t.Fatal("Tool müsste fehlen")
	}
}

func TestToolHangingIsNotFound(t *testing.T) {
	r := &execxtest.Recorder{Responses: map[string]execxtest.Response{"sbx version": {Err: context.DeadlineExceeded}}}
	got := New(r, found).Tool(context.Background(), "sbx")
	if got.Found || got.Detail == "" {
		t.Fatalf("%+v", got)
	}
}

func TestAuthFromDiagnose(t *testing.T) {
	r := &execxtest.Recorder{Responses: map[string]execxtest.Response{"sbx diagnose": {Stdout: `{"checks":[{"name":"Authentication","status":"pass","message":"authenticated"}]}`}}}
	if ok, msg := New(r, found).SbxAuthenticated(context.Background()); !ok || msg != "authenticated" {
		t.Fatal(ok, msg)
	}
	r.Responses["sbx diagnose"] = execxtest.Response{Stdout: `{"sections":[{"checks":[{"name":"Authentication","status":"fail","message":"nope"}]}]}`, Err: errors.New("exit 1")}
	if ok, msg := New(r, found).SbxAuthenticated(context.Background()); ok || msg != "nope" {
		t.Fatal(ok, msg)
	}
	r.Responses["sbx diagnose"] = execxtest.Response{Stdout: "kaputt"}
	if ok, _ := New(r, found).SbxAuthenticated(context.Background()); ok {
		t.Fatal("kaputtes JSON akzeptiert")
	}
}
