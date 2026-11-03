// Command boxai ist der Composition Root: Er erzeugt Adapter und verdrahtet Use Cases.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/feierabendgewitter/boxai/internal/adapter/in/cli"
	"github.com/feierabendgewitter/boxai/internal/adapter/out/execx"
	"github.com/feierabendgewitter/boxai/internal/adapter/out/fsscan"
	"github.com/feierabendgewitter/boxai/internal/adapter/out/gitcli"
	"github.com/feierabendgewitter/boxai/internal/adapter/out/hostprobe"
	"github.com/feierabendgewitter/boxai/internal/app/doctor"
	"github.com/feierabendgewitter/boxai/internal/app/scanrepos"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := cli.Execute(ctx, os.Args[1:], os.Stdout, os.Stderr, build)
	stop()
	os.Exit(code)
}

func build(rt cli.Runtime) cli.Services {
	level := slog.LevelWarn
	if rt.Verbose {
		level = slog.LevelDebug
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
	runner := &execx.Exec{DryRun: rt.DryRun, Log: log, PlanOut: os.Stdout}
	return cli.Services{
		Doctor: doctor.New(hostprobe.New(runner, exec.LookPath)),
		Scan:   scanrepos.New(fsscan.New(), gitcli.New(runner)),
	}
}
