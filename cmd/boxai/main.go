// Command boxai ist der Composition Root: Er erzeugt Adapter und verdrahtet Use Cases.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/feierabendgewitter/boxai/internal/adapter/in/cli"
	"github.com/feierabendgewitter/boxai/internal/adapter/out/assetfs"
	"github.com/feierabendgewitter/boxai/internal/adapter/out/configfile"
	"github.com/feierabendgewitter/boxai/internal/adapter/out/dockerbuild"
	"github.com/feierabendgewitter/boxai/internal/adapter/out/execx"
	"github.com/feierabendgewitter/boxai/internal/adapter/out/fsscan"
	"github.com/feierabendgewitter/boxai/internal/adapter/out/gitcli"
	"github.com/feierabendgewitter/boxai/internal/adapter/out/hostprobe"
	"github.com/feierabendgewitter/boxai/internal/adapter/out/kitfs"
	"github.com/feierabendgewitter/boxai/internal/adapter/out/sbxcli"
	"github.com/feierabendgewitter/boxai/internal/adapter/out/statejson"
	"github.com/feierabendgewitter/boxai/internal/app/createworkspace"
	"github.com/feierabendgewitter/boxai/internal/app/doctor"
	"github.com/feierabendgewitter/boxai/internal/app/lifecycle"
	"github.com/feierabendgewitter/boxai/internal/app/port"
	"github.com/feierabendgewitter/boxai/internal/app/removeworkspace"
	"github.com/feierabendgewitter/boxai/internal/app/scanrepos"
	"github.com/feierabendgewitter/boxai/internal/domain/agent"
	"github.com/feierabendgewitter/boxai/kits"
)

func main() {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Fehler: Home-Verzeichnis unbekannt:", err)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := cli.Execute(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr, func(rt cli.Runtime, p port.Prompter, n port.Notifier) cli.Services {
		return build(home, rt, p, n)
	})
	stop()
	os.Exit(code)
}

func build(home string, rt cli.Runtime, prompter port.Prompter, notifier port.Notifier) cli.Services {
	level := slog.LevelWarn
	if rt.Verbose {
		level = slog.LevelDebug
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
	runner := &execx.Exec{DryRun: rt.DryRun, Log: log, PlanOut: os.Stdout}

	boxaiHome := os.Getenv("BOXAI_HOME")
	if boxaiHome == "" {
		boxaiHome = filepath.Join(home, ".boxai")
	}
	git := gitcli.New(runner)
	scanner := fsscan.New()
	sbx := sbxcli.New(runner)
	kit := kitfs.New(runner, kits.FS, filepath.Join(boxaiHome, "builtin-kits"), rt.DryRun)
	state := statejson.New(filepath.Join(boxaiHome, "state"), rt.DryRun)
	config := configfile.New(home, filepath.Join(boxaiHome, "config.yaml"), os.LookupEnv, func(v string) { runner.Redact(v) })
	doc := doctor.New(hostprobe.New(runner, exec.LookPath))

	return cli.Services{
		Doctor: doc,
		Scan:   scanrepos.New(scanner, git),
		Create: createworkspace.New(createworkspace.Deps{
			Doctor: doc, Scanner: scanner, Git: git, Runtime: sbx, Secrets: sbx, Kits: kit,
			Assets: assetfs.New(), Images: dockerbuild.New(runner), Config: config, State: state,
			Prompter: prompter, Notifier: notifier, Registry: agent.NewRegistry(), Now: time.Now,
		}),
		Remove:    removeworkspace.New(git, sbx, sbx, kit, state, notifier),
		Lifecycle: lifecycle.New(sbx, state),
		Defaults: cli.Defaults{
			Root:      filepath.Join(home, "boxai-workspaces"),
			CacheRoot: filepath.Join(boxaiHome, "repos"),
			KitRoot:   filepath.Join(boxaiHome, "kits"),
		},
	}
}
