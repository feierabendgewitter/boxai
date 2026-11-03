// Package dockerbuild implementiert port.ImageBuilder über docker und sbx template.
package dockerbuild

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/feierabendgewitter/boxai/internal/adapter/out/execx"
)

const labelFormat = `{{ index .Config.Labels "boxai.agent" }}`

// Builder implementiert port.ImageBuilder.
type Builder struct {
	runner execx.Runner
}

// New erzeugt den Adapter.
func New(runner execx.Runner) *Builder { return &Builder{runner: runner} }

// BuildAndLoad baut mit dem Ordner des Dockerfiles als Kontext und lädt das Image in sbx.
func (b *Builder) BuildAndLoad(ctx context.Context, dockerfile, tag string) (string, error) {
	if _, err := b.runner.Run(ctx, execx.Cmd{
		Name: "docker", Args: []string{"build", "-f", dockerfile, "-t", tag, filepath.Dir(dockerfile)},
	}); err != nil {
		return "", err
	}
	tmp, err := os.MkdirTemp("", "boxai-image-")
	if err != nil {
		return "", err
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	tar := filepath.Join(tmp, "image.tar")
	if _, err := b.runner.Run(ctx, execx.Cmd{Name: "docker", Args: []string{"image", "save", tag, "-o", tar}}); err != nil {
		return "", err
	}
	if _, err := b.runner.Run(ctx, execx.Cmd{Name: "sbx", Args: []string{"template", "load", tar}}); err != nil {
		return "", err
	}
	return tag, nil
}

// Available liest `sbx template ls`, das kein JSON anbietet.
func (b *Builder) Available(ctx context.Context, ref string) (bool, error) {
	res, err := b.runner.Run(ctx, execx.Cmd{Name: "sbx", Args: []string{"template", "ls"}, ReadOnly: true})
	if err != nil {
		return false, fmt.Errorf("sbx template ls: %w", err)
	}
	want := canonical(ref)
	for line := range strings.SplitSeq(res.Stdout, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0]+":"+fields[1] == want {
			return true, nil
		}
	}
	return false, nil
}

// canonical ergänzt Registry und Tag so, wie `sbx template ls` sie anzeigt.
func canonical(ref string) string {
	if last := ref[strings.LastIndex(ref, "/")+1:]; !strings.Contains(last, ":") {
		ref += ":latest"
	}
	first, _, found := strings.Cut(ref, "/")
	switch {
	case !found:
		return "docker.io/library/" + ref
	case !strings.ContainsAny(first, ".:") && first != "localhost":
		return "docker.io/" + ref
	}
	return ref
}

// AgentLabel liest das Label aus dem lokalen Docker-Store.
func (b *Builder) AgentLabel(ctx context.Context, ref string) (string, error) {
	res, err := b.runner.Run(ctx, execx.Cmd{
		Name: "docker", Args: []string{"image", "inspect", "--format", labelFormat, ref}, ReadOnly: true,
	})
	if err != nil {
		return "", err
	}
	label := strings.TrimSpace(res.Stdout)
	if label == "<no value>" {
		return "", nil
	}
	return label, nil
}
