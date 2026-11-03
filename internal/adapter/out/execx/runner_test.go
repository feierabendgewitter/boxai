package execx

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestDryRunSkipsMutationsAndRunsReadOnly(t *testing.T) {
	var plan bytes.Buffer
	e := &Exec{DryRun: true, PlanOut: &plan}
	if _, err := e.Run(context.Background(), Cmd{Name: "false", Args: []string{"a b"}}); err != nil {
		t.Fatalf("Mutation darf im Dry-Run nicht laufen: %v", err)
	}
	if got := plan.String(); got != "false 'a b'\n" {
		t.Fatalf("Plan = %q", got)
	}
	res, err := e.Run(context.Background(), Cmd{Name: "echo", Args: []string{"x"}, ReadOnly: true})
	if err != nil || res.Stdout != "x\n" {
		t.Fatalf("ReadOnly: %q, %v", res.Stdout, err)
	}
}

func TestSecretsNeverAppearInOutputLogsOrErrors(t *testing.T) {
	const secret = "s3cr3t-value"
	var logs, plan bytes.Buffer
	log := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	e := &Exec{Log: log, PlanOut: &plan}
	e.Redact(secret)

	res, err := e.Run(context.Background(), Cmd{Name: "sh", Args: []string{"-c", "cat; echo " + secret + " >&2; exit 3"}, Stdin: secret})
	var exit *ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 3 {
		t.Fatalf("Fehler = %v", err)
	}
	all := res.Stdout + res.Stderr + err.Error() + logs.String()
	if strings.Contains(all, secret) {
		t.Fatalf("Secret ausgegeben: %s", all)
	}
}

func TestEnvValuesAreHiddenAndPassed(t *testing.T) {
	c := Cmd{Name: "sh", Args: []string{"-c", "printf %s \"$BOXAI_T\""}, Env: []string{"BOXAI_T=geheim"}}
	if got := Format(c); strings.Contains(got, "geheim") {
		t.Fatalf("Format zeigt Wert: %s", got)
	}
	res, err := (&Exec{}).Run(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	if res.Stdout != "[REDACTED]" && res.Stdout != "geheim" {
		t.Fatalf("Stdout = %q", res.Stdout)
	}
}

func TestTimeout(t *testing.T) {
	e := &Exec{Timeout: 50 * time.Millisecond}
	_, err := e.Run(context.Background(), Cmd{Name: "sleep", Args: []string{"5"}})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Fehler = %v", err)
	}
}

func TestFormatQuotes(t *testing.T) {
	got := Format(Cmd{Name: "sbx", Args: []string{"policy", "*.a.b"}, Stdin: "x"})
	if got != "sbx policy '*.a.b' < [stdin]" {
		t.Fatal(got)
	}
}
