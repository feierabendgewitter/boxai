package port

import (
	"context"

	"github.com/feierabendgewitter/boxai/internal/domain/sandbox"
	"github.com/feierabendgewitter/boxai/internal/domain/workspace"
)

// DoctorRequest wählt optionale Prüfungen.
type DoctorRequest struct {
	NeedDocker bool
}

// DoctorCheck ist das Ergebnis einer Prüfung.
type DoctorCheck struct {
	Name     string
	OK       bool
	Required bool
	Detail   string
}

// DoctorReport fasst alle Prüfungen zusammen.
type DoctorReport struct {
	Checks []DoctorCheck
}

// Healthy meldet, ob alle Pflichtprüfungen bestanden sind.
func (r DoctorReport) Healthy() bool {
	for _, c := range r.Checks {
		if c.Required && !c.OK {
			return false
		}
	}
	return true
}

// Doctor prüft die Voraussetzungen von boxai.
type Doctor interface {
	Run(ctx context.Context, req DoctorRequest) DoctorReport
}

// ScanRequest parametriert den Repo-Scan.
type ScanRequest struct {
	Dir   string
	Depth int
}

// RepoScan listet Repos samt Branch und Dirty-Status.
type RepoScan interface {
	Scan(ctx context.Context, req ScanRequest) ([]workspace.RepoRef, error)
}

// CreateRequest enthält die Flags von `boxai up`. Leere Werte bedeuten „nicht gesetzt“.
type CreateRequest struct {
	Dir        string
	Depth      int
	Root       string
	Name       string
	Repos      []string // Namen oder URLs
	Base       string
	Branch     string
	Checkout   string
	Agent      string
	Profiles   []string
	Image      string
	Dockerfile string
	Secrets    string
	Config     string
	NoDocker   bool
	KeepOnErr  bool
	// DefaultRoot gilt, wenn weder Flag noch Konfiguration eine Wurzel setzen.
	DefaultRoot string
	// CacheRoot ist der Clone-Ordner für Remote-Repos.
	CacheRoot string
	// KitRoot enthält die generierten Kits.
	KitRoot string
	// NoAttach legt alles an, startet den Agent aber nicht.
	NoAttach bool
	// DryRun überspringt Schritte, die auf bereits angelegten Worktrees aufbauen.
	DryRun bool
}

// CreateResult fasst einen erfolgreichen Lauf zusammen.
type CreateResult struct {
	Name        string
	Path        string
	SandboxName string
	Resumed     bool
}

// WorkspaceCreator ist der Use Case `boxai up`.
type WorkspaceCreator interface {
	Create(ctx context.Context, req CreateRequest) (CreateResult, error)
}

// RemoveRequest parametriert `boxai rm`.
type RemoveRequest struct {
	Name           string
	DeleteBranches bool
	Force          bool
}

// WorkspaceRemover ist der Use Case `boxai rm`.
type WorkspaceRemover interface {
	Remove(ctx context.Context, req RemoveRequest) error
}

// WorkspaceInfo ist eine Zeile von `boxai list`.
type WorkspaceInfo struct {
	State         WorkspaceState
	SandboxStatus string
	Repos         []string
}

// Lifecycle bündelt attach, stop und list.
type Lifecycle interface {
	Attach(ctx context.Context, name string) error
	Stop(ctx context.Context, name string) error
	List(ctx context.Context) ([]WorkspaceInfo, error)
}

// SandboxLister ist für Adapter nützlich, die nur den Status brauchen.
type SandboxLister interface {
	List(ctx context.Context) ([]sandbox.Info, error)
}
