package secret

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"testing"
)

const value = "super-secret-value"

func spec() SecretSpec {
	return SecretSpec{Name: "MAVEN_TOKEN", Hosts: []string{"nexus.firma.de"}, Value: value}
}

func TestValueNeverRendered(t *testing.T) {
	s := spec()
	var logs bytes.Buffer
	slog.New(slog.NewTextHandler(&logs, nil)).Info("x", "secret", s)
	outputs := []string{fmt.Sprint(s), fmt.Sprintf("%v %+v %#v %s", s, s, s, s), logs.String()}
	for _, out := range outputs {
		if strings.Contains(out, value) {
			t.Errorf("Wert sichtbar: %s", out)
		}
	}
	if _, err := json.Marshal(s); err == nil {
		t.Error("Serialisierung müsste scheitern")
	}
	if _, err := json.Marshal([]SecretSpec{s}); err == nil {
		t.Error("Serialisierung einer Liste müsste scheitern")
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name string
		edit func(*SecretSpec)
		mode Mode
		want string
	}{
		{"gültig", func(*SecretSpec) {}, ModeProxy, ""},
		{"ohne Hosts", func(s *SecretSpec) { s.Hosts = nil }, ModeProxy, "hosts fehlt"},
		{"Env ohne Hosts", func(s *SecretSpec) { s.Hosts = nil }, ModeEnv, ""},
		{"Name", func(s *SecretSpec) { s.Name = "1-a" }, ModeProxy, "Secret-Name"},
		{"Host", func(s *SecretSpec) { s.Hosts = []string{"https://x/y"} }, ModeProxy, "Host"},
		{"Wert", func(s *SecretSpec) { s.Value = "" }, ModeProxy, "leer"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := spec()
			tt.edit(&s)
			err := s.Validate(tt.mode)
			if tt.want == "" && err != nil {
				t.Fatal(err)
			}
			if tt.want != "" && (err == nil || !strings.Contains(err.Error(), tt.want)) {
				t.Fatalf("Fehler = %v", err)
			}
			if err != nil && strings.Contains(err.Error(), value) {
				t.Fatal("Fehlertext enthält den Wert")
			}
		})
	}
}

func TestValidateAll(t *testing.T) {
	if err := ValidateAll([]SecretSpec{spec()}, ModeProxy); err != nil {
		t.Fatal(err)
	}
	if err := ValidateAll([]SecretSpec{spec(), spec()}, ModeProxy); err == nil {
		t.Fatal("Duplikat akzeptiert")
	}
	if err := ValidateAll([]SecretSpec{{Name: "X"}}, ModeProxy); err == nil {
		t.Fatal("ungültiges Secret akzeptiert")
	}
}

func TestParseRef(t *testing.T) {
	tests := []struct {
		raw   string
		bare  bool
		want  Ref
		isErr bool
	}{
		{raw: "xyz", want: Ref{Literal, "xyz"}},
		{raw: "${env:A_B}", want: Ref{EnvRef, "A_B"}},
		{raw: "${file:~/.secrets/npm}", want: Ref{FileRef, "~/.secrets/npm"}},
		{raw: "${NEXUS}", bare: true, want: Ref{EnvRef, "NEXUS"}},
		{raw: "${NEXUS}", isErr: true},
		{raw: "${vault:x}", isErr: true},
		{raw: "${env:a-b}", isErr: true},
		{raw: "${file:}", isErr: true},
	}
	for _, tt := range tests {
		got, err := ParseRef(tt.raw, tt.bare)
		if (err != nil) != tt.isErr || got != tt.want {
			t.Errorf("%s: %+v, %v", tt.raw, got, err)
		}
	}
}

func TestParseMode(t *testing.T) {
	for raw, want := range map[string]Mode{"": ModeProxy, "proxy": ModeProxy, "env": ModeEnv} {
		if got, err := ParseMode(raw); err != nil || got != want {
			t.Errorf("%q: %v, %v", raw, got, err)
		}
	}
	if _, err := ParseMode("plain"); err == nil {
		t.Error("unbekannter Modus akzeptiert")
	}
}
