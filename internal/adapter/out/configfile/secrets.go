package configfile

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/feierabendgewitter/boxai/internal/domain/secret"
)

type secretsFile struct {
	Secrets []struct {
		Name  string   `yaml:"name"`
		Hosts []string `yaml:"hosts"`
		Value string   `yaml:"value"`
	} `yaml:"secrets"`
}

// LoadSecrets liest `.env`- oder YAML-Dateien und löst Referenzen auf.
func (l *Loader) LoadSecrets(_ context.Context, path string) ([]secret.SecretSpec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	baseDir := filepath.Dir(path)
	base := filepath.Base(path)
	if base == ".env" || strings.HasSuffix(base, ".env") {
		return l.parseEnv(string(data), baseDir)
	}
	var f secretsFile
	if err := decodeStrict(data, &f); err != nil {
		return nil, redactYAMLError(err)
	}
	specs := make([]secret.SecretSpec, 0, len(f.Secrets))
	for _, entry := range f.Secrets {
		value, err := l.resolveValue(entry.Name, entry.Value, false, baseDir)
		if err != nil {
			return nil, err
		}
		specs = append(specs, secret.SecretSpec{Name: entry.Name, Hosts: entry.Hosts, Value: value})
	}
	return specs, nil
}

// redactYAMLError entfernt Zeileninhalte, weil yaml.v3 Teile der Eingabe zitieren kann.
func redactYAMLError(err error) error {
	msg := err.Error()
	if first, _, found := strings.Cut(msg, ":"); found {
		return fmt.Errorf("secret-datei ist ungültig (%s)", first)
	}
	return errors.New("secret-datei ist ungültig")
}

// parseEnv liest NAME=wert; `# @hosts a b` gilt für die nächste Zuweisung. Das Format
// braucht diese Kommentare, die gängige .env-Parser verwerfen.
func (l *Loader) parseEnv(content, baseDir string) ([]secret.SecretSpec, error) {
	var specs []secret.SecretSpec
	var hosts []string
	for i, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(raw)
		if rest, ok := strings.CutPrefix(line, "# @hosts"); ok {
			hosts = strings.FieldsFunc(rest, func(r rune) bool { return r == ' ' || r == ',' || r == '\t' })
			continue
		}
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		name, rawValue, found := strings.Cut(line, "=")
		if !found {
			// Die Zeile könnte einen Wert enthalten; deshalb nur die Nummer nennen.
			return nil, fmt.Errorf("zeile %d: NAME=wert erwartet", i+1)
		}
		name = strings.TrimSpace(name)
		value, err := l.resolveValue(name, unquote(strings.TrimSpace(rawValue)), true, baseDir)
		if err != nil {
			return nil, err
		}
		specs = append(specs, secret.SecretSpec{Name: name, Hosts: hosts, Value: value})
		hosts = nil
	}
	return specs, nil
}

func unquote(v string) string {
	if len(v) >= 2 && (v[0] == '"' || v[0] == '\'') && v[len(v)-1] == v[0] {
		return v[1 : len(v)-1]
	}
	return v
}

func (l *Loader) resolveValue(name, raw string, allowBare bool, baseDir string) (string, error) {
	ref, err := secret.ParseRef(raw, allowBare)
	if err != nil {
		return "", fmt.Errorf("secret %s: %w", name, err)
	}
	var value string
	switch ref.Kind {
	case secret.Literal:
		value = ref.Arg
	case secret.EnvRef:
		var ok bool
		if value, ok = l.getenv(ref.Arg); !ok {
			return "", fmt.Errorf("secret %s: Umgebungsvariable %s ist nicht gesetzt", name, ref.Arg)
		}
	case secret.FileRef:
		data, err := os.ReadFile(l.resolve(baseDir, ref.Arg))
		if err != nil {
			return "", fmt.Errorf("secret %s: %w", name, err)
		}
		value = strings.TrimRight(string(data), "\r\n")
	}
	if l.onSecret != nil {
		l.onSecret(value)
	}
	return value, nil
}
