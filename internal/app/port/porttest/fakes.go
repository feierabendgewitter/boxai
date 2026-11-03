// Package porttest stellt In-Memory-Fakes aller Outbound-Ports bereit.
package porttest

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"

	"github.com/feierabendgewitter/boxai/internal/app/port"
	"github.com/feierabendgewitter/boxai/internal/domain/agent"
	"github.com/feierabendgewitter/boxai/internal/domain/profile"
	"github.com/feierabendgewitter/boxai/internal/domain/sandbox"
	"github.com/feierabendgewitter/boxai/internal/domain/secret"
	"github.com/feierabendgewitter/boxai/internal/domain/workspace"
)

// Journal zeichnet die Reihenfolge von Aufrufen über alle Fakes auf.
type Journal struct {
	mu      sync.Mutex
	Entries []string
}

// Add ergänzt einen Eintrag.
func (j *Journal) Add(format string, args ...any) {
	if j == nil {
		return
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	j.Entries = append(j.Entries, fmt.Sprintf(format, args...))
}

// Scanner ist ein Fake für port.RepoScanner.
type Scanner struct {
	Repos     []workspace.RepoRef
	Err       error
	LastDepth int
}

// Scan implementiert port.RepoScanner.
func (s *Scanner) Scan(_ context.Context, _ string, depth int) ([]workspace.RepoRef, error) {
	s.LastDepth = depth
	return slices.Clone(s.Repos), s.Err
}

// Git ist ein Fake für port.GitRepository.
type Git struct {
	Journal  *Journal
	Dirty    map[string]bool
	Worktree map[string]workspace.Worktree // Pfad → Worktree
	Branches map[string]bool               // "repo@branch"
	Clones   map[string]string             // dest → url
	Fetched  []string
	Pruned   []string
	// FailOn lässt eine Methode scheitern.
	FailOn map[string]error
	// FailAddFor lässt AddWorktree für ein bestimmtes Repo scheitern.
	FailAddFor string
	// ExistingPaths sind Repos, die schon geklont sind.
	ExistingPaths map[string]bool
}

// NewGit erzeugt einen leeren Fake.
func NewGit() *Git {
	return &Git{
		Dirty: map[string]bool{}, Worktree: map[string]workspace.Worktree{},
		Branches: map[string]bool{}, Clones: map[string]string{}, FailOn: map[string]error{},
		ExistingPaths: map[string]bool{},
	}
}

// DefaultBranch implementiert port.GitRepository.
func (g *Git) DefaultBranch(context.Context, string) (string, error) {
	return "main", g.FailOn["DefaultBranch"]
}

// CurrentBranch implementiert port.GitRepository.
func (g *Git) CurrentBranch(context.Context, string) (string, error) {
	return "main", g.FailOn["CurrentBranch"]
}

// IsDirty implementiert port.GitRepository.
func (g *Git) IsDirty(_ context.Context, repo string) (bool, error) {
	return g.Dirty[repo], g.FailOn["IsDirty"]
}

// Clone implementiert port.GitRepository.
func (g *Git) Clone(_ context.Context, url, dest string) error {
	g.Journal.Add("git clone %s", url)
	if err := g.FailOn["Clone"]; err != nil {
		return err
	}
	g.Clones[dest] = url
	return nil
}

// Fetch implementiert port.GitRepository.
func (g *Git) Fetch(_ context.Context, repo string) error {
	g.Journal.Add("git fetch %s", repo)
	g.Fetched = append(g.Fetched, repo)
	return g.FailOn["Fetch"]
}

// BranchExists implementiert port.GitRepository.
func (g *Git) BranchExists(_ context.Context, repo, branch string) (bool, error) {
	return g.Branches[repo+"@"+branch], g.FailOn["BranchExists"]
}

// AddWorktree implementiert port.GitRepository.
func (g *Git) AddWorktree(_ context.Context, wt workspace.Worktree) error {
	g.Journal.Add("worktree add %s", wt.RepoName)
	if err := g.FailOn["AddWorktree"]; err != nil {
		return err
	}
	if g.FailAddFor == wt.RepoName {
		return errors.New("worktree add fehlgeschlagen: " + wt.RepoName)
	}
	g.Worktree[wt.Path] = wt
	if wt.CreateBranch {
		g.Branches[wt.RepoPath+"@"+wt.Branch] = true
	}
	return nil
}

// RemoveWorktree implementiert port.GitRepository.
func (g *Git) RemoveWorktree(_ context.Context, wt workspace.Worktree, force bool) error {
	g.Journal.Add("worktree remove %s force=%v", wt.RepoName, force)
	if err := g.FailOn["RemoveWorktree"]; err != nil {
		return err
	}
	delete(g.Worktree, wt.Path)
	return nil
}

// PruneWorktrees implementiert port.GitRepository.
func (g *Git) PruneWorktrees(_ context.Context, repo string) error {
	g.Pruned = append(g.Pruned, repo)
	return g.FailOn["PruneWorktrees"]
}

// DeleteBranch implementiert port.GitRepository.
func (g *Git) DeleteBranch(_ context.Context, repo, branch string) error {
	g.Journal.Add("branch delete %s %s", repo, branch)
	delete(g.Branches, repo+"@"+branch)
	return g.FailOn["DeleteBranch"]
}

// CommonDir implementiert port.GitRepository.
func (g *Git) CommonDir(_ context.Context, path string) (string, error) {
	return path + "/.git", g.FailOn["CommonDir"]
}

// Runtime ist ein Fake für port.SandboxRuntime.
type Runtime struct {
	Journal   *Journal
	Sandboxes map[string]sandbox.Info
	Specs     []sandbox.SandboxSpec
	Network   map[string][]profile.NetworkRule
	Attached  []string
	FailOn    map[string]error
}

// NewRuntime erzeugt einen leeren Fake.
func NewRuntime() *Runtime {
	return &Runtime{Sandboxes: map[string]sandbox.Info{}, Network: map[string][]profile.NetworkRule{}, FailOn: map[string]error{}}
}

// Create implementiert port.SandboxRuntime.
func (r *Runtime) Create(_ context.Context, spec sandbox.SandboxSpec) error {
	r.Journal.Add("sandbox create %s", spec.Name)
	if err := r.FailOn["Create"]; err != nil {
		return err
	}
	r.Specs = append(r.Specs, spec)
	r.Sandboxes[spec.Name] = sandbox.Info{Name: spec.Name, Agent: spec.Agent, Status: "running"}
	return nil
}

// Attach implementiert port.SandboxRuntime.
func (r *Runtime) Attach(_ context.Context, name string) error {
	r.Journal.Add("sandbox attach %s", name)
	r.Attached = append(r.Attached, name)
	return r.FailOn["Attach"]
}

// Stop implementiert port.SandboxRuntime.
func (r *Runtime) Stop(_ context.Context, name string) error {
	r.Journal.Add("sandbox stop %s", name)
	return r.FailOn["Stop"]
}

// Remove implementiert port.SandboxRuntime.
func (r *Runtime) Remove(_ context.Context, name string) error {
	r.Journal.Add("sandbox remove %s", name)
	if err := r.FailOn["Remove"]; err != nil {
		return err
	}
	delete(r.Sandboxes, name)
	return nil
}

// List implementiert port.SandboxRuntime.
func (r *Runtime) List(context.Context) ([]sandbox.Info, error) {
	out := make([]sandbox.Info, 0, len(r.Sandboxes))
	for _, s := range r.Sandboxes {
		out = append(out, s)
	}
	return out, r.FailOn["List"]
}

// AllowNetwork implementiert port.SandboxRuntime.
func (r *Runtime) AllowNetwork(_ context.Context, name string, rules []profile.NetworkRule) error {
	r.Journal.Add("sandbox network %s", name)
	if err := r.FailOn["AllowNetwork"]; err != nil {
		return err
	}
	r.Network[name] = append(r.Network[name], rules...)
	return nil
}

// Secrets ist ein Fake für port.SecretStore.
type Secrets struct {
	Journal *Journal
	Stored  map[string][]secret.SecretSpec
	FailOn  map[string]error
}

// NewSecrets erzeugt einen leeren Fake.
func NewSecrets() *Secrets {
	return &Secrets{Stored: map[string][]secret.SecretSpec{}, FailOn: map[string]error{}}
}

// Put implementiert port.SecretStore.
func (s *Secrets) Put(_ context.Context, name string, spec secret.SecretSpec) error {
	s.Journal.Add("secret put %s", spec.Name)
	if err := s.FailOn["Put"]; err != nil {
		return err
	}
	s.Stored[name] = append(s.Stored[name], spec)
	return nil
}

// DeleteAll implementiert port.SecretStore.
func (s *Secrets) DeleteAll(_ context.Context, name string) error {
	s.Journal.Add("secret delete-all %s", name)
	delete(s.Stored, name)
	return s.FailOn["DeleteAll"]
}

// Kits ist ein Fake für port.KitWriter.
type Kits struct {
	Journal  *Journal
	Written  map[string]sandbox.KitSpec
	Removed  []string
	Builtins map[string]string
	FailOn   map[string]error
}

// NewKits erzeugt einen leeren Fake.
func NewKits() *Kits {
	return &Kits{Written: map[string]sandbox.KitSpec{}, Builtins: map[string]string{}, FailOn: map[string]error{}}
}

// Write implementiert port.KitWriter.
func (k *Kits) Write(_ context.Context, kit sandbox.KitSpec, dir string) error {
	k.Journal.Add("kit write %s", kit.Name)
	if err := k.FailOn["Write"]; err != nil {
		return err
	}
	k.Written[dir] = kit
	return nil
}

// Validate implementiert port.KitWriter.
func (k *Kits) Validate(context.Context, string) error {
	k.Journal.Add("kit validate")
	return k.FailOn["Validate"]
}

// Remove implementiert port.KitWriter.
func (k *Kits) Remove(_ context.Context, dir string) error {
	k.Journal.Add("kit remove")
	k.Removed = append(k.Removed, dir)
	delete(k.Written, dir)
	return k.FailOn["Remove"]
}

// Builtin implementiert port.KitWriter.
func (k *Kits) Builtin(_ context.Context, name string) (string, error) {
	dir := "/kits/builtin/" + name
	k.Builtins[name] = dir
	return dir, k.FailOn["Builtin"]
}

// Assets ist ein Fake für port.AssetSource.
type Assets struct {
	ByRepo       map[string][]agent.Asset
	Skipped      []string
	OKF          map[string]string
	Instructions map[string][]string
	Err          error
}

// NewAssets erzeugt einen leeren Fake.
func NewAssets() *Assets {
	return &Assets{ByRepo: map[string][]agent.Asset{}, OKF: map[string]string{}, Instructions: map[string][]string{}}
}

// Collect implementiert port.AssetSource.
func (a *Assets) Collect(_ context.Context, repo, _ string, _ []agent.AssetMapping) (port.AssetResult, error) {
	return port.AssetResult{Assets: a.ByRepo[repo], Skipped: a.Skipped}, a.Err
}

// RepoContext implementiert port.AssetSource.
func (a *Assets) RepoContext(_ context.Context, path string) (string, []string, error) {
	return a.OKF[path], a.Instructions[path], a.Err
}

// Images ist ein Fake für port.ImageBuilder.
type Images struct {
	Journal *Journal
	Loaded  map[string]bool
	Labels  map[string]string
	Built   []string
	Err     error
}

// NewImages erzeugt einen leeren Fake.
func NewImages() *Images {
	return &Images{Loaded: map[string]bool{}, Labels: map[string]string{}}
}

// BuildAndLoad implementiert port.ImageBuilder.
func (i *Images) BuildAndLoad(_ context.Context, dockerfile, tag string) (string, error) {
	i.Journal.Add("image build %s", dockerfile)
	i.Built = append(i.Built, dockerfile)
	i.Loaded[tag] = true
	return tag, i.Err
}

// Available implementiert port.ImageBuilder.
func (i *Images) Available(_ context.Context, ref string) (bool, error) {
	return i.Loaded[ref], nil
}

// AgentLabel implementiert port.ImageBuilder.
func (i *Images) AgentLabel(_ context.Context, ref string) (string, error) {
	return i.Labels[ref], nil
}

// Config ist ein Fake für port.ConfigLoader.
type Config struct {
	Files      map[string]port.WorkspaceConfig
	Global     port.WorkspaceConfig
	SecretSets map[string][]secret.SecretSpec
	Err        error
}

// NewConfig erzeugt einen leeren Fake.
func NewConfig() *Config {
	return &Config{Files: map[string]port.WorkspaceConfig{}, SecretSets: map[string][]secret.SecretSpec{}}
}

// LoadWorkspaceConfig implementiert port.ConfigLoader.
func (c *Config) LoadWorkspaceConfig(_ context.Context, path string) (port.WorkspaceConfig, error) {
	cfg, ok := c.Files[path]
	if !ok {
		return port.WorkspaceConfig{}, fmt.Errorf("keine Konfiguration unter %s", path)
	}
	return cfg, c.Err
}

// LoadGlobalConfig implementiert port.ConfigLoader.
func (c *Config) LoadGlobalConfig(context.Context) (port.WorkspaceConfig, error) {
	return c.Global, c.Err
}

// LoadSecrets implementiert port.ConfigLoader.
func (c *Config) LoadSecrets(_ context.Context, path string) ([]secret.SecretSpec, error) {
	return c.SecretSets[path], c.Err
}

// State ist ein Fake für port.StateStore.
type State struct {
	Journal *Journal
	States  map[string]port.WorkspaceState
	FailOn  map[string]error
}

// NewState erzeugt einen leeren Fake.
func NewState() *State {
	return &State{States: map[string]port.WorkspaceState{}, FailOn: map[string]error{}}
}

// Save implementiert port.StateStore.
func (s *State) Save(_ context.Context, st port.WorkspaceState) error {
	s.Journal.Add("state save %s", st.Status)
	if err := s.FailOn["Save"]; err != nil {
		return err
	}
	s.States[st.Name] = st
	return nil
}

// Load implementiert port.StateStore.
func (s *State) Load(_ context.Context, name string) (port.WorkspaceState, error) {
	st, ok := s.States[name]
	if !ok {
		return port.WorkspaceState{}, port.ErrStateNotFound
	}
	return st, s.FailOn["Load"]
}

// List implementiert port.StateStore.
func (s *State) List(context.Context) ([]port.WorkspaceState, error) {
	out := make([]port.WorkspaceState, 0, len(s.States))
	for _, st := range s.States {
		out = append(out, st)
	}
	slices.SortFunc(out, func(a, b port.WorkspaceState) int {
		if a.Name < b.Name {
			return -1
		}
		return 1
	})
	return out, s.FailOn["List"]
}

// Delete implementiert port.StateStore.
func (s *State) Delete(_ context.Context, name string) error {
	s.Journal.Add("state delete %s", name)
	delete(s.States, name)
	return s.FailOn["Delete"]
}

// Prompter ist ein Fake für port.Prompter.
type Prompter struct {
	Repos    []string
	Agent    agent.Kind
	Name     string
	Calls    int
	OfferedN int
}

// SelectRepos implementiert port.Prompter.
func (p *Prompter) SelectRepos(_ context.Context, repos []workspace.RepoRef) ([]workspace.RepoRef, error) {
	p.Calls++
	p.OfferedN = len(repos)
	var out []workspace.RepoRef
	for _, r := range repos {
		if slices.Contains(p.Repos, r.Name) {
			out = append(out, r)
		}
	}
	return out, nil
}

// SelectAgent implementiert port.Prompter.
func (p *Prompter) SelectAgent(context.Context, []agent.Kind) (agent.Kind, error) {
	p.Calls++
	return p.Agent, nil
}

// InputName implementiert port.Prompter.
func (p *Prompter) InputName(context.Context) (string, error) {
	p.Calls++
	return p.Name, nil
}

// Notifier ist ein Fake für port.Notifier.
type Notifier struct {
	Infos []string
	Warns []string
}

// Info implementiert port.Notifier.
func (n *Notifier) Info(msg string) { n.Infos = append(n.Infos, msg) }

// Warn implementiert port.Notifier.
func (n *Notifier) Warn(msg string) { n.Warns = append(n.Warns, msg) }

var (
	_ port.RepoScanner    = (*Scanner)(nil)
	_ port.GitRepository  = (*Git)(nil)
	_ port.SandboxRuntime = (*Runtime)(nil)
	_ port.SecretStore    = (*Secrets)(nil)
	_ port.KitWriter      = (*Kits)(nil)
	_ port.AssetSource    = (*Assets)(nil)
	_ port.ImageBuilder   = (*Images)(nil)
	_ port.ConfigLoader   = (*Config)(nil)
	_ port.StateStore     = (*State)(nil)
	_ port.Prompter       = (*Prompter)(nil)
	_ port.Notifier       = (*Notifier)(nil)
)
