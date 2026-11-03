package scanrepos

import (
	"context"
	"errors"
	"testing"

	"github.com/feierabendgewitter/boxai/internal/app/port"
	"github.com/feierabendgewitter/boxai/internal/app/port/porttest"
	"github.com/feierabendgewitter/boxai/internal/domain/workspace"
)

func TestScanEnrichesRepos(t *testing.T) {
	git := porttest.NewGit()
	git.Dirty["/src/api"] = true
	scanner := &porttest.Scanner{Repos: []workspace.RepoRef{{Name: "api", Path: "/src/api"}}}
	repos, err := New(scanner, git).Scan(context.Background(), port.ScanRequest{Dir: "/src"})
	if err != nil {
		t.Fatal(err)
	}
	r := repos[0]
	if !r.Dirty || r.CurrentBranch != "main" || r.DefaultBranch != "main" {
		t.Fatalf("%+v", r)
	}
	if scanner.LastDepth != DefaultDepth {
		t.Fatalf("Tiefe = %d", scanner.LastDepth)
	}
}

func TestScanPropagatesErrors(t *testing.T) {
	boom := errors.New("boom")
	_, err := New(&porttest.Scanner{Err: boom}, porttest.NewGit()).Scan(context.Background(), port.ScanRequest{Depth: 1})
	if !errors.Is(err, boom) {
		t.Fatal(err)
	}
	for _, method := range []string{"CurrentBranch", "DefaultBranch", "IsDirty"} {
		git := porttest.NewGit()
		git.FailOn[method] = boom
		scanner := &porttest.Scanner{Repos: []workspace.RepoRef{{Name: "api", Path: "/src/api"}}}
		if _, err := New(scanner, git).Scan(context.Background(), port.ScanRequest{}); !errors.Is(err, boom) {
			t.Errorf("%s: %v", method, err)
		}
	}
}
