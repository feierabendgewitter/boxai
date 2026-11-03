// Package execxtest stellt einen aufzeichnenden Fake-Runner für Adapter-Tests bereit.
package execxtest

import (
	"context"
	"strings"

	"github.com/feierabendgewitter/boxai/internal/adapter/out/execx"
)

// Recorder zeichnet Kommandos auf und liefert vorbereitete Antworten.
type Recorder struct {
	Cmds []execx.Cmd
	// Responses bildet Kommando-Präfixe (z. B. "sbx ls") auf Antworten ab.
	Responses map[string]Response
}

// Response ist eine vorbereitete Antwort.
type Response struct {
	Stdout string
	Err    error
}

// Run implementiert execx.Runner.
func (r *Recorder) Run(_ context.Context, c execx.Cmd) (execx.Result, error) {
	r.Cmds = append(r.Cmds, c)
	line := c.Name + " " + strings.Join(c.Args, " ")
	best := ""
	for prefix := range r.Responses {
		if strings.HasPrefix(line, prefix) && len(prefix) > len(best) {
			best = prefix
		}
	}
	if best == "" {
		return execx.Result{}, nil
	}
	resp := r.Responses[best]
	return execx.Result{Stdout: resp.Stdout}, resp.Err
}

// Lines liefert die aufgezeichneten Kommandos als Text.
func (r *Recorder) Lines() []string {
	lines := make([]string, 0, len(r.Cmds))
	for _, c := range r.Cmds {
		lines = append(lines, execx.Format(c))
	}
	return lines
}
