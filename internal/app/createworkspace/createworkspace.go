// Package createworkspace implementiert den Use Case `boxai up`.
package createworkspace

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"github.com/feierabendgewitter/boxai/internal/app/port"
	"github.com/feierabendgewitter/boxai/internal/app/scanrepos"
	"github.com/feierabendgewitter/boxai/internal/domain/agent"
	"github.com/feierabendgewitter/boxai/internal/domain/profile"
	"github.com/feierabendgewitter/boxai/internal/domain/sandbox"
	"github.com/feierabendgewitter/boxai/internal/domain/secret"
	"github.com/feierabendgewitter/boxai/internal/domain/workspace"
)

// dryRunDefaultBranch steht im Plan für den noch unbekannten Branch eines ungeklonten Repos.
const dryRunDefaultBranch = "<default-branch>"

// Deps sind die Ports des Use Cases.
type Deps struct {
	Doctor   port.Doctor
	Scanner  port.RepoScanner
	Git      port.GitRepository
	Runtime  port.SandboxRuntime
	Secrets  port.SecretStore
	Kits     port.KitWriter
	Assets   port.AssetSource
	Images   port.ImageBuilder
	Config   port.ConfigLoader
	State    port.StateStore
	Prompter port.Prompter
	Notifier port.Notifier
	Registry *agent.Registry
	Now      port.Clock
}

// Service implementiert port.WorkspaceCreator.
type Service struct {
	Deps
}

// New erzeugt den Use Case.
func New(deps Deps) *Service { return &Service{Deps: deps} }

// run hält den Zustand eines Laufs.
type run struct {
	cfg      settings
	def      agent.Definition
	ws       workspace.Workspace
	state    port.WorkspaceState
	resumed  bool
	rollback rollbackStack
	secrets  []secret.SecretSpec
	mode     secret.Mode
	resolved profile.Resolved
}

// Create implementiert port.WorkspaceCreator.
func (s *Service) Create(ctx context.Context, req port.CreateRequest) (port.CreateResult, error) {
	r := &run{}
	if err := s.prepare(ctx, req, r); err != nil {
		return port.CreateResult{}, err
	}
	if err := s.provision(ctx, r); err != nil {
		if r.cfg.KeepOnErr {
			s.Notifier.Warn("--keep-on-error: Angelegtes bleibt erhalten, Fortsetzen mit erneutem boxai up")
			return port.CreateResult{}, err
		}
		// Das Aufräumen soll auch nach einem Abbruch (Strg-C) laufen.
		for _, cleanupErr := range r.rollback.run(context.WithoutCancel(ctx)) {
			s.Notifier.Warn("Rollback unvollständig: " + cleanupErr.Error())
		}
		return port.CreateResult{}, err
	}
	result := port.CreateResult{Name: r.ws.Name, Path: r.ws.Path, SandboxName: r.ws.SandboxName(), Resumed: r.resumed}
	if r.cfg.NoAttach || r.cfg.DryRun {
		return result, nil
	}
	// Ab hier steht der Workspace; Fehler beim Attach lösen kein Rollback aus.
	return result, s.Runtime.Attach(ctx, r.ws.SandboxName())
}

// prepare umfasst alles vor der ersten Mutation: Konfiguration, Auswahl und Planung.
func (s *Service) prepare(ctx context.Context, req port.CreateRequest, r *run) error {
	cfg, err := s.loadSettings(ctx, req)
	if err != nil {
		return err
	}
	r.cfg = cfg

	report := s.Doctor.Run(ctx, port.DoctorRequest{NeedDocker: cfg.Dockerfile != ""})
	if !report.Healthy() {
		return preflightError(report)
	}

	if r.mode, err = secret.ParseMode(cfg.SecretsMode); err != nil {
		return err
	}
	if cfg.Image != "" && cfg.Dockerfile != "" {
		return errors.New("--image und --dockerfile schließen sich aus")
	}
	if r.def, err = s.chooseAgent(ctx, cfg); err != nil {
		return err
	}
	if r.def.Experimental {
		s.Notifier.Warn(fmt.Sprintf("Agent %s ist experimentell", r.def.Kind))
	}
	if cfg.Secrets != "" {
		if r.secrets, err = s.Config.LoadSecrets(ctx, cfg.Secrets); err != nil {
			return fmt.Errorf("secrets laden: %w", err)
		}
	}
	if err := secret.ValidateAll(r.secrets, r.mode); err != nil {
		return err
	}
	if r.mode == secret.ModeEnv && len(r.secrets) > 0 {
		s.Notifier.Warn("secrets.mode: env übergibt Klartextwerte an die Sandbox; der Proxy schützt sie dann nicht")
	}
	if r.resolved, err = profile.Resolve(cfg.Profiles, append(slices.Clone(cfg.NetworkAllow), secretHosts(r.secrets)...)); err != nil {
		return err
	}
	return s.planWorkspace(ctx, r)
}

func (s *Service) loadSettings(ctx context.Context, req port.CreateRequest) (settings, error) {
	global, err := s.Config.LoadGlobalConfig(ctx)
	if err != nil {
		return settings{}, fmt.Errorf("globale Konfiguration: %w", err)
	}
	for kind, mappings := range global.Mappings {
		if err := s.Registry.Extend(kind, mappings); err != nil {
			return settings{}, err
		}
	}
	var file port.WorkspaceConfig
	if req.Config != "" {
		if file, err = s.Config.LoadWorkspaceConfig(ctx, req.Config); err != nil {
			return settings{}, fmt.Errorf("konfiguration %s: %w", req.Config, err)
		}
	}
	cfg := merge(req, global, file)
	if cfg.Root != "" {
		if cfg.Root, err = s.Config.NormalizePath(ctx, cfg.Root); err != nil {
			return settings{}, fmt.Errorf("workspace-root: %w", err)
		}
	}
	return cfg, nil
}

func preflightError(report port.DoctorReport) error {
	var failed []string
	for _, c := range report.Checks {
		if c.Required && !c.OK {
			failed = append(failed, c.Name+": "+c.Detail)
		}
	}
	return fmt.Errorf("voraussetzungen fehlen (boxai doctor): %s", strings.Join(failed, "; "))
}

func (s *Service) chooseAgent(ctx context.Context, cfg settings) (agent.Definition, error) {
	kind := agent.Kind(cfg.Agent)
	if cfg.Agent == "" {
		var err error
		if kind, err = s.Prompter.SelectAgent(ctx, s.Registry.Kinds()); err != nil {
			return agent.Definition{}, err
		}
	}
	return s.Registry.Get(kind)
}

func secretHosts(specs []secret.SecretSpec) []string {
	var hosts []string
	for _, spec := range specs {
		hosts = append(hosts, spec.Hosts...)
	}
	return hosts
}

// planWorkspace nutzt den gespeicherten Plan, wenn der Workspace schon existiert.
func (s *Service) planWorkspace(ctx context.Context, r *run) error {
	name := r.cfg.Name
	if name == "" {
		var err error
		if name, err = s.Prompter.InputName(ctx); err != nil {
			return err
		}
	}
	{
		st, err := s.State.Load(ctx, name)
		switch {
		case err == nil:
			r.state, r.resumed = st, true
			r.ws = workspace.Workspace{Name: st.Name, Path: st.Path, Root: filepath.Dir(st.Path), Worktrees: st.Planned}
			if st.Agent != r.def.Kind {
				return fmt.Errorf("workspace %s läuft mit Agent %s, nicht %s; zuerst boxai rm", name, st.Agent, r.def.Kind)
			}
			s.Notifier.Info("Workspace " + name + " existiert, setze fort")
			return nil
		case !errors.Is(err, port.ErrStateNotFound):
			return err
		}
	}
	selections, err := s.selectRepos(ctx, r.cfg)
	if err != nil {
		return err
	}
	if r.ws, err = workspace.Plan(workspace.PlanInput{
		Name: name,
		Root: r.cfg.Root,
		Strategy: workspace.BranchStrategy{
			Branch: r.cfg.Branch, Base: r.cfg.Base, Checkout: r.cfg.Checkout,
		},
		Repos: selections,
	}); err != nil {
		return err
	}
	r.state = port.WorkspaceState{
		Name:            r.ws.Name,
		Path:            r.ws.Path,
		SandboxName:     r.ws.SandboxName(),
		Agent:           r.def.Kind,
		KitDir:          filepath.Join(r.cfg.KitRoot, r.ws.Name),
		Status:          port.StatusProvisioning,
		Planned:         r.ws.Worktrees,
		CreatedBranches: map[string]string{},
		CreatedAt:       s.Now(),
	}
	return nil
}

// selectRepos sammelt Repos aus Konfiguration, Flags oder der interaktiven Auswahl.
func (s *Service) selectRepos(ctx context.Context, cfg settings) ([]workspace.RepoSelection, error) {
	var selections []workspace.RepoSelection
	for _, rc := range cfg.Repos {
		ref, err := s.resolveConfigRepo(ctx, cfg, rc)
		if err != nil {
			return nil, err
		}
		selections = append(selections, workspace.RepoSelection{Ref: ref, Base: rc.Base})
	}

	var scanned []workspace.RepoRef
	needScan := len(cfg.RepoArgs) > 0 && !allRemote(cfg.RepoArgs) || len(cfg.RepoArgs) == 0 && len(selections) == 0
	if needScan {
		var err error
		if scanned, err = scanrepos.New(s.Scanner, s.Git).Scan(ctx, port.ScanRequest{Dir: cfg.Dir, Depth: cfg.Depth}); err != nil {
			return nil, err
		}
	}
	for _, arg := range cfg.RepoArgs {
		ref, err := s.resolveRepoArg(ctx, cfg, arg, scanned)
		if err != nil {
			return nil, err
		}
		selections = append(selections, workspace.RepoSelection{Ref: ref})
	}
	if len(selections) == 0 {
		if len(scanned) == 0 {
			return nil, fmt.Errorf("keine Git-Repos unter %s gefunden", cfg.Dir)
		}
		chosen, err := s.Prompter.SelectRepos(ctx, scanned)
		if err != nil {
			return nil, err
		}
		for _, ref := range chosen {
			selections = append(selections, workspace.RepoSelection{Ref: ref})
		}
	}
	for _, sel := range selections {
		if sel.Ref.Dirty {
			s.Notifier.Warn(fmt.Sprintf("Repo %s hat uncommittete Änderungen; der Worktree basiert auf dem Commit, lokale Änderungen fehlen", sel.Ref.Name))
		}
	}
	return selections, nil
}

func allRemote(args []string) bool {
	return !slices.ContainsFunc(args, func(a string) bool { return !workspace.IsRemote(a) })
}

func (s *Service) resolveConfigRepo(ctx context.Context, cfg settings, rc port.RepoConfig) (workspace.RepoRef, error) {
	if rc.URL != "" {
		return s.cloneOrFetch(ctx, cfg, rc.URL)
	}
	ref := workspace.RepoRef{Name: filepath.Base(rc.Path), Path: rc.Path}
	return s.enrich(ctx, ref)
}

func (s *Service) resolveRepoArg(ctx context.Context, cfg settings, arg string, scanned []workspace.RepoRef) (workspace.RepoRef, error) {
	if workspace.IsRemote(arg) {
		return s.cloneOrFetch(ctx, cfg, arg)
	}
	for _, ref := range scanned {
		if ref.Name == arg {
			return ref, nil
		}
	}
	return workspace.RepoRef{}, fmt.Errorf("repo %q nicht unter %s gefunden", arg, cfg.Dir)
}

func (s *Service) cloneOrFetch(ctx context.Context, cfg settings, url string) (workspace.RepoRef, error) {
	remote, err := workspace.ParseRemote(url)
	if err != nil {
		return workspace.RepoRef{}, err
	}
	dest := remote.CachePath(cfg.CacheRoot)
	if _, err := s.Git.CommonDir(ctx, dest); err == nil {
		if err := s.Git.Fetch(ctx, dest); err != nil {
			return workspace.RepoRef{}, fmt.Errorf("fetch %s: %w", url, err)
		}
	} else {
		if err := s.Git.Clone(ctx, url, dest); err != nil {
			return workspace.RepoRef{}, fmt.Errorf("clone %s: %w", url, err)
		}
		if cfg.DryRun {
			// Der Clone ist nur geplant; es gibt noch nichts zu lesen.
			return workspace.RepoRef{Name: remote.Repo, Path: dest, URL: url, DefaultBranch: dryRunDefaultBranch}, nil
		}
	}
	return s.enrich(ctx, workspace.RepoRef{Name: remote.Repo, Path: dest, URL: url})
}

func (s *Service) enrich(ctx context.Context, ref workspace.RepoRef) (workspace.RepoRef, error) {
	var err error
	if ref.CurrentBranch, err = s.Git.CurrentBranch(ctx, ref.Path); err != nil {
		return ref, fmt.Errorf("repo %s: %w", ref.Name, err)
	}
	if ref.DefaultBranch, err = s.Git.DefaultBranch(ctx, ref.Path); err != nil {
		return ref, fmt.Errorf("repo %s: %w", ref.Name, err)
	}
	if ref.Dirty, err = s.Git.IsDirty(ctx, ref.Path); err != nil {
		return ref, fmt.Errorf("repo %s: %w", ref.Name, err)
	}
	return ref, nil
}

// provision führt die mutierenden Schritte aus; jeder registriert sein Rollback.
func (s *Service) provision(ctx context.Context, r *run) error {
	if err := s.addWorktrees(ctx, r); err != nil {
		return err
	}
	exists, err := s.sandboxExists(ctx, r.ws.SandboxName())
	if err != nil {
		return err
	}
	if exists {
		// Kit, Secrets und Mounts einer bestehenden Sandbox lassen sich nicht nachträglich ändern.
		s.Notifier.Info("Sandbox " + r.ws.SandboxName() + " existiert bereits")
		r.state.Status = port.StatusReady
		return s.State.Save(ctx, r.state)
	}
	spec, err := s.buildSpec(ctx, r)
	if err != nil {
		return err
	}
	if err := s.putSecrets(ctx, r); err != nil {
		return err
	}
	r.rollback.push("Sandbox entfernen", func(ctx context.Context) error {
		return s.Runtime.Remove(ctx, spec.Name)
	})
	if err := s.Runtime.Create(ctx, spec); err != nil {
		return fmt.Errorf("sandbox erstellen: %w", err)
	}
	if err := s.Runtime.AllowNetwork(ctx, spec.Name, r.resolved.Network); err != nil {
		return fmt.Errorf("netzwerk freigeben: %w", err)
	}
	r.state.Status = port.StatusReady
	return s.State.Save(ctx, r.state)
}

func (s *Service) addWorktrees(ctx context.Context, r *run) error {
	if !r.resumed {
		// Der State entsteht vor dem ersten Worktree, damit ein abgestürzter Lauf fortsetzbar bleibt.
		if err := s.saveInitialState(ctx, r); err != nil {
			return err
		}
	}
	existing := map[string]bool{}
	for _, wt := range r.state.Worktrees {
		existing[wt.Path] = true
	}
	for _, wt := range r.ws.Worktrees {
		if existing[wt.Path] {
			continue
		}
		if err := s.addWorktree(ctx, r, wt); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) saveInitialState(ctx context.Context, r *run) error {
	if err := s.State.Save(ctx, r.state); err != nil {
		return err
	}
	r.rollback.push("State löschen", func(ctx context.Context) error {
		return s.State.Delete(ctx, r.ws.Name)
	})
	return nil
}

func (s *Service) addWorktree(ctx context.Context, r *run, wt workspace.Worktree) error {
	if wt.CreateBranch {
		exists, err := s.Git.BranchExists(ctx, wt.RepoPath, wt.Branch)
		if err != nil && !r.cfg.DryRun {
			return fmt.Errorf("repo %s: %w", wt.RepoName, err)
		}
		if exists {
			return fmt.Errorf("repo %s: Branch %s existiert bereits; --checkout %s nutzt ihn", wt.RepoName, wt.Branch, wt.Branch)
		}
	}
	if err := s.Git.AddWorktree(ctx, wt); err != nil {
		return fmt.Errorf("repo %s: %w", wt.RepoName, err)
	}
	r.rollback.push("Worktree "+wt.RepoName+" entfernen", func(ctx context.Context) error {
		return s.Git.RemoveWorktree(ctx, wt, true)
	})
	if wt.CreateBranch {
		r.state.CreatedBranches = cloneMap(r.state.CreatedBranches)
		r.state.CreatedBranches[wt.RepoPath] = wt.Branch
		// Das Löschen läuft nach dem Entfernen des Worktrees, weil das Rollback rückwärts arbeitet.
		r.rollback.insertBeforeLast("Branch "+wt.Branch+" löschen", func(ctx context.Context) error {
			return s.Git.DeleteBranch(ctx, wt.RepoPath, wt.Branch)
		})
	}
	r.state.Worktrees = append(r.state.Worktrees, wt)
	return s.State.Save(ctx, r.state)
}

func cloneMap(in map[string]string) map[string]string {
	if in == nil {
		return map[string]string{}
	}
	return maps.Clone(in)
}

func (s *Service) sandboxExists(ctx context.Context, name string) (bool, error) {
	list, err := s.Runtime.List(ctx)
	if err != nil {
		return false, fmt.Errorf("sandboxes auflisten: %w", err)
	}
	return slices.ContainsFunc(list, func(i sandbox.Info) bool { return i.Name == name }), nil
}

// buildSpec sammelt Assets, schreibt den Kit und bestimmt das Image.
func (s *Service) buildSpec(ctx context.Context, r *run) (sandbox.SandboxSpec, error) {
	mounts, err := s.gitMounts(ctx, r)
	if err != nil {
		return sandbox.SandboxSpec{}, err
	}
	kitDir, err := s.writeKit(ctx, r)
	if err != nil {
		return sandbox.SandboxSpec{}, err
	}
	template, local, err := s.resolveImage(ctx, r)
	if err != nil {
		return sandbox.SandboxSpec{}, err
	}
	agentRef := r.def.SbxAgent
	if r.def.BuiltinKit != "" {
		if agentRef, err = s.Kits.Builtin(ctx, r.def.BuiltinKit); err != nil {
			return sandbox.SandboxSpec{}, fmt.Errorf("kit %s bereitstellen: %w", r.def.BuiltinKit, err)
		}
	}
	spec := sandbox.SandboxSpec{
		Name:          r.ws.SandboxName(),
		Agent:         agentRef,
		Workspace:     r.ws.Path,
		Mounts:        mounts,
		KitDir:        kitDir,
		Template:      template,
		LocalTemplate: local,
	}
	if r.mode == secret.ModeEnv {
		spec.EnvSecrets = r.secrets
	}
	return spec, nil
}

// gitMounts löst pro Haupt-Repo das Common-Dir auf; Duplikate entfallen.
func (s *Service) gitMounts(ctx context.Context, r *run) ([]sandbox.Mount, error) {
	var mounts []sandbox.Mount
	seen := map[string]bool{}
	for _, wt := range r.ws.Worktrees {
		dir, err := s.Git.CommonDir(ctx, wt.RepoPath)
		if err != nil && r.cfg.DryRun {
			dir, err = filepath.Join(wt.RepoPath, ".git"), nil
		}
		if err != nil {
			return nil, fmt.Errorf("repo %s: %w", wt.RepoName, err)
		}
		if !seen[dir] {
			seen[dir] = true
			mounts = append(mounts, sandbox.Mount{Path: dir})
		}
	}
	return mounts, nil
}

func (s *Service) writeKit(ctx context.Context, r *run) (string, error) {
	assets, repos, err := s.collectAssets(ctx, r)
	if err != nil {
		return "", err
	}
	placed, conflicts, err := r.def.Place(assets)
	if err != nil {
		return "", err
	}
	for _, c := range conflicts {
		s.Notifier.Warn(fmt.Sprintf("Konflikt bei %s (%s): %s", c.Name, strings.Join(c.Repos, ", "), c.Resolution))
	}
	kit, notes, err := sandbox.BuildKit(sandbox.KitInput{
		Workspace: r.ws,
		Agent:     r.def,
		Assets:    placed,
		Config:    r.resolved.FilesFor(secretNames(r.secrets)),
		Context: sandbox.RenderContext(sandbox.ContextInput{
			WorkspaceName: r.ws.Name,
			Repos:         repos,
			Profiles:      r.resolved.Names,
			Hints:         r.resolved.Hints,
			Agent:         r.def,
		}),
	})
	if err != nil {
		return "", err
	}
	for _, note := range notes {
		s.Notifier.Warn(note)
	}
	dir := r.state.KitDir
	r.rollback.push("Kit entfernen", func(ctx context.Context) error { return s.Kits.Remove(ctx, dir) })
	if err := s.Kits.Write(ctx, kit, dir); err != nil {
		return "", fmt.Errorf("kit schreiben: %w", err)
	}
	if err := s.Kits.Validate(ctx, dir); err != nil {
		return "", fmt.Errorf("kit prüfen: %w", err)
	}
	return dir, nil
}

func (s *Service) collectAssets(ctx context.Context, r *run) ([]agent.Asset, []sandbox.RepoContext, error) {
	var assets []agent.Asset
	var repos []sandbox.RepoContext
	for _, wt := range r.ws.Worktrees {
		source := wt.Path
		if r.cfg.DryRun {
			// Im Dry-Run gibt es keinen Worktree; das Haupt-Repo zeigt denselben Stand am ehesten.
			source = wt.RepoPath
		}
		res, err := s.Assets.Collect(ctx, wt.RepoName, source, r.def.Mappings)
		if err != nil {
			return nil, nil, fmt.Errorf("assets aus %s: %w", wt.RepoName, err)
		}
		for _, skipped := range res.Skipped {
			s.Notifier.Warn(fmt.Sprintf("Repo %s: %s übersprungen (keine Textdatei)", wt.RepoName, skipped))
		}
		assets = append(assets, res.Assets...)
		okf, instructions, err := s.Assets.RepoContext(ctx, source)
		if err != nil {
			return nil, nil, fmt.Errorf("kontext aus %s: %w", wt.RepoName, err)
		}
		repos = append(repos, sandbox.RepoContext{Name: wt.RepoName, Branch: wt.Branch, OKFIndex: okf, Instructions: instructions})
	}
	return assets, repos, nil
}

func secretNames(specs []secret.SecretSpec) []string {
	names := make([]string, len(specs))
	for i, spec := range specs {
		names[i] = spec.Name
	}
	return names
}

// resolveImage bestimmt das Template; der zweite Rückgabewert markiert lokale Images.
func (s *Service) resolveImage(ctx context.Context, r *run) (string, bool, error) {
	cfg := r.cfg
	switch {
	case cfg.Dockerfile != "":
		tag := "boxai-custom-" + r.ws.Name + ":latest"
		ref, err := s.Images.BuildAndLoad(ctx, cfg.Dockerfile, tag)
		if err != nil {
			return "", false, fmt.Errorf("image bauen: %w", err)
		}
		s.warnLabel(ctx, ref, r.def.Kind)
		return ref, true, nil
	case cfg.Image != "":
		local := !hasRegistryDomain(cfg.Image)
		if local {
			available, err := s.Images.Available(ctx, cfg.Image)
			if err != nil {
				return "", false, err
			}
			if !available {
				return "", false, fmt.Errorf("image %s: ohne Registry-Domain gilt es als lokal, liegt aber nicht in sbx; bitte Domain angeben oder mit sbx template load laden", cfg.Image)
			}
		}
		s.warnLabel(ctx, cfg.Image, r.def.Kind)
		return cfg.Image, local, nil
	}
	if r.def.BuiltinKit != "" {
		// Der Kit bringt sein Image selbst mit; ein Template würde dessen Agent ersetzen.
		if len(cfg.Profiles) > 0 {
			s.Notifier.Warn(fmt.Sprintf("Agent %s nutzt das Image seines Kits; Tools der Profile fehlen dort, Netzwerk und Konfiguration gelten trotzdem", r.def.Kind))
		}
		return "", false, nil
	}
	tag := profile.ImageTag(string(r.def.Kind), cfg.Profiles, cfg.NoDocker)
	available, err := s.Images.Available(ctx, tag)
	if err != nil {
		return "", false, err
	}
	if available {
		return tag, true, nil
	}
	if len(cfg.Profiles) > 0 || cfg.NoDocker {
		return "", false, fmt.Errorf("image %s fehlt; bitte im boxai-Repo make images ausführen", tag)
	}
	s.Notifier.Warn("Image " + tag + " fehlt (make images); nutze das Standard-Image des Agents, dort fehlen unter anderem glab und aws")
	return "", false, nil
}

func (s *Service) warnLabel(ctx context.Context, ref string, kind agent.Kind) {
	label, err := s.Images.AgentLabel(ctx, ref)
	if err != nil {
		s.Notifier.Warn(fmt.Sprintf("Label von %s nicht lesbar: %v", ref, err))
		return
	}
	if label != string(kind) {
		s.Notifier.Warn(fmt.Sprintf("Image %s trägt boxai.agent=%q, gewählt ist %s; es muss auf dem passenden Agent-Template basieren", ref, label, kind))
	}
}

// hasRegistryDomain folgt der Docker-Konvention: Der erste Teil enthält Punkt oder Port.
func hasRegistryDomain(ref string) bool {
	first, _, found := strings.Cut(ref, "/")
	return found && (strings.ContainsAny(first, ".:") || first == "localhost")
}

func (s *Service) putSecrets(ctx context.Context, r *run) error {
	if r.mode != secret.ModeProxy || len(r.secrets) == 0 {
		return nil
	}
	name := r.ws.SandboxName()
	r.rollback.push("Secrets löschen", func(ctx context.Context) error { return s.Secrets.DeleteAll(ctx, name) })
	// Secrets müssen vor der Sandbox stehen: Nur dann landet der Platzhalter in ihrer Umgebung.
	for _, spec := range r.secrets {
		if err := s.Secrets.Put(ctx, name, spec); err != nil {
			return fmt.Errorf("secret %s: %w", spec.Name, err)
		}
	}
	return nil
}
