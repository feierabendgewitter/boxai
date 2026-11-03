package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/feierabendgewitter/boxai/internal/app/port"
)

type stubDoctor struct{ report port.DoctorReport }

func (s stubDoctor) Run(context.Context, port.DoctorRequest) port.DoctorReport { return s.report }

func TestDoctorPrintsChecksAndSetsExitCode(t *testing.T) {
	report := port.DoctorReport{Checks: []port.DoctorCheck{
		{Name: "git", OK: true, Required: true, Detail: "2.54.0"},
		{Name: "sbx", OK: false, Required: true, Detail: "nicht gefunden"},
	}}
	var out, errOut bytes.Buffer
	code := Execute(context.Background(), []string{"doctor"}, &out, &errOut, func(Runtime) Services {
		return Services{Doctor: stubDoctor{report}}
	})
	if code != 1 {
		t.Fatalf("Exit-Code = %d", code)
	}
	if !strings.Contains(out.String(), "✗ sbx") || !strings.Contains(out.String(), "✓ git") {
		t.Fatal(out.String())
	}
}
