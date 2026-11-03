package createworkspace

import (
	"slices"

	"github.com/feierabendgewitter/boxai/internal/app/port"
)

// settings ist die aufgelöste Konfiguration eines Laufs.
type settings struct {
	port.WorkspaceConfig
	Dir       string
	Depth     int
	RepoArgs  []string
	NoDocker  bool
	KeepOnErr bool
	NoAttach  bool
	DryRun    bool
	CacheRoot string
	KitRoot   string
}

// merge wendet die Reihenfolge Defaults < globale Konfiguration < boxai.yaml < Flags an.
func merge(req port.CreateRequest, global, file port.WorkspaceConfig) settings {
	flags := port.WorkspaceConfig{
		Name: req.Name, Root: req.Root, Agent: req.Agent, Profiles: req.Profiles,
		Image: req.Image, Dockerfile: req.Dockerfile, Branch: req.Branch, Base: req.Base,
		Checkout: req.Checkout, Secrets: req.Secrets,
	}
	defaults := port.WorkspaceConfig{Root: req.DefaultRoot}

	merged := defaults
	for _, layer := range []port.WorkspaceConfig{global, file, flags} {
		merged = overlay(merged, layer)
	}
	return settings{
		WorkspaceConfig: merged,
		Dir:             req.Dir,
		Depth:           req.Depth,
		RepoArgs:        req.Repos,
		NoDocker:        req.NoDocker,
		KeepOnErr:       req.KeepOnErr,
		NoAttach:        req.NoAttach,
		DryRun:          req.DryRun,
		CacheRoot:       req.CacheRoot,
		KitRoot:         req.KitRoot,
	}
}

func overlay(base, top port.WorkspaceConfig) port.WorkspaceConfig {
	str := func(dst *string, v string) {
		if v != "" {
			*dst = v
		}
	}
	str(&base.Name, top.Name)
	str(&base.Root, top.Root)
	str(&base.Agent, top.Agent)
	str(&base.Image, top.Image)
	str(&base.Dockerfile, top.Dockerfile)
	str(&base.Branch, top.Branch)
	str(&base.Base, top.Base)
	str(&base.Checkout, top.Checkout)
	str(&base.Secrets, top.Secrets)
	str(&base.SecretsMode, top.SecretsMode)
	if len(top.Profiles) > 0 {
		base.Profiles = slices.Clone(top.Profiles)
	}
	if len(top.Repos) > 0 {
		base.Repos = slices.Clone(top.Repos)
	}
	// Netzwerk-Freigaben addieren sich über alle Ebenen.
	base.NetworkAllow = append(slices.Clone(base.NetworkAllow), top.NetworkAllow...)
	return base
}
