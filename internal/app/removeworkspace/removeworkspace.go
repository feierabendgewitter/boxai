// Package removeworkspace implementiert den Use Case `boxai rm`.
package removeworkspace

import (
	"context"
	"fmt"
	"strings"

	"github.com/feierabendgewitter/boxai/internal/app/port"
	"github.com/feierabendgewitter/boxai/internal/domain/workspace"
)

// Service implementiert port.WorkspaceRemover.
type Service struct {
	git      port.GitRepository
	runtime  port.SandboxRuntime
	secrets  port.SecretStore
	kits     port.KitWriter
	state    port.StateStore
	notifier port.Notifier
}

// New erzeugt den Use Case.
func New(git port.GitRepository, runtime port.SandboxRuntime, secrets port.SecretStore, kits port.KitWriter, state port.StateStore, notifier port.Notifier) *Service {
	return &Service{git: git, runtime: runtime, secrets: secrets, kits: kits, state: state, notifier: notifier}
}

// Remove entfernt Sandbox, Secrets, Worktrees, Kit und State. Uncommittete Änderungen
// brechen vor dem ersten Schritt ab, damit nichts halb entfernt zurückbleibt.
func (s *Service) Remove(ctx context.Context, req port.RemoveRequest) error {
	st, err := s.state.Load(ctx, req.Name)
	if err != nil {
		return err
	}
	if !req.Force {
		if err := s.ensureClean(ctx, st); err != nil {
			return err
		}
	}
	if err := s.runtime.Remove(ctx, st.SandboxName); err != nil {
		return fmt.Errorf("sandbox entfernen: %w", err)
	}
	if err := s.secrets.DeleteAll(ctx, st.SandboxName); err != nil {
		return fmt.Errorf("secrets löschen: %w", err)
	}
	repos := map[string]bool{}
	for _, wt := range st.Worktrees {
		repos[wt.RepoPath] = true
		if err := s.git.RemoveWorktree(ctx, wt, req.Force); err != nil {
			return fmt.Errorf("worktree %s entfernen: %w", wt.RepoName, err)
		}
	}
	for repo := range repos {
		if err := s.git.PruneWorktrees(ctx, repo); err != nil {
			return fmt.Errorf("worktree prune in %s: %w", repo, err)
		}
	}
	if req.DeleteBranches {
		// Nur Branches, die boxai selbst angelegt hat; --checkout-Branches gehören dem Nutzer.
		for repo, branch := range st.CreatedBranches {
			if err := s.git.DeleteBranch(ctx, repo, branch); err != nil {
				return fmt.Errorf("branch %s in %s löschen: %w", branch, repo, err)
			}
		}
	}
	if st.KitDir != "" {
		if err := s.kits.Remove(ctx, st.KitDir); err != nil {
			return fmt.Errorf("kit entfernen: %w", err)
		}
	}
	leftovers, err := s.kits.RemoveWorkspaceRoot(ctx, st.Path)
	if err != nil {
		return fmt.Errorf("workspace-ordner entfernen: %w", err)
	}
	if len(leftovers) > 0 {
		s.notifier.Warn(fmt.Sprintf("%s bleibt bestehen, fremde Einträge: %s", st.Path, strings.Join(leftovers, ", ")))
	}
	return s.state.Delete(ctx, st.Name)
}

func (s *Service) ensureClean(ctx context.Context, st port.WorkspaceState) error {
	var dirty []string
	for _, wt := range st.Worktrees {
		isDirty, err := s.worktreeDirty(ctx, wt)
		if err != nil {
			return err
		}
		if isDirty {
			dirty = append(dirty, wt.RepoName)
		}
	}
	if len(dirty) > 0 {
		return fmt.Errorf("uncommittete Änderungen in %s; committen oder --force", strings.Join(dirty, ", "))
	}
	return nil
}

func (s *Service) worktreeDirty(ctx context.Context, wt workspace.Worktree) (bool, error) {
	isDirty, err := s.git.IsDirty(ctx, wt.Path)
	if err != nil {
		return false, fmt.Errorf("worktree %s prüfen: %w", wt.RepoName, err)
	}
	return isDirty, nil
}
