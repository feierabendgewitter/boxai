package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/charmbracelet/huh"

	"github.com/feierabendgewitter/boxai/internal/domain/agent"
	"github.com/feierabendgewitter/boxai/internal/domain/workspace"
)

// ErrNotInteractive erklärt, welche Flags einen fehlenden Prompt ersetzen.
var ErrNotInteractive = errors.New("keine interaktive Eingabe möglich; bitte --name, --repo und --agent setzen oder --config nutzen")

// Notifier schreibt Meldungen an den Nutzer.
type Notifier struct {
	Out io.Writer
}

// Info implementiert port.Notifier.
func (n *Notifier) Info(msg string) { fmt.Fprintln(n.Out, msg) }

// Warn implementiert port.Notifier.
func (n *Notifier) Warn(msg string) { fmt.Fprintln(n.Out, "Warnung:", msg) }

// Prompter implementiert port.Prompter mit huh.
type Prompter struct {
	Stdin io.Reader
}

func (p *Prompter) interactive() bool {
	file, ok := p.Stdin.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

// SelectRepos implementiert port.Prompter.
func (p *Prompter) SelectRepos(ctx context.Context, repos []workspace.RepoRef) ([]workspace.RepoRef, error) {
	if !p.interactive() {
		return nil, ErrNotInteractive
	}
	options := make([]huh.Option[int], len(repos))
	for i, r := range repos {
		options[i] = huh.NewOption(RepoLabel(r), i)
	}
	var picked []int
	field := huh.NewMultiSelect[int]().
		Title("Welche Repos gehören in den Workspace?").
		Options(options...).
		Value(&picked).
		Validate(func(v []int) error {
			if len(v) == 0 {
				return errors.New("mindestens ein Repo wählen")
			}
			return nil
		})
	if err := huh.NewForm(huh.NewGroup(field)).RunWithContext(ctx); err != nil {
		return nil, err
	}
	chosen := make([]workspace.RepoRef, len(picked))
	for i, idx := range picked {
		chosen[i] = repos[idx]
	}
	return chosen, nil
}

// RepoLabel zeigt Name, Pfad, Branch und Dirty-Status.
func RepoLabel(r workspace.RepoRef) string {
	status := ""
	if r.Dirty {
		status = " *geändert*"
	}
	return fmt.Sprintf("%s [%s]%s  %s", r.Name, r.CurrentBranch, status, r.Path)
}

// SelectAgent implementiert port.Prompter.
func (p *Prompter) SelectAgent(ctx context.Context, kinds []agent.Kind) (agent.Kind, error) {
	if !p.interactive() {
		return "", ErrNotInteractive
	}
	options := make([]huh.Option[agent.Kind], len(kinds))
	for i, k := range kinds {
		options[i] = huh.NewOption(string(k), k)
	}
	var kind agent.Kind
	field := huh.NewSelect[agent.Kind]().Title("Welcher Agent?").Options(options...).Value(&kind)
	if err := huh.NewForm(huh.NewGroup(field)).RunWithContext(ctx); err != nil {
		return "", err
	}
	return kind, nil
}

// InputName implementiert port.Prompter.
func (p *Prompter) InputName(ctx context.Context) (string, error) {
	if !p.interactive() {
		return "", ErrNotInteractive
	}
	var name string
	field := huh.NewInput().Title("Name des Workspaces").Value(&name).Validate(workspace.ValidateName)
	if err := huh.NewForm(huh.NewGroup(field)).RunWithContext(ctx); err != nil {
		return "", err
	}
	return name, nil
}
