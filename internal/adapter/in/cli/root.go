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

// Defaults sind Pfade, die der Composition Root aus dem Home ableitet.
type Defaults struct {
	Root      string
	CacheRoot string
	KitRoot   string
}

// Services sind die Inbound-Ports, die das CLI aufruft.
type Services struct {
	Doctor    port.Doctor
	Scan      port.RepoScan
	Create    port.WorkspaceCreator
	Remove    port.WorkspaceRemover
	Lifecycle port.Lifecycle
	Defaults  Defaults
}

// Factory baut die Services, sobald die Flags bekannt sind. Prompter und Notifier
// gehören zum CLI und werden deshalb hineingereicht.
type Factory func(Runtime, port.Prompter, port.Notifier) Services

// ErrUnhealthy signalisiert fehlgeschlagene Doctor-Prüfungen.
var ErrUnhealthy = errors.New("boxai doctor: mindestens eine Pflichtprüfung ist fehlgeschlagen")

// Execute führt das CLI aus und liefert den Exit-Code.
func Execute(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer, factory Factory) int {
	root := newRoot(stdin, stderr, factory)
	root.SetArgs(args)
	root.SetOut(stdout)
	root.SetErr(stderr)
	err := root.ExecuteContext(ctx)
	if err == nil {
		return 0
	}
	var agentExit *port.AgentExit
	if errors.As(err, &agentExit) {
		return agentExit.Code
	}
	fmt.Fprintln(stderr, "Fehler:", err)
	return 1
}

func newRoot(stdin io.Reader, stderr io.Writer, factory Factory) *cobra.Command {
	var rt Runtime
	root := &cobra.Command{
		Use:           "boxai",
		Short:         "Bündelt Git-Repos als Worktrees und startet sie in einer Docker Sandbox",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().BoolVar(&rt.DryRun, "dry-run", false, "geplante git- und sbx-Kommandos nur anzeigen")
	root.PersistentFlags().BoolVar(&rt.Verbose, "verbose", false, "strukturierte Logs ausgeben")
	notifier := &Notifier{Out: stderr}
	prompter := &Prompter{Stdin: stdin}
	services := func() Services { return factory(rt, prompter, notifier) }
	root.AddCommand(
		newUpCmd(services, &rt),
		newScanCmd(services),
		newListCmd(services),
		newAttachCmd(services),
		newStopCmd(services),
		newRmCmd(services),
		newDoctorCmd(services),
		newInitCmd(),
		newAgentHelpCmd(root),
	)
	return root
}
