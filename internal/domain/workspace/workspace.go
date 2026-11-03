// Package workspace enthält die Planung von Workspaces: Pfade, Branches und Namen.
package workspace

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// SandboxPrefix unterscheidet boxai-Sandboxes von fremden.
const SandboxPrefix = "boxai-"

var (
	namePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9.-]*$`)
	// ErrNoRepos meldet einen Workspace ohne Repos.
	ErrNoRepos = errors.New("workspace braucht mindestens ein Repo")
)

// RepoRef verweist auf ein lokales Repo.
type RepoRef struct {
	Name          string
	Path          string
	URL           string
	CurrentBranch string
	DefaultBranch string
	Dirty         bool
}

// Worktree beschreibt einen geplanten oder angelegten Worktree.
type Worktree struct {
	RepoName     string
	RepoPath     string
	Path         string
	Branch       string
	Base         string
	CreateBranch bool
}

// Workspace bündelt Worktrees unter einem gemeinsamen Ordner.
type Workspace struct {
	Name      string
	Root      string
	Path      string
	Worktrees []Worktree
}

// SandboxName liefert den Namen der zugehörigen Sandbox.
func (w Workspace) SandboxName() string { return SandboxName(w.Name) }

// BranchStrategy legt fest, welchen Branch die Worktrees bekommen.
type BranchStrategy struct {
	// Branch ist der neue Branch; leer bedeutet boxai/<workspace>.
	Branch string
	// Base ist der Startpunkt; leer bedeutet Default-Branch des Repos.
	Base string
	// Checkout wählt einen bestehenden Branch statt eines neuen.
	Checkout string
}

// RepoSelection ist ein gewähltes Repo mit optionalem eigenen Base-Branch.
type RepoSelection struct {
	Ref  RepoRef
	Base string
}

// PlanInput enthält alles für die Workspace-Planung.
type PlanInput struct {
	Name     string
	Root     string
	Strategy BranchStrategy
	Repos    []RepoSelection
}

// ValidateName prüft einen Workspace-Namen gegen die Regeln für Sandbox-Namen.
func ValidateName(name string) error {
	if len(name) < 2 || !namePattern.MatchString(name) {
		return fmt.Errorf("ungültiger Workspace-Name %q: mindestens zwei Zeichen, Buchstaben, Ziffern, '-' und '.'", name)
	}
	return nil
}

// SandboxName liefert den Sandbox-Namen eines Workspaces.
func SandboxName(name string) string { return SandboxPrefix + name }

// DefaultBranchName liefert den Default-Branch neuer Worktrees.
func DefaultBranchName(name string) string { return "boxai/" + name }

// Plan berechnet Pfade und Branches, ohne etwas anzulegen.
func Plan(in PlanInput) (Workspace, error) {
	if err := ValidateName(in.Name); err != nil {
		return Workspace{}, err
	}
	if in.Root == "" {
		return Workspace{}, errors.New("workspace-root fehlt")
	}
	if len(in.Repos) == 0 {
		return Workspace{}, ErrNoRepos
	}
	if in.Strategy.Checkout != "" && in.Strategy.Branch != "" {
		return Workspace{}, errors.New("--branch und --checkout schließen sich aus")
	}
	ws := Workspace{Name: in.Name, Root: in.Root, Path: filepath.Join(in.Root, in.Name)}
	seen := map[string]bool{}
	for _, sel := range in.Repos {
		if seen[sel.Ref.Name] {
			return Workspace{}, fmt.Errorf("zwei gewählte Repos heißen %q", sel.Ref.Name)
		}
		seen[sel.Ref.Name] = true
		wt, err := planWorktree(ws.Path, in.Name, in.Strategy, sel)
		if err != nil {
			return Workspace{}, err
		}
		ws.Worktrees = append(ws.Worktrees, wt)
	}
	return ws, nil
}

func planWorktree(wsPath, name string, s BranchStrategy, sel RepoSelection) (Worktree, error) {
	wt := Worktree{
		RepoName: sel.Ref.Name,
		RepoPath: sel.Ref.Path,
		Path:     filepath.Join(wsPath, sel.Ref.Name),
	}
	if s.Checkout != "" {
		wt.Branch = s.Checkout
		return wt, nil
	}
	wt.CreateBranch = true
	wt.Branch = s.Branch
	if wt.Branch == "" {
		wt.Branch = DefaultBranchName(name)
	}
	wt.Base = firstNonEmpty(sel.Base, s.Base, sel.Ref.DefaultBranch)
	if wt.Base == "" {
		return Worktree{}, fmt.Errorf("repo %s: kein Base-Branch ermittelbar, bitte --base setzen", sel.Ref.Name)
	}
	return wt, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// Remote ist eine zerlegte Clone-URL.
type Remote struct {
	Host  string
	Owner string
	Repo  string
}

// ParseRemote zerlegt HTTPS-, ssh://- und scp-artige Clone-URLs.
func ParseRemote(raw string) (Remote, error) {
	invalid := fmt.Errorf("keine gültige Clone-URL: %q", raw)
	s := strings.TrimSpace(raw)
	var host, path string
	switch {
	case strings.Contains(s, "://"):
		scheme, rest, _ := strings.Cut(s, "://")
		if scheme != "https" && scheme != "http" && scheme != "ssh" {
			return Remote{}, invalid
		}
		var found bool
		if host, path, found = strings.Cut(rest, "/"); !found {
			return Remote{}, invalid
		}
		if _, after, ok := strings.Cut(host, "@"); ok {
			host = after
		}
		host, _, _ = strings.Cut(host, ":")
	case strings.Contains(s, ":") && !strings.HasPrefix(s, "/"):
		var found bool
		if host, path, found = strings.Cut(s, ":"); !found {
			return Remote{}, invalid
		}
		if _, after, ok := strings.Cut(host, "@"); ok {
			host = after
		}
	default:
		return Remote{}, invalid
	}
	path = strings.TrimSuffix(strings.Trim(path, "/"), ".git")
	idx := strings.LastIndex(path, "/")
	if host == "" || idx <= 0 || idx == len(path)-1 {
		return Remote{}, invalid
	}
	for segment := range strings.SplitSeq(path, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return Remote{}, invalid
		}
	}
	return Remote{Host: host, Owner: path[:idx], Repo: path[idx+1:]}, nil
}

// IsRemote erkennt, ob ein Repo-Argument eine Clone-URL statt eines Namens ist.
func IsRemote(arg string) bool {
	_, err := ParseRemote(arg)
	return err == nil
}

// CachePath liefert den Clone-Ordner unter dem Cache-Root.
func (r Remote) CachePath(cacheRoot string) string {
	return filepath.Join(cacheRoot, r.Host, filepath.FromSlash(r.Owner), r.Repo)
}
