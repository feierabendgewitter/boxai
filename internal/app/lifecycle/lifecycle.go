// Package lifecycle implementiert attach, stop und list.
package lifecycle

import (
	"context"
	"fmt"

	"github.com/feierabendgewitter/boxai/internal/app/port"
)

// StatusMissing steht in `boxai list` für eine Sandbox, die es nicht mehr gibt.
const StatusMissing = "fehlt"

// Service implementiert port.Lifecycle.
type Service struct {
	runtime port.SandboxRuntime
	state   port.StateStore
}

// New erzeugt den Use Case.
func New(runtime port.SandboxRuntime, state port.StateStore) *Service {
	return &Service{runtime: runtime, state: state}
}

// Attach startet den Agent des Workspaces.
func (s *Service) Attach(ctx context.Context, name string) error {
	st, err := s.state.Load(ctx, name)
	if err != nil {
		return err
	}
	return s.runtime.Attach(ctx, st.SandboxName)
}

// Stop hält die Sandbox des Workspaces an.
func (s *Service) Stop(ctx context.Context, name string) error {
	st, err := s.state.Load(ctx, name)
	if err != nil {
		return err
	}
	return s.runtime.Stop(ctx, st.SandboxName)
}

// List verbindet gespeicherte Workspaces mit dem Laufzeitstatus.
func (s *Service) List(ctx context.Context) ([]port.WorkspaceInfo, error) {
	states, err := s.state.List(ctx)
	if err != nil {
		return nil, err
	}
	sandboxes, err := s.runtime.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("sandboxes auflisten: %w", err)
	}
	status := make(map[string]string, len(sandboxes))
	for _, sb := range sandboxes {
		status[sb.Name] = sb.Status
	}
	infos := make([]port.WorkspaceInfo, 0, len(states))
	for _, st := range states {
		info := port.WorkspaceInfo{State: st, SandboxStatus: StatusMissing}
		if s, ok := status[st.SandboxName]; ok {
			info.SandboxStatus = s
		}
		for _, wt := range st.Worktrees {
			info.Repos = append(info.Repos, wt.RepoName)
		}
		infos = append(infos, info)
	}
	return infos, nil
}
