// Package scanrepos implementiert den Use Case `boxai scan`.
package scanrepos

import (
	"context"
	"fmt"

	"github.com/feierabendgewitter/boxai/internal/app/port"
	"github.com/feierabendgewitter/boxai/internal/domain/workspace"
)

// DefaultDepth ist die Scan-Tiefe ohne `--depth`.
const DefaultDepth = 3

// Service implementiert port.RepoScan.
type Service struct {
	scanner port.RepoScanner
	git     port.GitRepository
}

// New erzeugt den Use Case.
func New(scanner port.RepoScanner, git port.GitRepository) *Service {
	return &Service{scanner: scanner, git: git}
}

// Scan implementiert port.RepoScan.
func (s *Service) Scan(ctx context.Context, req port.ScanRequest) ([]workspace.RepoRef, error) {
	depth := req.Depth
	if depth <= 0 {
		depth = DefaultDepth
	}
	repos, err := s.scanner.Scan(ctx, req.Dir, depth)
	if err != nil {
		return nil, fmt.Errorf("repo-scan in %s: %w", req.Dir, err)
	}
	for i := range repos {
		if err := s.enrich(ctx, &repos[i]); err != nil {
			return nil, err
		}
	}
	return repos, nil
}

func (s *Service) enrich(ctx context.Context, r *workspace.RepoRef) error {
	var err error
	if r.CurrentBranch, err = s.git.CurrentBranch(ctx, r.Path); err != nil {
		return fmt.Errorf("repo %s: %w", r.Name, err)
	}
	if r.DefaultBranch, err = s.git.DefaultBranch(ctx, r.Path); err != nil {
		return fmt.Errorf("repo %s: %w", r.Name, err)
	}
	if r.Dirty, err = s.git.IsDirty(ctx, r.Path); err != nil {
		return fmt.Errorf("repo %s: %w", r.Name, err)
	}
	return nil
}
