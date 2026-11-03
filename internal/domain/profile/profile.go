// Package profile enthält die Stack-Profile: Netzwerk, Konfigurationsdateien und Image-Wahl.
package profile

import (
	"fmt"
	"slices"
	"strings"
)

// Namen der mitgelieferten Profile.
const (
	KotlinSpring = "kotlin-spring"
	React        = "react"
)

// NetworkRule erlaubt der Sandbox den Zugriff auf einen Host.
type NetworkRule struct {
	Host string
}

// ConfigFile ist eine Konfigurationsdatei relativ zum Home der Sandbox. Sie
// verweist auf Env-Variablen und enthält nie Secret-Werte.
type ConfigFile struct {
	Path    string
	Content string
	Mode    uint32
	// RequiresSecret: Die Datei entsteht nur, wenn dieses Secret existiert. Tools wie
	// npm scheitern sonst an einer nicht ersetzbaren Variable.
	RequiresSecret string
}

// Profile beschreibt einen Stack.
type Profile struct {
	Name    string
	Network []NetworkRule
	Files   []ConfigFile
	// Hints sind Build-Kommandos für die generierte AGENTS.md.
	Hints []string
}

func hosts(names ...string) []NetworkRule {
	rules := make([]NetworkRule, len(names))
	for i, n := range names {
		rules[i] = NetworkRule{Host: n}
	}
	return rules
}

const mavenSettings = `<settings>
  <servers>
    <server>
      <id>boxai-private</id>
      <username>${env.MAVEN_USERNAME}</username>
      <password>${env.MAVEN_TOKEN}</password>
    </server>
  </servers>
</settings>
`

// Gradle ersetzt in gradle.properties keine Env-Variablen. Build-Skripte lesen
// das Token deshalb selbst aus der Umgebung.
const gradleProperties = `# Von boxai erzeugt. Secrets liegen nicht hier, sondern in der Umgebung:
# Build-Skripte lesen System.getenv("MAVEN_TOKEN").
org.gradle.daemon=false
`

const npmrc = "//registry.npmjs.org/:_authToken=${NPM_TOKEN}\n"

var builtin = map[string]Profile{
	KotlinSpring: {
		Name:    KotlinSpring,
		Network: hosts("repo.maven.apache.org", "repo1.maven.org", "plugins.gradle.org", "services.gradle.org", "downloads.gradle.org"),
		Files: []ConfigFile{
			{Path: ".m2/settings.xml", Content: mavenSettings, Mode: 0o600, RequiresSecret: "MAVEN_TOKEN"},
			{Path: ".gradle/gradle.properties", Content: gradleProperties, Mode: 0o644},
		},
		Hints: []string{"./gradlew build", "./gradlew test", "./mvnw verify"},
	},
	React: {
		Name:    React,
		Network: hosts("registry.npmjs.org", "registry.yarnpkg.com"),
		Files:   []ConfigFile{{Path: ".npmrc", Content: npmrc, Mode: 0o600, RequiresSecret: "NPM_TOKEN"}},
		Hints:   []string{"npm ci", "npm test", "npm run build"},
	},
}

// alwaysAllowed gilt für jede Sandbox. `**` deckt auch regionale AWS-Hosts mit
// mehreren Labels ab, etwa s3.eu-central-1.amazonaws.com.
var alwaysAllowed = hosts("github.com", "api.github.com", "gitlab.com", "**.amazonaws.com")

// Names liefert die mitgelieferten Profile sortiert.
func Names() []string {
	names := make([]string, 0, len(builtin))
	for n := range builtin {
		names = append(names, n)
	}
	slices.Sort(names)
	return names
}

// AllHosts liefert alle Freigaben der mitgelieferten Profile ohne Duplikate.
func AllHosts() []string {
	r, err := Resolve(Names(), nil)
	if err != nil {
		panic(err) // Names liefert nur bekannte Profile.
	}
	hosts := make([]string, len(r.Network))
	for i, rule := range r.Network {
		hosts[i] = rule.Host
	}
	return hosts
}

// Resolved ist die Kombination gewählter Profile.
type Resolved struct {
	Names   []string
	Network []NetworkRule
	Files   []ConfigFile
	Hints   []string
}

// Resolve kombiniert Profile; extraHosts ergänzt Freigaben aus Konfiguration und Secrets.
func Resolve(names, extraHosts []string) (Resolved, error) {
	r := Resolved{Names: slices.Clone(names)}
	seen := map[string]bool{}
	addHost := func(host string) {
		if host != "" && !seen[host] {
			seen[host] = true
			r.Network = append(r.Network, NetworkRule{Host: host})
		}
	}
	for _, rule := range alwaysAllowed {
		addHost(rule.Host)
	}
	for _, name := range names {
		p, ok := builtin[name]
		if !ok {
			return Resolved{}, fmt.Errorf("unbekanntes Profil %q (verfügbar: %s)", name, strings.Join(Names(), ", "))
		}
		for _, rule := range p.Network {
			addHost(rule.Host)
		}
		r.Files = append(r.Files, p.Files...)
		r.Hints = append(r.Hints, p.Hints...)
	}
	for _, host := range extraHosts {
		addHost(stripPort(host))
	}
	return r, nil
}

// stripPort vereinheitlicht host:443 und host zu einer Freigabe pro Host. sbx kann
// Ports einschränken; Freigaben hier gelten bewusst für den Host.
func stripPort(host string) string {
	name, _, found := strings.Cut(host, ":")
	if found {
		return name
	}
	return host
}

// FilesFor filtert Dateien, deren benötigtes Secret fehlt.
func (r Resolved) FilesFor(secretNames []string) []ConfigFile {
	out := make([]ConfigFile, 0, len(r.Files))
	for _, f := range r.Files {
		if f.RequiresSecret == "" || slices.Contains(secretNames, f.RequiresSecret) {
			out = append(out, f)
		}
	}
	return out
}

// ImageTag wählt das mitgelieferte Image für Agent und Profile.
func ImageTag(agent string, names []string, noDocker bool) string {
	kotlin := slices.Contains(names, KotlinSpring)
	react := slices.Contains(names, React)
	layer := "base"
	switch {
	case kotlin && react:
		layer = "fullstack"
	case kotlin:
		layer = "kotlin"
	case react:
		layer = "react"
	}
	tag := "boxai-" + layer + "-" + agent
	if noDocker {
		tag += "-nodocker"
	}
	return tag + ":latest"
}
