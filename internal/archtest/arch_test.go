// Package archtest erzwingt die Abhängigkeitsregel der hexagonalen Architektur.
package archtest

import (
	"bufio"
	"bytes"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const module = "github.com/feierabendgewitter/boxai"

// touchSources liest alle Quelldateien des Kerns. Der Test-Cache von Go kennt nur
// Dateien, die der Test selbst öffnet; ohne das bliebe ein neuer Verstoß, den nur
// `go list` sieht, hinter einem gecachten `ok` verborgen.
func touchSources(t *testing.T) {
	t.Helper()
	for _, dir := range []string{"../domain", "../app"} {
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") {
				return err
			}
			_, err = os.ReadFile(path)
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
		// Neue Dateien ändern die Verzeichnisliste; auch sie muss der Cache sehen.
		if err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || !d.IsDir() {
				return err
			}
			_, err = os.ReadDir(path)
			return err
		}); err != nil {
			t.Fatal(err)
		}
	}
}

func goList(t *testing.T, args ...string) []string {
	t.Helper()
	touchSources(t)
	out, err := exec.Command("go", append([]string{"list"}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("go list %v: %v\n%s", args, err, out)
	}
	var lines []string
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	return lines
}

// forbiddenDep meldet Pakete, die transitiv nie im Kern auftauchen dürfen.
func forbiddenDep(dep string) bool {
	return dep == "os/exec" || strings.HasPrefix(dep, module+"/internal/adapter")
}

func TestCoreHasNoForbiddenDependencies(t *testing.T) {
	for _, pattern := range []string{module + "/internal/domain/...", module + "/internal/app/..."} {
		for _, dep := range goList(t, "-deps", pattern) {
			if forbiddenDep(dep) {
				t.Errorf("%s hängt von %s ab", pattern, dep)
			}
		}
	}
}

// `os` steckt transitiv in fmt; deshalb zählen hier die direkten Imports.
func TestCoreDoesNoDirectIO(t *testing.T) {
	banned := map[string]bool{"os": true, "net": true, "os/exec": true, "net/http": true}
	template := `{{.ImportPath}}: {{join .Imports " "}}`
	for _, pattern := range []string{module + "/internal/domain/...", module + "/internal/app/..."} {
		for _, line := range goList(t, "-f", template, pattern) {
			pkg, imports, _ := strings.Cut(line, ": ")
			for imp := range strings.FieldsSeq(imports) {
				if banned[imp] {
					t.Errorf("%s importiert %s direkt", pkg, imp)
				}
			}
		}
	}
}

func TestDomainDoesNotImportApp(t *testing.T) {
	for _, dep := range goList(t, "-deps", module+"/internal/domain/...") {
		if strings.HasPrefix(dep, module+"/internal/app") {
			t.Errorf("domain hängt von %s ab", dep)
		}
	}
}

func TestForbiddenDepDetectsViolations(t *testing.T) {
	for _, dep := range []string{"os/exec", module + "/internal/adapter/out/gitcli"} {
		if !forbiddenDep(dep) {
			t.Errorf("%s müsste verboten sein", dep)
		}
	}
	if forbiddenDep("fmt") {
		t.Error("fmt ist erlaubt")
	}
}
