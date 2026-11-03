// Package sbxcli implementiert port.SandboxRuntime und port.SecretStore über das sbx-Binary.
package sbxcli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/feierabendgewitter/boxai/internal/adapter/out/execx"
	"github.com/feierabendgewitter/boxai/internal/app/port"
	"github.com/feierabendgewitter/boxai/internal/domain/profile"
	"github.com/feierabendgewitter/boxai/internal/domain/sandbox"
	"github.com/feierabendgewitter/boxai/internal/domain/secret"
	"github.com/feierabendgewitter/boxai/internal/domain/workspace"
)

const kitSourceRepo = "docker.io/library/sbx-kit-src"

var kitHash = regexp.MustCompile(`^[0-9a-f]{12}$`)

// CLI implementiert beide Ports.
type CLI struct {
	runner execx.Runner
}

// New erzeugt den Adapter.
func New(runner execx.Runner) *CLI { return &CLI{runner: runner} }

// Create erzeugt die Sandbox. `--skills off` ist nötig, weil sbx sonst einen read-only
// Skills-Store über das Ziel der Kit-Dateien legt.
func (c *CLI) Create(ctx context.Context, spec sandbox.SandboxSpec) error {
	args := []string{"create", "--no-input", "--no-progress", "--name", spec.Name, "--skills", "off"}
	if spec.KitDir != "" {
		args = append(args, "--kit", spec.KitDir)
	}
	if spec.Template != "" {
		args = append(args, "--template", spec.Template)
	}
	if spec.LocalTemplate {
		args = append(args, "--pull", "never")
	}
	cmd := execx.Cmd{Name: "sbx"}
	for _, s := range spec.EnvSecrets {
		// `-e NAME` übernimmt den Wert aus der Umgebung und hält ihn aus der Prozessliste.
		args = append(args, "--env", s.Name)
		cmd.Env = append(cmd.Env, s.Name+"="+s.Value)
		cmd.Secrets = append(cmd.Secrets, s.Value)
	}
	args = append(args, spec.Agent, spec.Workspace)
	for _, m := range spec.Mounts {
		mount := m.Path
		if m.ReadOnly {
			mount += ":ro"
		}
		args = append(args, mount)
	}
	cmd.Args = args
	_, err := c.runner.Run(ctx, cmd)
	return err
}

// Attach öffnet die Agent-Session. Lokal übernimmt `sbx run` das, `sbx attach` ist nur --cloud.
func (c *CLI) Attach(ctx context.Context, name string) error {
	_, err := c.runner.Run(ctx, execx.Cmd{Name: "sbx", Args: []string{"run", "--name", name}, Interactive: true})
	var exit *execx.ExitError
	if errors.As(err, &exit) {
		return &port.AgentExit{Code: exit.ExitCode()}
	}
	return err
}

// Stop implementiert port.SandboxRuntime.
func (c *CLI) Stop(ctx context.Context, name string) error {
	_, err := c.runner.Run(ctx, execx.Cmd{Name: "sbx", Args: []string{"stop", name}})
	return err
}

// Remove entfernt die Sandbox; eine fehlende Sandbox gilt als entfernt, damit Rollback
// und `boxai rm` wiederholbar bleiben.
func (c *CLI) Remove(ctx context.Context, name string) error {
	_, err := c.runner.Run(ctx, execx.Cmd{Name: "sbx", Args: []string{"rm", "--force", name}})
	if err := ignoreNotFound(err); err != nil {
		return err
	}
	return c.removeKitImages(ctx, name)
}

// removeKitImages entfernt die Images, die sbx pro Mixin als `sbx-kit-src:<kit>-<hash>`
// anlegt und bei `sbx rm` stehen lässt. Der Kit heißt wie die Sandbox; der genaue Hash-Vergleich
// schützt boxai-foo-bar davor, beim Entfernen von boxai-foo mitzugehen.
func (c *CLI) removeKitImages(ctx context.Context, name string) error {
	res, err := c.runner.Run(ctx, execx.Cmd{Name: "sbx", Args: []string{"template", "ls"}, ReadOnly: true})
	if err != nil {
		return err
	}
	for line := range strings.SplitSeq(res.Stdout, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		kitImage := fields[0] == kitSourceRepo && isKitTag(fields[1], name)
		// `boxai up --dockerfile` legt das Image pro Workspace an; es gehört zum Workspace.
		customImage := fields[0] == customRepo(name)
		if !kitImage && !customImage {
			continue
		}
		ref := fields[0] + ":" + fields[1]
		if _, err := c.runner.Run(ctx, execx.Cmd{Name: "sbx", Args: []string{"template", "rm", "--force", ref}}); err != nil {
			return err
		}
	}
	return nil
}

func customRepo(sandboxName string) string {
	return "docker.io/library/boxai-custom-" + strings.TrimPrefix(sandboxName, workspace.SandboxPrefix)
}

func isKitTag(tag, kit string) bool {
	hash, ok := strings.CutPrefix(tag, kit+"-")
	return ok && kitHash.MatchString(hash)
}

func ignoreNotFound(err error) error {
	var exit *execx.ExitError
	if errors.As(err, &exit) && strings.Contains(exit.Stderr, "not found") {
		return nil
	}
	return err
}

type listEntry struct {
	Name   string `json:"name"`
	Agent  string `json:"agent"`
	Status string `json:"status"`
}

// List implementiert port.SandboxRuntime.
func (c *CLI) List(ctx context.Context) ([]sandbox.Info, error) {
	res, err := c.runner.Run(ctx, execx.Cmd{Name: "sbx", Args: []string{"ls", "--format", "json"}, ReadOnly: true})
	if err != nil {
		return nil, err
	}
	var entries []listEntry
	if err := json.NewDecoder(strings.NewReader(res.Stdout)).Decode(&entries); err != nil {
		return nil, fmt.Errorf("sbx ls liefert unerwartetes JSON: %w", err)
	}
	infos := make([]sandbox.Info, len(entries))
	for i, e := range entries {
		infos[i] = sandbox.Info(e)
	}
	return infos, nil
}

// AllowNetwork fügt Regeln nur für diese Sandbox hinzu; die globale Policy bleibt unberührt.
func (c *CLI) AllowNetwork(ctx context.Context, name string, rules []profile.NetworkRule) error {
	if len(rules) == 0 {
		return nil
	}
	hosts := make([]string, len(rules))
	for i, r := range rules {
		hosts[i] = r.Host
	}
	_, err := c.runner.Run(ctx, execx.Cmd{
		Name: "sbx",
		Args: []string{"policy", "allow", "network", "--sandbox", name, strings.Join(hosts, ",")},
	})
	return err
}

// Put implementiert port.SecretStore; der Wert läuft ausschließlich über stdin.
func (c *CLI) Put(ctx context.Context, sandboxName string, s secret.SecretSpec) error {
	args := []string{"secret", "set-custom", "--sandbox", sandboxName, "--env", s.Name, "--value-stdin"}
	for _, h := range s.Hosts {
		args = append(args, "--host", h)
	}
	_, err := c.runner.Run(ctx, execx.Cmd{Name: "sbx", Args: args, Stdin: s.Value, Secrets: []string{s.Value}})
	return err
}

type secretList struct {
	CustomSecrets []struct {
		Scope       string `json:"scope"`
		Placeholder string `json:"placeholder"`
	} `json:"custom_secrets"`
}

// DeleteAll entfernt nur Custom-Secrets dieser Sandbox. Die JSON-Ausgabe enthält teilmaskierte
// Werte; sie wird deshalb ausschließlich auf Scope und Platzhalter gelesen.
func (c *CLI) DeleteAll(ctx context.Context, sandboxName string) error {
	res, err := c.runner.Run(ctx, execx.Cmd{
		Name: "sbx", Args: []string{"secret", "ls", "--sandbox", sandboxName, "--json"}, ReadOnly: true,
	})
	if err != nil {
		return ignoreNotFound(err)
	}
	var list secretList
	if err := json.NewDecoder(strings.NewReader(res.Stdout)).Decode(&list); err != nil {
		return errors.New("sbx secret ls liefert unerwartetes JSON")
	}
	for _, s := range list.CustomSecrets {
		if s.Scope != sandboxName {
			continue
		}
		// Die positionale Form endet mit Exit 0, ohne etwas zu löschen; nur --placeholder wirkt.
		_, err := c.runner.Run(ctx, execx.Cmd{
			Name: "sbx", Args: []string{"secret", "rm", "--placeholder", s.Placeholder, "--sandbox", sandboxName, "--force"},
		})
		if ignoreNotFound(err) != nil {
			return err
		}
	}
	return nil
}
