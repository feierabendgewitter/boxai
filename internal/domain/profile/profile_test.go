package profile

import (
	"slices"
	"strings"
	"testing"
)

func hostNames(r Resolved) []string {
	out := make([]string, len(r.Network))
	for i, n := range r.Network {
		out[i] = n.Host
	}
	return out
}

func TestResolveAlwaysAllowsGitAndAws(t *testing.T) {
	r, err := Resolve(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := hostNames(r)
	for _, want := range []string{"github.com", "api.github.com", "gitlab.com", "**.amazonaws.com"} {
		if !slices.Contains(got, want) {
			t.Errorf("%s fehlt in %v", want, got)
		}
	}
}

func TestResolveCombinesProfilesWithoutDuplicates(t *testing.T) {
	r, err := Resolve([]string{KotlinSpring, React}, []string{"nexus.firma.de:443", "github.com", ""})
	if err != nil {
		t.Fatal(err)
	}
	got := hostNames(r)
	for _, want := range []string{"plugins.gradle.org", "registry.npmjs.org", "nexus.firma.de"} {
		if !slices.Contains(got, want) {
			t.Errorf("%s fehlt in %v", want, got)
		}
	}
	seen := map[string]int{}
	for _, h := range got {
		seen[h]++
		if seen[h] > 1 {
			t.Errorf("%s doppelt", h)
		}
	}
	if len(r.Files) != 3 || !slices.Contains(r.Hints, "npm ci") || !slices.Contains(r.Hints, "./gradlew build") {
		t.Fatalf("%+v", r)
	}
}

func TestResolveUnknownProfile(t *testing.T) {
	_, err := Resolve([]string{"rust"}, nil)
	if err == nil || !strings.Contains(err.Error(), "kotlin-spring, react") {
		t.Fatal(err)
	}
}

func TestConfigFilesReferenceEnvNotValues(t *testing.T) {
	r, _ := Resolve([]string{KotlinSpring, React}, nil)
	byPath := map[string]string{}
	for _, f := range r.Files {
		byPath[f.Path] = f.Content
	}
	if !strings.Contains(byPath[".m2/settings.xml"], "${env.MAVEN_TOKEN}") {
		t.Error(byPath[".m2/settings.xml"])
	}
	if !strings.Contains(byPath[".npmrc"], "${NPM_TOKEN}") {
		t.Error(byPath[".npmrc"])
	}
}

func TestFilesForDropsFilesWithoutSecret(t *testing.T) {
	r, _ := Resolve([]string{KotlinSpring, React}, nil)
	paths := func(files []ConfigFile) []string {
		out := []string{}
		for _, f := range files {
			out = append(out, f.Path)
		}
		return out
	}
	if got := paths(r.FilesFor(nil)); !slices.Equal(got, []string{".gradle/gradle.properties"}) {
		t.Fatal(got)
	}
	if got := paths(r.FilesFor([]string{"NPM_TOKEN"})); !slices.Contains(got, ".npmrc") || slices.Contains(got, ".m2/settings.xml") {
		t.Fatal(got)
	}
}

func TestImageTag(t *testing.T) {
	tests := []struct {
		names    []string
		noDocker bool
		want     string
	}{
		{nil, false, "boxai-base-claude:latest"},
		{[]string{KotlinSpring}, false, "boxai-kotlin-claude:latest"},
		{[]string{React}, false, "boxai-react-claude:latest"},
		{[]string{React, KotlinSpring}, false, "boxai-fullstack-claude:latest"},
		{[]string{React}, true, "boxai-react-claude-nodocker:latest"},
	}
	for _, tt := range tests {
		if got := ImageTag("claude", tt.names, tt.noDocker); got != tt.want {
			t.Errorf("%v: %s", tt.names, got)
		}
	}
}

func TestNames(t *testing.T) {
	if got := Names(); !slices.Equal(got, []string{KotlinSpring, React}) {
		t.Fatal(got)
	}
}

func TestAllHosts(t *testing.T) {
	got := AllHosts()
	for _, want := range []string{"github.com", "plugins.gradle.org", "registry.npmjs.org"} {
		if !slices.Contains(got, want) {
			t.Errorf("%s fehlt in %v", want, got)
		}
	}
}
