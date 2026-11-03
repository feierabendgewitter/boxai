package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLint(t *testing.T) {
	const validIndex = "---\nokf_version: \"0.2\"\n---\n# Index\n"
	const concept = "---\ntype: Decision\ntitle: T\nstatus: accepted\n---\n# T\n"
	tests := []struct {
		name  string
		files map[string]string
		want  string
	}{
		{name: "gültig", files: map[string]string{
			"index.md": validIndex + "- [a](a.md)\n",
			"a.md":     concept,
			"log.md":   "# Log\n\n## 2026-11-03\n\nText\n\n## 2026-11-02\n\nText\n",
		}},
		{name: "falsche Version", files: map[string]string{"index.md": "---\nokf_version: \"0.1\"\n---\n"}, want: "okf_version"},
		{name: "type fehlt", files: map[string]string{"a.md": "---\ntitle: T\nstatus: x\n---\n"}, want: `"type"`},
		{name: "kein Frontmatter", files: map[string]string{"a.md": "# T\n"}, want: "braucht YAML"},
		{name: "toter Link", files: map[string]string{"a.md": concept + "[x](fehlt.md)\n"}, want: "toter Link"},
		{name: "Log nicht sortiert", files: map[string]string{"log.md": "## 2026-11-01\n\n## 2026-11-02\n"}, want: "absteigend"},
		{name: "Log-Datum", files: map[string]string{"log.md": "## gestern\n"}, want: "ISO-Datum"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			write(t, root, tt.files)
			problems, err := lint(root)
			if err != nil {
				t.Fatal(err)
			}
			joined := strings.Join(problems, "\n")
			if tt.want == "" && joined != "" {
				t.Fatalf("unerwartete Probleme: %s", joined)
			}
			if !strings.Contains(joined, tt.want) {
				t.Fatalf("erwartet %q in %q", tt.want, joined)
			}
		})
	}
}
