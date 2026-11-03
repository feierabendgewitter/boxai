// Package gitcli implementiert port.GitRepository über das git-Binary.
package gitcli

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/feierabendgewitter/boxai/internal/adapter/out/execx"
	"github.com/feierabendgewitter/boxai/internal/domain/workspace"
)

// Git implementiert port.GitRepository.
type Git struct {
	runner execx.Runner
}

// New erzeugt den Adapter.
func New(runner execx.Runner) *Git { return &Git{runner: runner} }

func (g *Git) read(ctx context.Context, repo string, args ...string) (string, error) {
	res, err := g.runner.Run(ctx, execx.Cmd{Name: "git", Args: append([]string{"-C", repo}, args...), ReadOnly: true})
	return strings.TrimSpace(res.Stdout), err
}

func (g *Git) write(ctx context.Context, args ...string) error {
	_, err := g.runner.Run(ctx, execx.Cmd{Name: "git", Args: args})
	return err
}

// DefaultBranch nutzt origin/HEAD, sonst main oder master, sonst den aktuellen Branch.
func (g *Git) DefaultBranch(ctx context.Context, repo string) (string, error) {
	if out, err := g.read(ctx, repo, "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD"); err == nil && out != "" {
		return strings.TrimPrefix(out, "origin/"), nil
	}
	for _, name := range []string{"main", "master"} {
		if ok, err := g.BranchExists(ctx, repo, name); err == nil && ok {
			return name, nil
		}
	}
	return g.CurrentBranch(ctx, repo)
}

// CurrentBranch liefert den Branch; bei losgelöstem HEAD den Kurz-Hash.
func (g *Git) CurrentBranch(ctx context.Context, repo string) (string, error) {
	out, err := g.read(ctx, repo, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err == nil && out != "" {
		return out, nil
	}
	return g.read(ctx, repo, "rev-parse", "--short", "HEAD")
}

// IsDirty meldet uncommittete Änderungen.
func (g *Git) IsDirty(ctx context.Context, repo string) (bool, error) {
	out, err := g.read(ctx, repo, "status", "--porcelain")
	return out != "", err
}

// Clone klont ein Remote-Repo.
func (g *Git) Clone(ctx context.Context, url, dest string) error {
	return g.write(ctx, "clone", "--", url, dest)
}

// Fetch aktualisiert einen vorhandenen Clone.
func (g *Git) Fetch(ctx context.Context, repo string) error {
	return g.write(ctx, "-C", repo, "fetch", "--prune", "origin")
}

// BranchExists prüft einen lokalen Branch.
func (g *Git) BranchExists(ctx context.Context, repo, branch string) (bool, error) {
	_, err := g.read(ctx, repo, "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	if err == nil {
		return true, nil
	}
	var exit *execx.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 1 {
		return false, nil
	}
	return false, err
}

// AddWorktree legt einen Worktree an.
func (g *Git) AddWorktree(ctx context.Context, wt workspace.Worktree) error {
	args := []string{"-C", wt.RepoPath, "worktree", "add"}
	if wt.CreateBranch {
		args = append(args, "-b", wt.Branch, wt.Path, wt.Base)
	} else {
		args = append(args, wt.Path, wt.Branch)
	}
	return g.write(ctx, args...)
}

// RemoveWorktree entfernt einen Worktree; ohne force scheitert git bei Änderungen.
func (g *Git) RemoveWorktree(ctx context.Context, wt workspace.Worktree, force bool) error {
	args := []string{"-C", wt.RepoPath, "worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	return g.write(ctx, append(args, wt.Path)...)
}

// PruneWorktrees räumt verwaiste Worktree-Einträge auf.
func (g *Git) PruneWorktrees(ctx context.Context, repo string) error {
	return g.write(ctx, "-C", repo, "worktree", "prune")
}

// DeleteBranch löscht einen Branch auch ungemergt; der Nutzer fordert das mit --delete-branches an.
func (g *Git) DeleteBranch(ctx context.Context, repo, branch string) error {
	return g.write(ctx, "-C", repo, "branch", "-D", "--", branch)
}

// CommonDir liefert das absolute Git-Common-Dir ohne Symlinks.
func (g *Git) CommonDir(ctx context.Context, path string) (string, error) {
	out, err := g.read(ctx, path, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return "", err
	}
	if out == "" {
		return "", fmt.Errorf("git liefert kein Common-Dir für %s", path)
	}
	resolved, err := filepath.EvalSymlinks(out)
	if err != nil {
		return "", err
	}
	return resolved, nil
}
