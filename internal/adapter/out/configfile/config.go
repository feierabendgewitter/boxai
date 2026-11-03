// Package configfile implementiert port.ConfigLoader für boxai.yaml und Secret-Dateien.
package configfile

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/feierabendgewitter/boxai/internal/app/port"
	"github.com/feierabendgewitter/boxai/internal/domain/agent"
)

// Loader implementiert port.ConfigLoader.
type Loader struct {
	home       string
	globalPath string
	getenv     func(string) (string, bool)
	// onSecret erhält jeden aufgelösten Wert, damit der Runner ihn redigieren kann.
	onSecret func(string)
}

// New erzeugt den Loader.
func New(home, globalPath string, getenv func(string) (string, bool), onSecret func(string)) *Loader {
	return &Loader{home: home, globalPath: globalPath, getenv: getenv, onSecret: onSecret}
}

type fileConfig struct {
	Name       string       `yaml:"name"`
	Root       string       `yaml:"root"`
	Agent      string       `yaml:"agent"`
	Profiles   []string     `yaml:"profiles"`
	Image      string       `yaml:"image"`
	Dockerfile string       `yaml:"dockerfile"`
	Branch     string       `yaml:"branch"`
	Base       string       `yaml:"base"`
	Checkout   string       `yaml:"checkout"`
	Repos      []fileRepo   `yaml:"repos"`
	Secrets    secretsField `yaml:"secrets"`
	Network    struct {
		Allow []string `yaml:"allow"`
	} `yaml:"network"`
	Agents map[string]struct {
		Mappings []fileMapping `yaml:"mappings"`
	} `yaml:"agents"`
}

type fileRepo struct {
	Path string `yaml:"path"`
	URL  string `yaml:"url"`
	Base string `yaml:"base"`
}

type fileMapping struct {
	Source string `yaml:"source"`
	Kind   string `yaml:"kind"`
	Target string `yaml:"target"`
	Scope  string `yaml:"scope"`
}

// secretsField akzeptiert `secrets: ./datei` und `secrets: {file: ./datei, mode: env}`.
type secretsField struct {
	File string `yaml:"file"`
	Mode string `yaml:"mode"`
}

// UnmarshalYAML implementiert yaml.Unmarshaler.
func (s *secretsField) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		s.File = node.Value
		return nil
	}
	type plain secretsField
	return node.Decode((*plain)(s))
}

func decodeStrict(data []byte, out any) error {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(out); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

// LoadWorkspaceConfig implementiert port.ConfigLoader.
func (l *Loader) LoadWorkspaceConfig(ctx context.Context, path string) (port.WorkspaceConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return port.WorkspaceConfig{}, err
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return port.WorkspaceConfig{}, err
	}
	return l.parse(ctx, data, filepath.Dir(abs))
}

// LoadGlobalConfig implementiert port.ConfigLoader.
func (l *Loader) LoadGlobalConfig(ctx context.Context) (port.WorkspaceConfig, error) {
	data, err := os.ReadFile(l.globalPath)
	if errors.Is(err, os.ErrNotExist) {
		return port.WorkspaceConfig{}, nil
	}
	if err != nil {
		return port.WorkspaceConfig{}, err
	}
	return l.parse(ctx, data, filepath.Dir(l.globalPath))
}

func (l *Loader) parse(ctx context.Context, data []byte, baseDir string) (port.WorkspaceConfig, error) {
	var f fileConfig
	if err := decodeStrict(data, &f); err != nil {
		return port.WorkspaceConfig{}, err
	}
	cfg := port.WorkspaceConfig{
		Name: f.Name, Agent: f.Agent, Profiles: f.Profiles, Image: f.Image,
		Branch: f.Branch, Base: f.Base, Checkout: f.Checkout,
		SecretsMode: f.Secrets.Mode, NetworkAllow: f.Network.Allow,
		Root:       l.resolve(baseDir, f.Root),
		Dockerfile: l.resolve(baseDir, f.Dockerfile),
		Secrets:    l.resolve(baseDir, f.Secrets.File),
	}
	for _, r := range f.Repos {
		if (r.Path == "") == (r.URL == "") {
			return port.WorkspaceConfig{}, errors.New("jeder Repo-Eintrag braucht genau eines von path und url")
		}
		rc := port.RepoConfig{URL: r.URL, Base: r.Base}
		if r.Path != "" {
			normalized, err := l.NormalizePath(ctx, l.resolve(baseDir, r.Path))
			if err != nil {
				return port.WorkspaceConfig{}, fmt.Errorf("repo %s: %w", r.Path, err)
			}
			rc.Path = normalized
		}
		cfg.Repos = append(cfg.Repos, rc)
	}
	var err error
	if cfg.Mappings, err = parseMappings(f); err != nil {
		return port.WorkspaceConfig{}, err
	}
	return cfg, nil
}

func parseMappings(f fileConfig) (map[agent.Kind][]agent.AssetMapping, error) {
	if len(f.Agents) == 0 {
		return nil, nil
	}
	out := make(map[agent.Kind][]agent.AssetMapping, len(f.Agents))
	for kind, entry := range f.Agents {
		for _, m := range entry.Mappings {
			mapping := agent.AssetMapping{Source: m.Source, Target: m.Target}
			switch m.Kind {
			case "files":
				mapping.Kind = agent.MarkdownFiles
			case "dirs":
				mapping.Kind = agent.Directories
			default:
				return nil, fmt.Errorf("mapping %s: kind muss files oder dirs sein, nicht %q", m.Source, m.Kind)
			}
			switch m.Scope {
			case "", "home":
				mapping.Scope = agent.Home
			case "workspace":
				mapping.Scope = agent.Workspace
			default:
				return nil, fmt.Errorf("mapping %s: scope muss home oder workspace sein, nicht %q", m.Source, m.Scope)
			}
			out[agent.Kind(kind)] = append(out[agent.Kind(kind)], mapping)
		}
	}
	return out, nil
}

// resolve expandiert ~ und macht relative Pfade absolut zum Ordner der Datei.
func (l *Loader) resolve(baseDir, path string) string {
	if path == "" {
		return ""
	}
	if path == "~" {
		return l.home
	}
	if rest, ok := strings.CutPrefix(path, "~/"); ok {
		return filepath.Join(l.home, rest)
	}
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(baseDir, path)
}

// NormalizePath implementiert port.ConfigLoader. Noch fehlende Pfadteile bleiben erhalten,
// der vorhandene Teil wird von Symlinks befreit (unter macOS etwa /tmp → /private/tmp).
func (l *Loader) NormalizePath(_ context.Context, path string) (string, error) {
	abs, err := filepath.Abs(l.resolve("", path))
	if err != nil {
		return "", err
	}
	existing, rest := abs, ""
	for {
		resolved, err := filepath.EvalSymlinks(existing)
		if err == nil {
			return filepath.Join(resolved, rest), nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(existing)
		if parent == existing {
			return abs, nil
		}
		rest = filepath.Join(filepath.Base(existing), rest)
		existing = parent
	}
}
