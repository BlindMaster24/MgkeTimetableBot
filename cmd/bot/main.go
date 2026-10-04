package main

import (
	"context"
	"flag"
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
	cfgPath := flag.String("config", "", "path to config file (default: configs/config.yaml)")
	flag.Parse()
	os.Exit(app.Run(context.Background(), app.ResolveConfigPath(*cfgPath), build.New(version, commit, date)))
}
