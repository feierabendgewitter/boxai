// Package cli ist der Driving Adapter: Es parst Flags, ruft einen Inbound-Port auf
// und formatiert das Ergebnis.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/feierabendgewitter/boxai/internal/app/port"
)

// Runtime enthält die globalen Optionen, mit denen der Composition Root Adapter baut.
type Runtime struct {
	DryRun  bool
	Verbose bool
}

// Services sind die Inbound-Ports, die das CLI aufruft.
type Services struct {
	Doctor port.Doctor
}

// Factory baut die Services zur Laufzeit, sobald die Flags bekannt sind.
type Factory func(Runtime) Services

// ExitCoder wird von Fehlern implementiert, deren Exit-Code boxai übernimmt.
type ExitCoder interface{ ExitCode() int }

// ErrUnhealthy signalisiert fehlgeschlagene Doctor-Prüfungen.
var ErrUnhealthy = errors.New("boxai doctor: mindestens eine Pflichtprüfung ist fehlgeschlagen")

// Execute führt das CLI aus und liefert den Exit-Code.
func Execute(ctx context.Context, args []string, stdout, stderr io.Writer, factory Factory) int {
	root := newRoot(factory)
	root.SetArgs(args)
	root.SetOut(stdout)
	root.SetErr(stderr)
	err := root.ExecuteContext(ctx)
	if err == nil {
		return 0
	}
	var coder ExitCoder
	if errors.As(err, &coder) {
		return coder.ExitCode()
	}
	fmt.Fprintln(stderr, "Fehler:", err)
	return 1
}

func newRoot(factory Factory) *cobra.Command {
	var rt Runtime
	root := &cobra.Command{
		Use:           "boxai",
		Short:         "Bündelt Git-Repos als Worktrees und startet sie in einer Docker Sandbox",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().BoolVar(&rt.DryRun, "dry-run", false, "geplante git- und sbx-Kommandos nur anzeigen")
	root.PersistentFlags().BoolVar(&rt.Verbose, "verbose", false, "strukturierte Logs ausgeben")
	services := func() Services { return factory(rt) }
	root.AddCommand(newDoctorCmd(services))
	return root
}
