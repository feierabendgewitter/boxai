// Package execx ist der gemeinsame Subprozess-Runner aller Outbound-Adapter:
// Dry-Run, Logging und Redaction von Secret-Werten.
package execx

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// DefaultTimeout begrenzt nicht-interaktive Kommandos. sbx kann ohne Ausgabe hängen.
const DefaultTimeout = 20 * time.Minute

// Cmd beschreibt einen Subprozess.
type Cmd struct {
	Name string
	Args []string
	Dir  string
	// Stdin wird nie geloggt; darüber laufen Secret-Werte.
	Stdin string
	// Env ergänzt die Umgebung des Kindprozesses um NAME=WERT. Werte werden nie geloggt.
	Env []string
	// ReadOnly-Kommandos laufen auch im Dry-Run, weil sie nur Zustand lesen.
	ReadOnly bool
	// Interactive reicht stdin, stdout und stderr durch und setzt kein Zeitlimit.
	Interactive bool
	// Secrets sind Werte, die aus Ausgaben und Fehlern entfernt werden.
	Secrets []string
}

// Result enthält die redigierte Ausgabe.
type Result struct {
	Stdout string
	Stderr string
}

// Runner führt Kommandos aus. Adapter hängen nur von diesem Interface ab.
type Runner interface {
	Run(ctx context.Context, cmd Cmd) (Result, error)
}

// ExitError trägt den Exit-Code eines fehlgeschlagenen Kommandos.
type ExitError struct {
	Command string
	Code    int
	Stderr  string
}

func (e *ExitError) Error() string {
	msg := strings.TrimSpace(e.Stderr)
	if msg == "" {
		return fmt.Sprintf("%s: exit %d", e.Command, e.Code)
	}
	return fmt.Sprintf("%s: exit %d: %s", e.Command, e.Code, msg)
}

// ExitCode liefert den Exit-Code des Kommandos.
func (e *ExitError) ExitCode() int { return e.Code }

// Exec führt Kommandos wirklich aus.
type Exec struct {
	DryRun  bool
	Log     *slog.Logger
	PlanOut io.Writer // erhält im Dry-Run die geplanten Kommandos
	Timeout time.Duration

	mu      sync.Mutex
	secrets []string
}

// Redact merkt sich einen Wert, damit er aus allen späteren Ausgaben entfernt wird.
func (e *Exec) Redact(values ...string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, v := range values {
		if v != "" {
			e.secrets = append(e.secrets, v)
		}
	}
}

func (e *Exec) scrub(s string, extra []string) string {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, v := range append(append([]string{}, e.secrets...), extra...) {
		if v != "" {
			s = strings.ReplaceAll(s, v, "[REDACTED]")
		}
	}
	return s
}

func (e *Exec) logger() *slog.Logger {
	if e.Log != nil {
		return e.Log
	}
	return slog.New(slog.DiscardHandler)
}

// Run implementiert Runner.
func (e *Exec) Run(ctx context.Context, c Cmd) (Result, error) {
	display := e.scrub(Format(c), c.Secrets)
	if e.DryRun && !c.ReadOnly {
		if e.PlanOut != nil {
			fmt.Fprintln(e.PlanOut, display)
		}
		return Result{}, nil
	}
	e.logger().Debug("exec", "cmd", display)

	if !c.Interactive {
		timeout := e.Timeout
		if timeout == 0 {
			timeout = DefaultTimeout
		}
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	proc := exec.CommandContext(ctx, c.Name, c.Args...)
	proc.Dir = c.Dir
	if len(c.Env) > 0 {
		proc.Env = append(os.Environ(), c.Env...)
	}

	var stdout, stderr bytes.Buffer
	if c.Interactive {
		proc.Stdin, proc.Stdout, proc.Stderr = os.Stdin, os.Stdout, os.Stderr
	} else {
		proc.Stdin = strings.NewReader(c.Stdin)
		proc.Stdout, proc.Stderr = &stdout, &stderr
	}
	err := proc.Run()
	res := Result{Stdout: e.scrub(stdout.String(), c.Secrets), Stderr: e.scrub(stderr.String(), c.Secrets)}
	e.logger().Debug("exec done", "cmd", display, "err", err != nil)
	if err == nil {
		return res, nil
	}
	if ctx.Err() != nil {
		return res, fmt.Errorf("%s: %w", display, ctx.Err())
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return res, &ExitError{Command: display, Code: exitErr.ExitCode(), Stderr: res.Stderr}
	}
	return res, fmt.Errorf("%s: %s", display, e.scrub(err.Error(), c.Secrets))
}

// Format rendert ein Kommando shell-lesbar. Stdin und Env-Werte erscheinen nur als Hinweis.
func Format(c Cmd) string {
	parts := make([]string, 0, len(c.Args)+3)
	for _, kv := range c.Env {
		name, _, _ := strings.Cut(kv, "=")
		parts = append(parts, name+"=[REDACTED]")
	}
	parts = append(parts, quote(c.Name))
	for _, a := range c.Args {
		parts = append(parts, quote(a))
	}
	if c.Stdin != "" {
		parts = append(parts, "< [stdin]")
	}
	return strings.Join(parts, " ")
}

func quote(s string) string {
	if s != "" && !strings.ContainsAny(s, " \t\n'\"$`\\*?&;|<>()#") {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
