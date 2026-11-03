// Package hostprobe implementiert port.HostProbe über Subprozesse.
package hostprobe

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/feierabendgewitter/boxai/internal/adapter/out/execx"
	"github.com/feierabendgewitter/boxai/internal/app/port"
)

const probeTimeout = 45 * time.Second

var versionPattern = regexp.MustCompile(`\d+\.\d+(\.\d+)?`)

// Probe implementiert port.HostProbe.
type Probe struct {
	runner   execx.Runner
	lookPath func(string) (string, error)
}

// New erzeugt die Probe. lookPath ist in Tests ersetzbar.
func New(runner execx.Runner, lookPath func(string) (string, error)) *Probe {
	return &Probe{runner: runner, lookPath: lookPath}
}

// Tool implementiert port.HostProbe.
func (p *Probe) Tool(ctx context.Context, name string) port.ToolInfo {
	if _, err := p.lookPath(name); err != nil {
		return port.ToolInfo{}
	}
	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	res, err := p.runner.Run(ctx, execx.Cmd{Name: name, Args: []string{"version"}, ReadOnly: true})
	if name == "git" || name == "docker" {
		res, err = p.runner.Run(ctx, execx.Cmd{Name: name, Args: []string{"--version"}, ReadOnly: true})
	}
	if err != nil {
		// Das Binary existiert, antwortet aber nicht: ein hängendes sbx soll auffallen.
		return port.ToolInfo{Detail: "gefunden, antwortet aber nicht: " + firstLine(err.Error())}
	}
	return port.ToolInfo{Found: true, Version: versionPattern.FindString(res.Stdout)}
}

type diagnoseReport struct {
	Sections []struct {
		Checks []diagnoseCheck `json:"checks"`
	} `json:"sections"`
	Checks []diagnoseCheck `json:"checks"`
}

type diagnoseCheck struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

// SbxAuthenticated liest die Authentication-Prüfung aus `sbx diagnose`.
func (p *Probe) SbxAuthenticated(ctx context.Context) (bool, string) {
	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	// Der Exit-Code ist ungleich 0, sobald irgendeine Prüfung fehlschlägt; maßgeblich ist das JSON.
	res, runErr := p.runner.Run(ctx, execx.Cmd{Name: "sbx", Args: []string{"diagnose", "--format", "json"}, ReadOnly: true})
	var report diagnoseReport
	if err := json.NewDecoder(strings.NewReader(res.Stdout)).Decode(&report); err != nil {
		if runErr != nil {
			return false, firstLine(runErr.Error())
		}
		return false, "sbx diagnose liefert kein JSON"
	}
	checks := report.Checks
	for _, s := range report.Sections {
		checks = append(checks, s.Checks...)
	}
	for _, c := range checks {
		if c.Name == "Authentication" {
			return c.Status == "pass", c.Message
		}
	}
	return false, "sbx diagnose meldet keinen Authentication-Status"
}

type policyCheck struct {
	Allowed *bool `json:"allowed"`
}

// NetworkAllowed nutzt die Auswertung des Daemons. Exit 1 heißt dort „gesperrt“,
// maßgeblich ist deshalb das JSON.
func (p *Probe) NetworkAllowed(ctx context.Context, sandbox, host string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	args := []string{"policy", "check", "network", "--json"}
	if sandbox != "" {
		args = append(args, "--sandbox", sandbox)
	}
	res, runErr := p.runner.Run(ctx, execx.Cmd{Name: "sbx", Args: append(args, host), ReadOnly: true})
	var check policyCheck
	if err := json.NewDecoder(strings.NewReader(res.Stdout)).Decode(&check); err != nil || check.Allowed == nil {
		if runErr != nil {
			return false, errors.New(firstLine(runErr.Error()))
		}
		return false, errors.New("sbx policy check liefert kein gültiges JSON")
	}
	return *check.Allowed, nil
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
}
