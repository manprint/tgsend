package main

import (
	"context"
	"os"

	"github.com/manprint/tgsend/internal/app"
	"github.com/manprint/tgsend/internal/buildinfo"
	"github.com/manprint/tgsend/internal/cli"
)

func main() {
	deps := cli.Dependencies{
		Stdin:     os.Stdin,
		Stdout:    os.Stdout,
		Stderr:    os.Stderr,
		BuildInfo: buildinfo.Current(),
		App:       app.NewService(os.Stdin, app.IsTerminal(os.Stdin)),
	}
	os.Exit(cli.Execute(context.Background(), deps, os.Args[1:]))
}
