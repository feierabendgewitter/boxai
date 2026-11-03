package createworkspace

import (
	"context"
	"fmt"
	"slices"
)

type rollbackStep struct {
	name string
	undo func(context.Context) error
}

// rollbackStack macht Schritte in umgekehrter Reihenfolge rückgängig.
type rollbackStack struct {
	steps []rollbackStep
}

func (r *rollbackStack) push(name string, undo func(context.Context) error) {
	r.steps = append(r.steps, rollbackStep{name, undo})
}

// insertBeforeLast sorgt dafür, dass der Schritt erst nach dem zuletzt registrierten läuft.
func (r *rollbackStack) insertBeforeLast(name string, undo func(context.Context) error) {
	r.steps = slices.Insert(r.steps, len(r.steps)-1, rollbackStep{name, undo})
}

// run führt alle Schritte aus und sammelt Fehler, statt beim ersten abzubrechen.
func (r *rollbackStack) run(ctx context.Context) []error {
	var errs []error
	for _, step := range slices.Backward(r.steps) {
		if err := step.undo(ctx); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", step.name, err))
		}
	}
	return errs
}
