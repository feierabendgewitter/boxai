//go:build e2e

// Package e2e prüft boxai gegen ein echtes sbx. Es legt nur Objekte mit dem Präfix
// boxai-e2e an und nutzt erfundene Secret-Werte.
package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const fakeSecret = "boxai-e2e-fake-value-7f3a"

type harness struct {
	t        *testing.T
	bin      string
	home     string
	root     string
	src      string
	name     string
	sandbox  string
	combined bytes.Buffer
}

func newHarness(t *testing.T, suffix string) *harness {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	h := &harness{
		t: t, bin: filepath.Join(base, "boxai"), home: filepath.Join(base, "home"),
		root: filepath.Join(base, "ws"), src: filepath.Join(base, "src"), name: "e2e-" + suffix,
	}
	h.sandbox = "boxai-" + h.name
	out, err := exec.Command("go", "build", "-o", h.bin, "../cmd/boxai").CombinedOutput()
	if err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	return h
}

func (h *harness) run(timeout time.Duration, name string, args ...string) (string, error) {
	h.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = append(os.Environ(), "BOXAI_HOME="+h.home, "BOXAI_E2E_TOKEN="+fakeSecret)
	out, err := cmd.CombinedOutput()
	h.combined.Write(out)
	return string(out), err
}

func (h *harness) must(timeout time.Duration, name string, args ...string) string {
	h.t.Helper()
	out, err := h.run(timeout, name, args...)
	if err != nil {
		h.t.Fatalf("%s %v: %v\n%s", name, args, err, out)
	}
	return out
}

func (h *harness) exec(script string) string {
	h.t.Helper()
	return h.must(2*time.Minute, "sbx", "exec", h.sandbox, "sh", "-c", script)
}

func (h *harness) repo(name string, files map[string]string) string {
	h.t.Helper()
	dir := filepath.Join(h.src, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		h.t.Fatal(err)
	}
	for rel, content := range files {
		path := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			h.t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			h.t.Fatal(err)
		}
	}
	h.must(time.Minute, "git", "init", "-q", "-b", "main", dir)
	h.must(time.Minute, "git", "-C", dir, "add", ".")
	h.must(time.Minute, "git", "-C", dir, "-c", "user.name=e2e", "-c", "user.email=e2e@example.invalid", "commit", "-q", "--allow-empty", "-m", "init")
	return dir
}

func (h *harness) cleanup() {
	// Bewusst namentlich: fremde Sandboxes bleiben unberührt.
	_, _ = h.run(2*time.Minute, h.bin, "rm", h.name, "--force", "--delete-branches")
	_, _ = h.run(2*time.Minute, "sbx", "rm", "--force", h.sandbox)
}

func (h *harness) sandboxExists() bool {
	h.t.Helper()
	out := h.must(time.Minute, "sbx", "ls", "--format", "json")
	var list []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal([]byte(out), &list); err != nil {
		h.t.Fatalf("%v\n%s", err, out)
	}
	for _, s := range list {
		if s.Name == h.sandbox {
			return true
		}
	}
	return false
}

// assertNoSecret durchsucht alles, was boxai auf Platte oder in Ausgaben hinterlässt.
func (h *harness) assertNoSecret() {
	h.t.Helper()
	if strings.Contains(h.combined.String(), fakeSecret) {
		h.t.Fatal("Secret-Wert in einer Ausgabe")
	}
	err := filepath.WalkDir(h.home, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if bytes.Contains(data, []byte(fakeSecret)) {
			return fmt.Errorf("Secret-Wert in %s", path)
		}
		return nil
	})
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		h.t.Fatal(err)
	}
}

func TestWorkspaceLifecycle(t *testing.T) {
	h := newHarness(t, "life")
	t.Cleanup(h.cleanup)
	api := h.repo("api", map[string]string{
		".claude/agents/reviewer.md":   "# Reviewer\n",
		".claude/skills/lint/SKILL.md": "---\nname: lint\n---\nRegeln\n",
		".okf/index.md":                "# Wissen\n",
		".okf/a.md":                    "---\ntype: Decision\n---\n# A\n",
	})
	ui := h.repo("ui", map[string]string{".claude/agents/reviewer.md": "# Reviewer UI\n"})
	secrets := filepath.Join(h.src, "secrets.yaml")
	content := "secrets:\n  - name: E2E_TOKEN\n    hosts: [httpbin.org]\n    value: ${env:BOXAI_E2E_TOKEN}\n"
	if err := os.WriteFile(secrets, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	out := h.must(25*time.Minute, h.bin, "up", "--no-attach", "--dir", h.src, "--root", h.root, "--name", h.name,
		"--repo", "api", "--repo", "ui", "--agent", "claude", "--secrets", secrets)
	if !strings.Contains(out, "Konflikt bei reviewer.md") {
		t.Fatalf("Konflikt nicht gemeldet:\n%s", out)
	}
	wsPath := filepath.Join(h.root, h.name)

	t.Run("Git in beiden Worktrees", func(t *testing.T) {
		for _, repo := range []string{"api", "ui"} {
			h.exec(fmt.Sprintf("cd %s/%s && echo x > e2e.txt && git add e2e.txt && git -c user.name=a -c user.email=a@example.invalid commit -qm aus-sandbox", wsPath, repo))
		}
		for _, repo := range []string{api, ui} {
			log := h.must(time.Minute, "git", "-C", repo, "log", "--format=%s", "-1", "boxai/"+h.name)
			if strings.TrimSpace(log) != "aus-sandbox" {
				t.Fatalf("%s: %q", repo, log)
			}
		}
	})

	t.Run("Kit-Dateien", func(t *testing.T) {
		got := h.exec(fmt.Sprintf("cat ~/.claude/agents/api-reviewer.md ~/.claude/agents/ui-reviewer.md ~/.claude/skills/lint/SKILL.md %s/AGENTS.md %s/CLAUDE.md", wsPath, wsPath))
		for _, want := range []string{"# Reviewer UI", "Regeln", "api/.okf/index.md", "Lies zuerst"} {
			if !strings.Contains(got, want) {
				t.Errorf("%q fehlt:\n%s", want, got)
			}
		}
	})

	t.Run("Secret erreicht die Sandbox nur als Platzhalter", func(t *testing.T) {
		// Vor dem Echo prüfen: Danach enthält die Ausgabe den Wert gewollt.
		h.assertNoSecret()
		env := h.exec(`printf %s "$E2E_TOKEN"`)
		if strings.TrimSpace(env) == "" || strings.Contains(env, fakeSecret) {
			t.Fatalf("Env = %q", env)
		}
		echoed := h.exec(`curl -fsS -H "Authorization: Bearer $E2E_TOKEN" https://httpbin.org/headers`)
		if !strings.Contains(echoed, "Bearer "+fakeSecret) {
			t.Fatalf("Proxy ersetzt den Platzhalter nicht:\n%s", echoed)
		}
		// Diese Ausgabe enthält den Wert gewollt; sie gehört nicht zur Redaction-Prüfung.
		h.combined.Reset()
	})

	t.Run("Netzwerk gilt nur für diese Sandbox", func(t *testing.T) {
		scoped := h.must(time.Minute, "sbx", "policy", "ls", h.sandbox)
		if !strings.Contains(scoped, h.sandbox) {
			t.Fatal(scoped)
		}
	})

	t.Run("Zweiter Lauf ist idempotent", func(t *testing.T) {
		out := h.must(5*time.Minute, h.bin, "up", "--no-attach", "--name", h.name, "--agent", "claude", "--root", h.root)
		if !strings.Contains(out, "existiert") {
			t.Fatal(out)
		}
	})

	t.Run("list zeigt den Workspace", func(t *testing.T) {
		out := h.must(time.Minute, h.bin, "list")
		if !strings.Contains(out, h.name) || !strings.Contains(out, "api,ui") {
			t.Fatal(out)
		}
	})

	t.Run("Dirty blockiert rm", func(t *testing.T) {
		if err := os.WriteFile(filepath.Join(wsPath, "api", "dirty.txt"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		out, err := h.run(time.Minute, h.bin, "rm", h.name)
		if err == nil || !strings.Contains(out, "uncommittete") {
			t.Fatalf("%v\n%s", err, out)
		}
		if !h.sandboxExists() {
			t.Fatal("Sandbox trotz Abbruch entfernt")
		}
		if err := os.Remove(filepath.Join(wsPath, "api", "dirty.txt")); err != nil {
			t.Fatal(err)
		}
	})

	h.assertNoSecret()

	t.Run("rm räumt vollständig auf", func(t *testing.T) {
		h.must(3*time.Minute, h.bin, "rm", h.name)
		if h.sandboxExists() {
			t.Error("Sandbox existiert noch")
		}
		if _, err := os.Stat(wsPath); !os.IsNotExist(err) {
			entries, _ := os.ReadDir(wsPath)
			t.Errorf("Workspace existiert noch: %v", entries)
		}
		if _, err := os.Stat(filepath.Join(h.home, "state", h.name+".json")); !os.IsNotExist(err) {
			t.Error("State existiert noch")
		}
		if _, err := os.Stat(filepath.Join(h.home, "kits", h.name)); !os.IsNotExist(err) {
			t.Error("Kit existiert noch")
		}
		list := h.must(time.Minute, "git", "-C", api, "worktree", "list")
		if strings.Contains(list, wsPath) {
			t.Errorf("Worktree registriert: %s", list)
		}
		branches := h.must(time.Minute, "git", "-C", api, "branch", "--list", "boxai/"+h.name)
		if !strings.Contains(branches, "boxai/"+h.name) {
			t.Error("Branch ohne --delete-branches gelöscht")
		}
		secrets := h.must(time.Minute, "sbx", "secret", "ls", "--sandbox", h.sandbox, "-q")
		if strings.TrimSpace(secrets) != "" {
			t.Errorf("Secrets übrig: %q", secrets)
		}
	})
}

func TestRollbackAfterFailedSandbox(t *testing.T) {
	h := newHarness(t, "roll")
	t.Cleanup(h.cleanup)
	api := h.repo("api", nil)
	secrets := filepath.Join(h.src, "secrets.env")
	if err := os.WriteFile(secrets, []byte("# @hosts example.invalid\nE2E_TOKEN=${BOXAI_E2E_TOKEN}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := h.run(10*time.Minute, h.bin, "up", "--no-attach", "--dir", h.src, "--root", h.root, "--name", h.name,
		"--repo", "api", "--agent", "claude", "--secrets", secrets, "--image", "ghcr.io/boxai-e2e/gibt-es-nicht:1")
	if err == nil {
		t.Fatalf("Fehler erwartet:\n%s", out)
	}
	// Die Secrets standen schon vor dem gescheiterten create; hier greift DeleteAll wirklich.
	if left := h.must(time.Minute, "sbx", "secret", "ls", "--sandbox", h.sandbox, "-q"); strings.TrimSpace(left) != "" {
		t.Errorf("Secrets übrig: %q\nAusgabe von up:\n%s", left, out)
	}
	h.assertNoSecret()
	list := h.must(time.Minute, "git", "-C", api, "worktree", "list")
	if strings.Contains(list, filepath.Join(h.root, h.name)) {
		t.Errorf("Worktree übrig: %s", list)
	}
	if branches := h.must(time.Minute, "git", "-C", api, "branch", "--list", "boxai/"+h.name); strings.TrimSpace(branches) != "" {
		t.Errorf("Branch übrig: %s", branches)
	}
	if _, err := os.Stat(filepath.Join(h.home, "state", h.name+".json")); !os.IsNotExist(err) {
		t.Error("State übrig")
	}
	if h.sandboxExists() {
		t.Error("Sandbox übrig")
	}
}
