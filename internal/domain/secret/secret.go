// Package secret enthält das Secret-Modell mit Validierung und Redaction.
package secret

import (
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
)

// Redacted ersetzt Secret-Werte in jeder Ausgabe.
const Redacted = "[REDACTED]"

var envName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// Mode legt fest, wie Secrets in die Sandbox gelangen.
type Mode string

// Secret-Modi.
const (
	// ModeProxy hinterlegt Werte im sbx-Secret-Store; die Sandbox sieht nur Platzhalter.
	ModeProxy Mode = "proxy"
	// ModeEnv übergibt echte Werte als Umgebung, etwa für AWS-SigV4. Opt-in mit Warnung.
	ModeEnv Mode = "env"
)

// ParseMode prüft einen Modus; leer bedeutet ModeProxy.
func ParseMode(raw string) (Mode, error) {
	switch Mode(raw) {
	case "", ModeProxy:
		return ModeProxy, nil
	case ModeEnv:
		return ModeEnv, nil
	}
	return "", fmt.Errorf("unbekannter secrets.mode %q (erlaubt: proxy, env)", raw)
}

// SecretSpec ist ein aufgelöstes Secret. Der Wert verlässt den Prozess nur über
// den SecretStore; alle Formatierungen und Serialisierungen maskieren ihn.
type SecretSpec struct {
	Name  string
	Hosts []string
	Value string
}

// String implementiert fmt.Stringer ohne den Wert.
func (s SecretSpec) String() string {
	return fmt.Sprintf("%s(hosts=%s, value=%s)", s.Name, strings.Join(s.Hosts, ","), Redacted)
}

// GoString maskiert %#v.
func (s SecretSpec) GoString() string { return s.String() }

// LogValue maskiert slog.
func (s SecretSpec) LogValue() slog.Value { return slog.StringValue(s.String()) }

// MarshalJSON verhindert, dass ein Wert versehentlich serialisiert wird.
func (s SecretSpec) MarshalJSON() ([]byte, error) {
	return nil, errors.New("secret.SecretSpec darf nicht serialisiert werden")
}

// Validate prüft ein Secret. Im Proxy-Modus sind Hosts Pflicht, weil der Proxy
// sonst nicht weiß, wo er den Wert einsetzen soll.
func (s SecretSpec) Validate(mode Mode) error {
	if !envName.MatchString(s.Name) {
		return fmt.Errorf("ungültiger Secret-Name %q", s.Name)
	}
	if mode == ModeProxy && len(s.Hosts) == 0 {
		return fmt.Errorf("secret %s: hosts fehlt; ohne Hosts weiß der Proxy nicht, wo der Wert eingesetzt wird", s.Name)
	}
	for _, h := range s.Hosts {
		if h == "" || strings.ContainsAny(h, " /") {
			return fmt.Errorf("secret %s: ungültiger Host %q", s.Name, h)
		}
	}
	if s.Value == "" {
		return fmt.Errorf("secret %s: Wert ist leer", s.Name)
	}
	return nil
}

// ValidateAll prüft alle Secrets und verbietet doppelte Namen.
func ValidateAll(specs []SecretSpec, mode Mode) error {
	seen := map[string]bool{}
	for _, s := range specs {
		if err := s.Validate(mode); err != nil {
			return err
		}
		if seen[s.Name] {
			return fmt.Errorf("secret %s ist doppelt definiert", s.Name)
		}
		seen[s.Name] = true
	}
	return nil
}

// RefKind unterscheidet Herkunftsarten von Secret-Werten.
type RefKind int

// Herkunftsarten.
const (
	Literal RefKind = iota
	EnvRef
	FileRef
)

// Ref beschreibt, woher ein Wert stammt. Die Auflösung erledigt ein Adapter.
type Ref struct {
	Kind RefKind
	Arg  string
}

// ParseRef erkennt ${env:NAME}, ${file:pfad} und mit allowBare auch ${NAME}.
func ParseRef(raw string, allowBare bool) (Ref, error) {
	if !strings.HasPrefix(raw, "${") || !strings.HasSuffix(raw, "}") {
		return Ref{Kind: Literal, Arg: raw}, nil
	}
	inner := raw[2 : len(raw)-1]
	kind, arg, hasKind := strings.Cut(inner, ":")
	switch {
	case hasKind && kind == "env" && envName.MatchString(arg):
		return Ref{Kind: EnvRef, Arg: arg}, nil
	case hasKind && kind == "file" && arg != "":
		return Ref{Kind: FileRef, Arg: arg}, nil
	case !hasKind && allowBare && envName.MatchString(inner):
		return Ref{Kind: EnvRef, Arg: inner}, nil
	}
	return Ref{}, fmt.Errorf("ungültige Secret-Referenz %q (erlaubt: ${env:NAME}, ${file:pfad})", raw)
}
