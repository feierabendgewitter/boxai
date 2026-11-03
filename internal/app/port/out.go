// Package port definiert die Inbound- und Outbound-Ports des Anwendungskerns.
package port

import (
	"context"
	"errors"
	"time"

	"github.com/feierabendgewitter/boxai/internal/domain/agent"
	"github.com/feierabendgewitter/boxai/internal/domain/profile"
	"github.com/feierabendgewitter/boxai/internal/domain/sandbox"
	"github.com/feierabendgewitter/boxai/internal/domain/secret"
	"github.com/feierabendgewitter/boxai/internal/domain/workspace"
)

// ErrStateNotFound meldet einen unbekannten Workspace.
var ErrStateNotFound = errors.New("workspace nicht gefunden")

// ToolInfo beschreibt den Fund eines Host-Tools.
type ToolInfo struct {
	Found   bool
	Version string
	Detail  string
}

// HostProbe prüft die Host-Umgebung für `boxai doctor`.
type HostProbe interface {
	Tool(ctx context.Context, name string) ToolInfo
	// SbxAuthenticated meldet, ob `sbx` bei Docker angemeldet ist.
	SbxAuthenticated(ctx context.Context) (bool, string)
	// NetworkAllowed fragt die sbx-Policy; sandbox leer bedeutet globale Policy.
	NetworkAllowed(ctx context.Context, sandbox, host string) (bool, error)
}

// RepoScanner findet lokale Git-Repos.
type RepoScanner interface {
	Scan(ctx context.Context, root string, depth int) ([]workspace.RepoRef, error)
}

// GitRepository kapselt Git-Operationen.
type GitRepository interface {
	DefaultBranch(ctx context.Context, repo string) (string, error)
	CurrentBranch(ctx context.Context, repo string) (string, error)
	IsDirty(ctx context.Context, repo string) (bool, error)
	Clone(ctx context.Context, url, dest string) error
	Fetch(ctx context.Context, repo string) error
	BranchExists(ctx context.Context, repo, branch string) (bool, error)
	AddWorktree(ctx context.Context, wt workspace.Worktree) error
	RemoveWorktree(ctx context.Context, wt workspace.Worktree, force bool) error
	PruneWorktrees(ctx context.Context, repo string) error
	DeleteBranch(ctx context.Context, repo, branch string) error
	// CommonDir liefert das absolute Git-Common-Dir eines Repos oder Worktrees.
	CommonDir(ctx context.Context, path string) (string, error)
}

// SandboxRuntime steuert Sandboxes.
type SandboxRuntime interface {
	Create(ctx context.Context, spec sandbox.SandboxSpec) error
	// Attach startet den Agent interaktiv und reicht stdin und stdout durch.
	Attach(ctx context.Context, name string) error
	Stop(ctx context.Context, name string) error
	Remove(ctx context.Context, name string) error
	List(ctx context.Context) ([]sandbox.Info, error)
	AllowNetwork(ctx context.Context, name string, rules []profile.NetworkRule) error
}

// SecretStore hinterlegt Secrets pro Sandbox.
type SecretStore interface {
	Put(ctx context.Context, sandboxName string, s secret.SecretSpec) error
	DeleteAll(ctx context.Context, sandboxName string) error
}

// KitWriter schreibt und prüft den generierten Kit.
type KitWriter interface {
	// Write erzeugt den Kit im Ordner dir.
	Write(ctx context.Context, kit sandbox.KitSpec, dir string) error
	Validate(ctx context.Context, dir string) error
	// Remove entfernt den Kit-Ordner.
	Remove(ctx context.Context, dir string) error
	// RemoveWorkspaceRoot entfernt generierte Dateien und den Ordner, falls er danach
	// leer ist. Fremde Einträge bleiben stehen und werden gemeldet.
	RemoveWorkspaceRoot(ctx context.Context, path string) (leftovers []string, err error)
	// Builtin stellt einen mitgelieferten Kit bereit und liefert seinen Ordner.
	Builtin(ctx context.Context, name string) (string, error)
}

// AssetSource liest Agent- und Skill-Dateien aus einem Worktree.
type AssetSource interface {
	Collect(ctx context.Context, repoName, worktreePath string, mappings []agent.AssetMapping) (AssetResult, error)
	// RepoContext erkennt OKF-Bundle und Anweisungsdateien eines Worktrees.
	RepoContext(ctx context.Context, worktreePath string) (okfIndex string, instructions []string, err error)
}

// AssetResult enthält gelesene Assets und übersprungene Dateien.
type AssetResult struct {
	Assets  []agent.Asset
	Skipped []string
}

// ImageBuilder baut und lädt Images.
type ImageBuilder interface {
	BuildAndLoad(ctx context.Context, dockerfile, tag string) (string, error)
	// Available meldet, ob ein lokales Template in der sbx-Runtime liegt.
	Available(ctx context.Context, ref string) (bool, error)
	// AgentLabel liefert das Label boxai.agent eines lokalen Images; leer, wenn es fehlt.
	AgentLabel(ctx context.Context, ref string) (string, error)
}

// RepoConfig ist ein Repo-Eintrag aus boxai.yaml.
type RepoConfig struct {
	Path string
	URL  string
	Base string
}

// WorkspaceConfig ist der Inhalt einer boxai.yaml oder ~/.boxai/config.yaml.
type WorkspaceConfig struct {
	Name         string
	Root         string
	Agent        string
	Profiles     []string
	Image        string
	Dockerfile   string
	Branch       string
	Base         string
	Checkout     string
	Repos        []RepoConfig
	Secrets      string
	SecretsMode  string
	NetworkAllow []string
	// Mappings ergänzt die Agent-Registry; nur in ~/.boxai/config.yaml sinnvoll.
	Mappings map[agent.Kind][]agent.AssetMapping
}

// ConfigLoader liest Konfiguration und Secrets.
type ConfigLoader interface {
	// LoadWorkspaceConfig löst relative Pfade gegen den Ordner der Datei auf.
	LoadWorkspaceConfig(ctx context.Context, path string) (WorkspaceConfig, error)
	// LoadGlobalConfig liest ~/.boxai/config.yaml; eine fehlende Datei ist kein Fehler.
	LoadGlobalConfig(ctx context.Context) (WorkspaceConfig, error)
	LoadSecrets(ctx context.Context, path string) ([]secret.SecretSpec, error)
	// NormalizePath macht einen Pfad absolut und löst Symlinks auf, auch wenn er noch nicht existiert.
	NormalizePath(ctx context.Context, path string) (string, error)
}

// WorkspaceStatus ist der Fortschritt eines Workspaces.
type WorkspaceStatus string

// Status-Werte.
const (
	StatusProvisioning WorkspaceStatus = "provisioning"
	StatusReady        WorkspaceStatus = "ready"
)

// WorkspaceState ist der persistierte Zustand. Er enthält nie Secret-Werte.
type WorkspaceState struct {
	Name        string
	Path        string
	SandboxName string
	Agent       agent.Kind
	KitDir      string
	Status      WorkspaceStatus
	// Planned sind alle vorgesehenen Worktrees, Worktrees die bereits angelegten.
	Planned   []workspace.Worktree
	Worktrees []workspace.Worktree
	// CreatedBranches listet Branches, die boxai neu angelegt hat, als repoPfad → Branch.
	CreatedBranches map[string]string
	CreatedAt       time.Time
}

// StateStore persistiert WorkspaceState.
type StateStore interface {
	Save(ctx context.Context, st WorkspaceState) error
	// Load liefert ErrStateNotFound, wenn der Workspace unbekannt ist.
	Load(ctx context.Context, name string) (WorkspaceState, error)
	List(ctx context.Context) ([]WorkspaceState, error)
	Delete(ctx context.Context, name string) error
}

// Prompter fragt interaktiv; vom CLI-Adapter implementiert.
type Prompter interface {
	SelectRepos(ctx context.Context, repos []workspace.RepoRef) ([]workspace.RepoRef, error)
	SelectAgent(ctx context.Context, options []agent.Kind) (agent.Kind, error)
	// InputName fragt den Workspace-Namen ab.
	InputName(ctx context.Context) (string, error)
}

// Notifier meldet Fortschritt und Warnungen an den Nutzer.
type Notifier interface {
	Info(msg string)
	Warn(msg string)
}

// Clock liefert die aktuelle Zeit.
type Clock func() time.Time
