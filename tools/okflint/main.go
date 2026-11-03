// Command okflint prüft ein Bundle gegen die Konformitätsregeln von OKF v0.2.
package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const wantVersion = "0.2"

var (
	dateHeading = regexp.MustCompile(`^## \d{4}-\d{2}-\d{2}$`)
	mdLink      = regexp.MustCompile(`\]\(([^)#\s]+\.md)(#[^)]*)?\)`)
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: okflint <bundle>")
		os.Exit(2)
	}
	problems, err := lint(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	for _, p := range problems {
		fmt.Fprintln(os.Stderr, p)
	}
	if len(problems) > 0 {
		os.Exit(1)
	}
}

// lint liefert alle Verstöße. Zusätzlich zur Spezifikation fordert boxai die
// Konventionsfelder title und status sowie auflösbare Links (--strict-Niveau).
func lint(root string) ([]string, error) {
	var problems []string
	report := func(path, format string, args ...any) {
		problems = append(problems, path+": "+fmt.Sprintf(format, args...))
	}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".md") {
			return err
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		content := string(raw)
		fields, hasFrontmatter, fmErr := frontmatter(content)
		if fmErr != nil {
			report(path, "%v", fmErr)
		}
		rel, _ := filepath.Rel(root, path)
		switch filepath.Base(path) {
		case "index.md":
			if rel == "index.md" {
				if fields["okf_version"] != wantVersion || len(fields) != 1 {
					report(path, "root index.md darf nur okf_version: %q tragen", wantVersion)
				}
			} else if hasFrontmatter {
				report(path, "index.md außerhalb der Wurzel trägt kein Frontmatter")
			}
		case "log.md":
			if hasFrontmatter {
				report(path, "log.md trägt kein Frontmatter")
			}
			checkLog(content, func(msg string) { report(path, "%s", msg) })
		default:
			if !hasFrontmatter {
				report(path, "Konzept braucht YAML-Frontmatter")
			}
			for _, key := range []string{"type", "title", "status"} {
				if fields[key] == "" {
					report(path, "Frontmatter-Feld %q fehlt oder ist leer", key)
				}
			}
		}
		for _, m := range mdLink.FindAllStringSubmatch(content, -1) {
			target := m[1]
			if strings.Contains(target, "://") {
				continue
			}
			resolved := filepath.Join(filepath.Dir(path), target)
			if strings.HasPrefix(target, "/") {
				resolved = filepath.Join(root, target)
			}
			if _, statErr := os.Stat(resolved); statErr != nil {
				report(path, "toter Link %s", target)
			}
		}
		return nil
	})
	return problems, err
}

// frontmatter liest die Top-Level-Schlüssel. Es prüft Blockstruktur statt voller
// YAML-Grammatik; verschachtelte Werte kommen im Bundle nicht vor.
func frontmatter(content string) (map[string]string, bool, error) {
	fields := map[string]string{}
	if !strings.HasPrefix(content, "---\n") {
		return fields, false, nil
	}
	body, _, found := strings.Cut(content[4:], "\n---")
	if !found {
		return fields, true, errors.New("offenes Frontmatter")
	}
	for line := range strings.SplitSeq(body, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			return fields, true, fmt.Errorf("ungültige Zeile im Frontmatter: %q", line)
		}
		fields[strings.TrimSpace(key)] = strings.Trim(strings.TrimSpace(value), `"`)
	}
	return fields, true, nil
}

func checkLog(content string, report func(string)) {
	last := ""
	for line := range strings.SplitSeq(content, "\n") {
		if !strings.HasPrefix(line, "## ") {
			continue
		}
		if !dateHeading.MatchString(line) {
			report("Überschrift ist kein ISO-Datum: " + line)
			continue
		}
		if last != "" && line > last {
			report("Einträge sind nicht absteigend sortiert: " + line)
		}
		last = line
	}
}
