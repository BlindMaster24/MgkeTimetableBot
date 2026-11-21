// Command bot starts the timetable bot: configuration, polling or webhook, and the HTTP API.
package main

import (
	"context"
	"flag"
	"io"
	"os"

	"github.com/blindmaster24/MgkeTimetableBot/internal/app"
	"github.com/blindmaster24/MgkeTimetableBot/internal/build"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stderr))
}

// run keeps the flag parsing and the exit code testable without spawning the
// whole process: a test hands it a missing config and expects 1.
func run(args []string, out io.Writer) int {
	flags := flag.NewFlagSet("bot", flag.ContinueOnError)
	flags.SetOutput(out)
	cfgPath := flags.String("config", "", "path to config file (default: configs/config.yaml)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	return app.Run(context.Background(), app.ResolveConfigPath(*cfgPath), build.New(version, commit, date))
}
