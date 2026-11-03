package createworkspace

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/feierabendgewitter/boxai/internal/app/port"
	"github.com/feierabendgewitter/boxai/internal/app/port/porttest"
	"github.com/feierabendgewitter/boxai/internal/domain/agent"
	"github.com/feierabendgewitter/boxai/internal/domain/secret"
	"github.com/feierabendgewitter/boxai/internal/domain/workspace"
)

type healthyDoctor struct{ report port.DoctorReport }

func (h healthyDoctor) Run(context.Context, port.DoctorRequest) port.DoctorReport { return h.report }

type env struct {
	journal  *porttest.Journal
	git      *porttest.Git
	runtime  *porttest.Runtime
	secrets  *porttest.Secrets
	kits     *porttest.Kits
	assets   *porttest.Assets
	images   *porttest.Images
	config   *porttest.Config
	state    *porttest.State
	prompter *porttest.Prompter
	notifier *porttest.Notifier
	scanner  *porttest.Scanner
	doctor   *healthyDoctor
	svc      *Service
}

func newEnv() *env {
	j := &porttest.Journal{}
	e := &env{
		journal: j, git: porttest.NewGit(), runtime: porttest.NewRuntime(), secrets: porttest.NewSecrets(),
		kits: porttest.NewKits(), assets: porttest.NewAssets(), images: porttest.NewImages(), config: porttest.NewConfig(),
		state: porttest.NewState(), prompter: &porttest.Prompter{}, notifier: &porttest.Notifier{},
		scanner: &porttest.Scanner{Repos: []workspace.RepoRef{
			{Name: "api", Path: "/src/api"}, {Name: "ui", Path: "/src/ui"},
		}},
		doctor: &healthyDoctor{},
	}
	e.git.Journal, e.runtime.Journal, e.secrets.Journal, e.kits.Journal = j, j, j, j
	e.images.Journal, e.state.Journal = j, j
	e.svc = New(Deps{
		Doctor: e.doctor, Scanner: e.scanner, Git: e.git, Runtime: e.runtime, Secrets: e.secrets,
		Kits: e.kits, Assets: e.assets, Images: e.images, Config: e.config, State: e.state,
		Prompter: e.prompter, Notifier: e.notifier, Registry: agent.NewRegistry(),
		Now: func() time.Time { return time.Unix(0, 0) },
	})
	return e
}

func request() port.CreateRequest {
	return port.CreateRequest{
		Dir: "/src", Name: "shop", Repos: []string{"api", "ui"}, Agent: "claude", DefaultRoot: "/ws",
		CacheRoot: "/cache", KitRoot: "/kits", NoAttach: true,
	}
}

func (e *env) index(t *testing.T, prefix string) int {
	t.Helper()
	i := slices.IndexFunc(e.journal.Entries, func(s string) bool { return strings.HasPrefix(s, prefix) })
	if i < 0 {
		t.Fatalf("%q fehlt in %v", prefix, e.journal.Entries)
	}
	return i
}

func TestCreateHappyPathOrder(t *testing.T) {
	e := newEnv()
	e.config.SecretSets["/s.yaml"] = []secret.SecretSpec{{Name: "TOKEN", Hosts: []string{"nexus.firma.de"}, Value: "geheim"}}
	req := request()
	req.Secrets = "/s.yaml"
	req.NoAttach = false
	res, err := e.svc.Create(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if res.SandboxName != "boxai-shop" || res.Path != "/ws/shop" || res.Resumed {
		t.Fatalf("%+v", res)
	}
	order := []string{"worktree add api", "worktree add ui", "kit write", "kit validate", "secret put TOKEN", "sandbox create", "sandbox network", "sandbox attach"}
	last := -1
	for _, step := range order {
		i := e.index(t, step)
		if i <= last {
			t.Fatalf("%s steht falsch: %v", step, e.journal.Entries)
		}
		last = i
	}
	spec := e.runtime.Specs[0]
	if spec.Workspace != "/ws/shop" || spec.Agent != "claude" || len(spec.Mounts) != 2 || spec.Mounts[0].Path != "/src/api/.git" {
		t.Fatalf("%+v", spec)
	}
	if e.state.States["shop"].Status != port.StatusReady {
		t.Fatalf("%+v", e.state.States["shop"])
	}
	if len(e.runtime.Network["boxai-shop"]) == 0 {
		t.Fatal("keine Netzwerkregeln")
	}
}

func TestSecretHostsAreAllowed(t *testing.T) {
	e := newEnv()
	e.config.SecretSets["/s.yaml"] = []secret.SecretSpec{{Name: "TOKEN", Hosts: []string{"nexus.firma.de"}, Value: "geheim"}}
	req := request()
	req.Secrets = "/s.yaml"
	if _, err := e.svc.Create(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, rule := range e.runtime.Network["boxai-shop"] {
		found = found || rule.Host == "nexus.firma.de"
	}
	if !found {
		t.Fatalf("%+v", e.runtime.Network)
	}
}

// Akzeptanzkriterium Phase 4: Ein Fehler in Repo 2 entfernt den Worktree von Repo 1.
func TestFailureInSecondRepoRollsBackFirst(t *testing.T) {
	e := newEnv()
	e.git.FailAddFor = "ui"
	_, err := e.svc.Create(context.Background(), request())
	if err == nil || !strings.Contains(err.Error(), "ui") {
		t.Fatal(err)
	}
	if len(e.git.Worktree) != 0 {
		t.Fatalf("Worktree übrig: %+v", e.git.Worktree)
	}
	if e.git.Branches["/src/api@boxai/shop"] {
		t.Fatal("neu angelegter Branch übrig")
	}
	if _, ok := e.state.States["shop"]; ok {
		t.Fatal("State übrig")
	}
	remove, del := e.index(t, "worktree remove api"), e.index(t, "branch delete /src/api")
	if remove > del {
		t.Fatalf("Branch vor Worktree gelöscht: %v", e.journal.Entries)
	}
}

func TestLateFailureRollsBackEverythingInReverse(t *testing.T) {
	e := newEnv()
	e.config.SecretSets["/s.yaml"] = []secret.SecretSpec{{Name: "TOKEN", Hosts: []string{"h.example"}, Value: "v"}}
	e.runtime.FailOn["AllowNetwork"] = errors.New("netz kaputt")
	req := request()
	req.Secrets = "/s.yaml"
	if _, err := e.svc.Create(context.Background(), req); err == nil {
		t.Fatal("Fehler erwartet")
	}
	sandboxGone, secretsGone, kitGone, wtGone := e.index(t, "sandbox remove"), e.index(t, "secret delete-all"), e.index(t, "kit remove"), e.index(t, "worktree remove ui")
	if sandboxGone > secretsGone || secretsGone > kitGone || kitGone > wtGone {
		t.Fatalf("Reihenfolge: %v", e.journal.Entries)
	}
	if len(e.runtime.Sandboxes) != 0 || len(e.kits.Written) != 0 || len(e.git.Worktree) != 0 {
		t.Fatal("Reste übrig")
	}
}

func TestRollbackFailureIsReportedAndContinues(t *testing.T) {
	e := newEnv()
	e.runtime.FailOn["Create"] = errors.New("create kaputt")
	e.runtime.FailOn["Remove"] = errors.New("remove kaputt")
	_, err := e.svc.Create(context.Background(), request())
	if err == nil || !strings.Contains(err.Error(), "create kaputt") {
		t.Fatal(err)
	}
	if len(e.git.Worktree) != 0 {
		t.Fatal("Rollback hat aufgehört")
	}
	if !slices.ContainsFunc(e.notifier.Warns, func(s string) bool { return strings.Contains(s, "remove kaputt") }) {
		t.Fatalf("%v", e.notifier.Warns)
	}
}

func TestKeepOnError(t *testing.T) {
	e := newEnv()
	e.git.FailAddFor = "ui"
	req := request()
	req.KeepOnErr = true
	if _, err := e.svc.Create(context.Background(), req); err == nil {
		t.Fatal("Fehler erwartet")
	}
	if len(e.git.Worktree) != 1 {
		t.Fatalf("%+v", e.git.Worktree)
	}
	if st := e.state.States["shop"]; st.Status != port.StatusProvisioning || len(st.Worktrees) != 1 {
		t.Fatalf("%+v", st)
	}

	// Der nächste Lauf holt genau das fehlgeschlagene Repo nach.
	e.git.FailAddFor = ""
	res, err := e.svc.Create(context.Background(), req)
	if err != nil || !res.Resumed {
		t.Fatal(res, err)
	}
	if len(e.git.Worktree) != 2 || len(e.state.States["shop"].Worktrees) != 2 {
		t.Fatalf("%+v", e.git.Worktree)
	}
}

// Akzeptanzkriterium Phase 4: Der zweite Lauf ist idempotent.
func TestSecondRunIsIdempotent(t *testing.T) {
	e := newEnv()
	req := request()
	if _, err := e.svc.Create(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	before := len(e.journal.Entries)
	addedBefore := len(e.git.Worktree)
	req.NoAttach = false
	res, err := e.svc.Create(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Resumed {
		t.Fatal("kein Resume erkannt")
	}
	for _, entry := range e.journal.Entries[before:] {
		if strings.HasPrefix(entry, "worktree add") || strings.HasPrefix(entry, "sandbox create") || strings.HasPrefix(entry, "secret put") {
			t.Fatalf("doppelt ausgeführt: %s", entry)
		}
	}
	if len(e.git.Worktree) != addedBefore || len(e.runtime.Specs) != 1 {
		t.Fatal("dupliziert")
	}
	if len(e.runtime.Attached) != 1 {
		t.Fatal("zweiter Lauf startet den Agent nicht")
	}
}

func TestResumeAfterMissingSandboxCreatesIt(t *testing.T) {
	e := newEnv()
	req := request()
	if _, err := e.svc.Create(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	delete(e.runtime.Sandboxes, "boxai-shop")
	if _, err := e.svc.Create(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if len(e.runtime.Specs) != 2 {
		t.Fatalf("%+v", e.runtime.Specs)
	}
}

func TestResumeWithOtherAgentFails(t *testing.T) {
	e := newEnv()
	req := request()
	if _, err := e.svc.Create(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	req.Agent = "copilot"
	if _, err := e.svc.Create(context.Background(), req); err == nil || !strings.Contains(err.Error(), "boxai rm") {
		t.Fatal(err)
	}
}

// Akzeptanzkriterium Phase 4: nicht-interaktiver Lauf über boxai.yaml.
func TestNonInteractiveRunFromConfigFile(t *testing.T) {
	e := newEnv()
	e.git.Missing["/cache/gitlab.com/team/shared-lib"] = true
	e.config.Files["/p/boxai.yaml"] = port.WorkspaceConfig{
		Name: "shop", Root: "/custom", Agent: "copilot", Branch: "feature/x", Base: "main",
		Repos: []port.RepoConfig{
			{Path: "/src/api"},
			{Path: "/src/ui", Base: "develop"},
			{URL: "git@gitlab.com:team/shared-lib.git"},
		},
		NetworkAllow: []string{"nexus.firma.de:443"},
	}
	_, err := e.svc.Create(context.Background(), port.CreateRequest{Config: "/p/boxai.yaml", DefaultRoot: "/ws", CacheRoot: "/cache", KitRoot: "/kits", NoAttach: true})
	if err != nil {
		t.Fatal(err)
	}
	if e.prompter.Calls != 0 {
		t.Fatal("Prompt im nicht-interaktiven Lauf")
	}
	st := e.state.States["shop"]
	if st.Path != "/custom/shop" || st.Agent != agent.Copilot || len(st.Worktrees) != 3 {
		t.Fatalf("%+v", st)
	}
	if st.Worktrees[1].Base != "develop" || st.Worktrees[0].Base != "main" || st.Worktrees[0].Branch != "feature/x" {
		t.Fatalf("%+v", st.Worktrees)
	}
	if e.git.Clones["/cache/gitlab.com/team/shared-lib"] == "" {
		t.Fatalf("%+v", e.git.Clones)
	}
}

func TestFlagsOverrideConfigOverrideGlobal(t *testing.T) {
	e := newEnv()
	e.config.Global = port.WorkspaceConfig{Root: "/global", Agent: "copilot", Branch: "global", NetworkAllow: []string{"g.example"}}
	e.config.Files["/b.yaml"] = port.WorkspaceConfig{Name: "shop", Root: "/file", Branch: "file", Repos: []port.RepoConfig{{Path: "/src/api"}}, NetworkAllow: []string{"f.example"}}
	req := port.CreateRequest{Config: "/b.yaml", Branch: "flag", DefaultRoot: "/ws", KitRoot: "/kits", NoAttach: true}
	if _, err := e.svc.Create(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	st := e.state.States["shop"]
	if st.Path != "/file/shop" || st.Worktrees[0].Branch != "flag" || st.Agent != agent.Copilot {
		t.Fatalf("%+v", st)
	}
	var hosts []string
	for _, r := range e.runtime.Network["boxai-shop"] {
		hosts = append(hosts, r.Host)
	}
	if !slices.Contains(hosts, "g.example") || !slices.Contains(hosts, "f.example") {
		t.Fatal(hosts)
	}
}

func TestInteractiveSelection(t *testing.T) {
	e := newEnv()
	e.prompter.Repos = []string{"ui"}
	e.prompter.Agent = agent.Copilot
	e.prompter.Name = "picked"
	req := port.CreateRequest{Dir: "/src", DefaultRoot: "/ws", KitRoot: "/kits", NoAttach: true}
	if _, err := e.svc.Create(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if e.prompter.OfferedN != 2 {
		t.Fatal(e.prompter.OfferedN)
	}
	st := e.state.States["picked"]
	if len(st.Worktrees) != 1 || st.Worktrees[0].RepoName != "ui" || st.Agent != agent.Copilot {
		t.Fatalf("%+v", st)
	}
}

func TestDirtyRepoWarns(t *testing.T) {
	e := newEnv()
	e.git.Dirty["/src/api"] = true
	if _, err := e.svc.Create(context.Background(), request()); err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(e.notifier.Warns, func(s string) bool { return strings.Contains(s, "api") && strings.Contains(s, "uncommittete") }) {
		t.Fatalf("%v", e.notifier.Warns)
	}
}

func TestRemoteCloneVersusFetch(t *testing.T) {
	e := newEnv()
	e.git.Missing["/cache/github.com/acme/lib"] = true
	req := request()
	req.Repos = []string{"https://github.com/acme/lib.git"}
	if _, err := e.svc.Create(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if e.git.Clones["/cache/github.com/acme/lib"] == "" {
		t.Fatalf("%+v", e.git.Clones)
	}

	e = newEnv()
	req.Name = "other"
	if _, err := e.svc.Create(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if len(e.git.Clones) != 0 || !slices.Contains(e.git.Fetched, "/cache/github.com/acme/lib") {
		t.Fatalf("%+v %v", e.git.Clones, e.git.Fetched)
	}
}

func TestPreparationErrorsMutateNothing(t *testing.T) {
	failing := port.DoctorReport{Checks: []port.DoctorCheck{{Name: "sbx", Required: true, Detail: "fehlt"}}}
	tests := []struct {
		name string
		edit func(*env, *port.CreateRequest)
		want string
	}{
		{"Doctor", func(e *env, _ *port.CreateRequest) { e.doctor.report = failing }, "sbx: fehlt"},
		{"Agent", func(_ *env, r *port.CreateRequest) { r.Agent = "cursor" }, "unbekannter Agent"},
		{"Profil", func(_ *env, r *port.CreateRequest) { r.Profiles = []string{"rust"} }, "unbekanntes Profil"},
		{"Image und Dockerfile", func(_ *env, r *port.CreateRequest) { r.Image, r.Dockerfile = "a", "b" }, "schließen"},
		{"Repo unbekannt", func(_ *env, r *port.CreateRequest) { r.Repos = []string{"fehlt"} }, "nicht unter"},
		{"Secret ohne Hosts", func(e *env, r *port.CreateRequest) {
			e.config.SecretSets["/s"] = []secret.SecretSpec{{Name: "A", Value: "v"}}
			r.Secrets = "/s"
		}, "hosts fehlt"},
		{"Konfigurationsdatei", func(_ *env, r *port.CreateRequest) { r.Config = "/fehlt.yaml" }, "fehlt.yaml"},
		{"Secrets-Modus", func(e *env, r *port.CreateRequest) {
			e.config.Global = port.WorkspaceConfig{SecretsMode: "plain"}
		}, "secrets.mode"},
		{"Keine Repos", func(e *env, r *port.CreateRequest) { e.scanner.Repos = nil; r.Repos = nil }, "keine Git-Repos"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newEnv()
			req := request()
			tt.edit(e, &req)
			_, err := e.svc.Create(context.Background(), req)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Fehler = %v", err)
			}
			if len(e.journal.Entries) != 0 {
				t.Fatalf("Mutation vor der Prüfung: %v", e.journal.Entries)
			}
		})
	}
}

func TestExistingBranchFailsWithHint(t *testing.T) {
	e := newEnv()
	e.git.Branches["/src/api@boxai/shop"] = true
	_, err := e.svc.Create(context.Background(), request())
	if err == nil || !strings.Contains(err.Error(), "--checkout") {
		t.Fatal(err)
	}
	if !e.git.Branches["/src/api@boxai/shop"] {
		t.Fatal("fremder Branch wurde gelöscht")
	}
}

func TestCheckoutKeepsExistingBranchOnRollback(t *testing.T) {
	e := newEnv()
	e.runtime.FailOn["Create"] = errors.New("kaputt")
	e.git.Branches["/src/api@develop"] = true
	req := request()
	req.Repos = []string{"api"}
	req.Checkout = "develop"
	if _, err := e.svc.Create(context.Background(), req); err == nil {
		t.Fatal("Fehler erwartet")
	}
	if !e.git.Branches["/src/api@develop"] {
		t.Fatal("bestehender Branch gelöscht")
	}
}

func TestAttachFailureDoesNotRollBack(t *testing.T) {
	e := newEnv()
	e.runtime.FailOn["Attach"] = errors.New("exit 3")
	req := request()
	req.NoAttach = false
	if _, err := e.svc.Create(context.Background(), req); err == nil {
		t.Fatal("Fehler erwartet")
	}
	if len(e.git.Worktree) != 2 || len(e.runtime.Sandboxes) != 1 {
		t.Fatal("Attach-Fehler darf nichts entfernen")
	}
}

func TestSecretsEnvMode(t *testing.T) {
	e := newEnv()
	e.config.Global = port.WorkspaceConfig{SecretsMode: "env"}
	e.config.SecretSets["/s"] = []secret.SecretSpec{{Name: "AWS_KEY", Value: "v"}}
	req := request()
	req.Secrets = "/s"
	if _, err := e.svc.Create(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if len(e.secrets.Stored) != 0 {
		t.Fatal("Env-Modus darf den Store nicht nutzen")
	}
	if got := e.runtime.Specs[0].EnvSecrets; len(got) != 1 || got[0].Name != "AWS_KEY" {
		t.Fatalf("%v", got)
	}
	if !slices.ContainsFunc(e.notifier.Warns, func(s string) bool { return strings.Contains(s, "Klartext") }) {
		t.Fatal(e.notifier.Warns)
	}
}

func TestImageSelection(t *testing.T) {
	tests := []struct {
		name     string
		edit     func(*env, *port.CreateRequest)
		template string
		local    bool
		wantErr  string
		wantWarn string
	}{
		{"Profil-Image", func(e *env, r *port.CreateRequest) {
			r.Profiles = []string{"react"}
			e.images.Loaded["boxai-react-claude:latest"] = true
		}, "boxai-react-claude:latest", true, "", ""},
		{"Profil-Image fehlt", func(_ *env, r *port.CreateRequest) { r.Profiles = []string{"react"} }, "", false, "make images", ""},
		{"Ohne Profil und Image: Standard mit Warnung", func(*env, *port.CreateRequest) {}, "", false, "", "make images"},
		{"--no-docker", func(e *env, r *port.CreateRequest) {
			r.NoDocker = true
			e.images.Loaded["boxai-base-claude-nodocker:latest"] = true
		}, "boxai-base-claude-nodocker:latest", true, "", ""},
		{"Registry-Image", func(e *env, r *port.CreateRequest) {
			r.Image = "ghcr.io/acme/box:1"
			e.images.Labels[r.Image] = "claude"
		}, "ghcr.io/acme/box:1", false, "", ""},
		{"Lokales Image fehlt", func(_ *env, r *port.CreateRequest) { r.Image = "box:1" }, "", false, "Registry-Domain", ""},
		{"Lokales Image", func(e *env, r *port.CreateRequest) {
			r.Image = "box:1"
			e.images.Loaded["box:1"] = true
			e.images.Labels["box:1"] = "claude"
		}, "box:1", true, "", ""},
		{"Falsches Label", func(e *env, r *port.CreateRequest) {
			r.Image = "ghcr.io/acme/box:1"
			e.images.Labels[r.Image] = "copilot"
		}, "ghcr.io/acme/box:1", false, "", "boxai.agent"},
		{"Dockerfile", func(e *env, r *port.CreateRequest) {
			r.Dockerfile = "/d/Dockerfile"
			e.images.Labels["boxai-custom-shop:latest"] = "claude"
		}, "boxai-custom-shop:latest", true, "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newEnv()
			req := request()
			tt.edit(e, &req)
			_, err := e.svc.Create(context.Background(), req)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("Fehler = %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			spec := e.runtime.Specs[0]
			if spec.Template != tt.template || spec.LocalTemplate != tt.local {
				t.Fatalf("%+v", spec)
			}
			if tt.wantWarn != "" && !slices.ContainsFunc(e.notifier.Warns, func(s string) bool { return strings.Contains(s, tt.wantWarn) }) {
				t.Fatalf("%v", e.notifier.Warns)
			}
		})
	}
}

func TestBuiltinKitAgent(t *testing.T) {
	e := newEnv()
	req := request()
	req.Agent = "antigravity"
	if _, err := e.svc.Create(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if got := e.runtime.Specs[0].Agent; got != "/kits/builtin/antigravity" {
		t.Fatal(got)
	}
	if !slices.ContainsFunc(e.notifier.Warns, func(s string) bool { return strings.Contains(s, "experimentell") }) {
		t.Fatal(e.notifier.Warns)
	}
}

func TestAssetsAndContextEndUpInKit(t *testing.T) {
	e := newEnv()
	mapping := agent.AssetMapping{Source: ".claude/agents", Target: ".claude/agents"}
	e.assets.ByRepo["api"] = []agent.Asset{{Repo: "api", Mapping: mapping, Name: "a.md", Files: []agent.AssetFile{{Content: "A"}}}}
	e.assets.ByRepo["ui"] = []agent.Asset{{Repo: "ui", Mapping: mapping, Name: "a.md", Files: []agent.AssetFile{{Content: "U"}}}}
	e.assets.OKF["/ws/shop/api"] = ".okf/index.md"
	e.assets.Skipped = []string{"logo.png"}
	if _, err := e.svc.Create(context.Background(), request()); err != nil {
		t.Fatal(err)
	}
	kit := e.kits.Written["/kits/shop"]
	paths := map[string]string{}
	for _, f := range kit.Files {
		paths[f.Path] = f.Content
	}
	if paths["/home/agent/.claude/agents/api-a.md"] != "A" || paths["/home/agent/.claude/agents/ui-a.md"] != "U" {
		t.Fatalf("%+v", paths)
	}
	if !strings.Contains(paths["/ws/shop/AGENTS.md"], "api/.okf/index.md") {
		t.Fatal(paths["/ws/shop/AGENTS.md"])
	}
	if _, ok := paths["/ws/shop/CLAUDE.md"]; !ok {
		t.Fatal("CLAUDE.md fehlt")
	}
	joined := strings.Join(e.notifier.Warns, "\n")
	if !strings.Contains(joined, "Konflikt") || !strings.Contains(joined, "logo.png") {
		t.Fatal(joined)
	}
}

func TestDryRunReadsAssetsFromMainRepo(t *testing.T) {
	e := newEnv()
	e.assets.OKF["/src/api"] = ".okf/index.md"
	req := request()
	req.DryRun = true
	req.NoAttach = false
	if _, err := e.svc.Create(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if len(e.runtime.Attached) != 0 {
		t.Fatal("Dry-Run startet den Agent")
	}
	if !strings.Contains(e.kits.Written["/kits/shop"].Files[0].Content, "api/.okf/index.md") {
		t.Fatalf("%+v", e.kits.Written)
	}
}

func TestDryRunWithUnclonedRemoteRepo(t *testing.T) {
	e := newEnv()
	e.git.Missing["/cache/github.com/acme/lib"] = true
	e.git.DryRun = true
	e.git.FailOn["BranchExists"] = errors.New("kein Repo")
	req := request()
	req.DryRun = true
	req.Repos = []string{"https://github.com/acme/lib.git"}
	if _, err := e.svc.Create(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if got := e.runtime.Specs[0].Mounts[0].Path; got != "/cache/github.com/acme/lib/.git" {
		t.Fatal(got)
	}
}

func TestStepFailuresAreWrapped(t *testing.T) {
	boom := errors.New("boom")
	tests := []struct {
		name string
		edit func(*env)
	}{
		{"Kit", func(e *env) { e.kits.FailOn["Write"] = boom }},
		{"Validate", func(e *env) { e.kits.FailOn["Validate"] = boom }},
		{"Builtin", func(e *env) { e.kits.FailOn["Builtin"] = boom }},
		{"List", func(e *env) { e.runtime.FailOn["List"] = boom }},
		{"Save", func(e *env) { e.state.FailOn["Save"] = boom }},
		{"Assets", func(e *env) { e.assets.Err = boom }},
		{"BranchExists", func(e *env) { e.git.FailOn["BranchExists"] = boom }},
		{"CommonDir", func(e *env) { e.git.FailOn["CommonDir"] = boom }},
		{"Secrets", func(e *env) { e.secrets.FailOn["Put"] = boom }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newEnv()
			e.config.SecretSets["/s"] = []secret.SecretSpec{{Name: "A", Hosts: []string{"h.example"}, Value: "v"}}
			req := request()
			req.Secrets = "/s"
			if tt.name == "Builtin" {
				req.Agent = "antigravity"
			}
			tt.edit(e)
			if _, err := e.svc.Create(context.Background(), req); !errors.Is(err, boom) {
				t.Fatalf("Fehler = %v", err)
			}
		})
	}
}

func TestHasRegistryDomain(t *testing.T) {
	for ref, want := range map[string]bool{
		"ghcr.io/a/b:1": true, "localhost/a:1": true, "reg:5000/a": true,
		"a/b:1": false, "box:1": false,
	} {
		if got := hasRegistryDomain(ref); got != want {
			t.Errorf("%s: %v", ref, got)
		}
	}
}
