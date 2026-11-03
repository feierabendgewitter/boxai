package workspace

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func selection(name, def string) RepoSelection {
	return RepoSelection{Ref: RepoRef{Name: name, Path: "/src/" + name, DefaultBranch: def}}
}

func TestPlanNewBranch(t *testing.T) {
	ws, err := Plan(PlanInput{
		Name: "shop", Root: "/ws",
		Repos: []RepoSelection{selection("api", "main"), {Ref: RepoRef{Name: "ui", Path: "/src/ui", DefaultBranch: "main"}, Base: "develop"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []Worktree{
		{RepoName: "api", RepoPath: "/src/api", Path: "/ws/shop/api", Branch: "boxai/shop", Base: "main", CreateBranch: true},
		{RepoName: "ui", RepoPath: "/src/ui", Path: "/ws/shop/ui", Branch: "boxai/shop", Base: "develop", CreateBranch: true},
	}
	if !reflect.DeepEqual(ws.Worktrees, want) {
		t.Fatalf("%+v", ws.Worktrees)
	}
	if ws.Path != "/ws/shop" || ws.SandboxName() != "boxai-shop" {
		t.Fatalf("%+v", ws)
	}
}

func TestPlanBasePrecedence(t *testing.T) {
	ws, err := Plan(PlanInput{
		Name: "shop", Root: "/ws", Strategy: BranchStrategy{Base: "release", Branch: "feature/x"},
		Repos: []RepoSelection{selection("api", "main")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := ws.Worktrees[0]; got.Base != "release" || got.Branch != "feature/x" {
		t.Fatalf("%+v", got)
	}
}

func TestPlanCheckout(t *testing.T) {
	ws, err := Plan(PlanInput{
		Name: "shop", Root: "/ws", Strategy: BranchStrategy{Checkout: "develop"},
		Repos: []RepoSelection{selection("api", "")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := ws.Worktrees[0]; got.CreateBranch || got.Branch != "develop" || got.Base != "" {
		t.Fatalf("%+v", got)
	}
}

func TestPlanErrors(t *testing.T) {
	repos := []RepoSelection{selection("api", "main")}
	tests := []struct {
		name string
		in   PlanInput
		want string
	}{
		{"Name", PlanInput{Name: "x", Root: "/ws", Repos: repos}, "ungültiger"},
		{"Root", PlanInput{Name: "shop", Repos: repos}, "root"},
		{"ohne Repos", PlanInput{Name: "shop", Root: "/ws"}, "mindestens ein Repo"},
		{"Flags", PlanInput{Name: "shop", Root: "/ws", Repos: repos, Strategy: BranchStrategy{Branch: "a", Checkout: "b"}}, "schließen"},
		{"Duplikat", PlanInput{Name: "shop", Root: "/ws", Repos: []RepoSelection{selection("api", "m"), selection("api", "m")}}, "zwei"},
		{"ohne Base", PlanInput{Name: "shop", Root: "/ws", Repos: []RepoSelection{selection("api", "")}}, "Base"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Plan(tt.in)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Fehler = %v", err)
			}
		})
	}
	if _, err := Plan(PlanInput{Name: "shop", Root: "/ws"}); !errors.Is(err, ErrNoRepos) {
		t.Fatal(err)
	}
}

func TestValidateName(t *testing.T) {
	for _, ok := range []string{"ab", "shop-feature-x", "v1.2"} {
		if err := ValidateName(ok); err != nil {
			t.Errorf("%s: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "a", "-ab", "a b", "a/b", "../x"} {
		if err := ValidateName(bad); err == nil {
			t.Errorf("%q müsste ungültig sein", bad)
		}
	}
}

func TestParseRemote(t *testing.T) {
	tests := []struct {
		raw  string
		want Remote
	}{
		{"https://github.com/acme/api.git", Remote{"github.com", "acme", "api"}},
		{"https://github.com/acme/api/", Remote{"github.com", "acme", "api"}},
		{"git@gitlab.com:team/shared-lib.git", Remote{"gitlab.com", "team", "shared-lib"}},
		{"git@gitlab.com:team/sub/lib.git", Remote{"gitlab.com", "team/sub", "lib"}},
		{"ssh://git@git.firma.de:2222/team/lib.git", Remote{"git.firma.de", "team", "lib"}},
		{"https://token@github.com/acme/api", Remote{"github.com", "acme", "api"}},
	}
	for _, tt := range tests {
		got, err := ParseRemote(tt.raw)
		if err != nil || got != tt.want {
			t.Errorf("%s: %+v, %v", tt.raw, got, err)
		}
	}
	for _, bad := range []string{"", "shop-backend", "../shop-backend", "/abs/path", "https://github.com", "https://github.com/onlyowner", "ftp://h/o/r", "git@h:o/../r", "h:/r/"} {
		if IsRemote(bad) {
			t.Errorf("%q müsste ungültig sein", bad)
		}
	}
}

func TestCachePath(t *testing.T) {
	r := Remote{"gitlab.com", "team/sub", "lib"}
	if got := r.CachePath("/home/j/.boxai/repos"); got != "/home/j/.boxai/repos/gitlab.com/team/sub/lib" {
		t.Fatal(got)
	}
}
