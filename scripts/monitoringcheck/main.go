package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/blindmaster24/MgkeTimetableBot/internal/preflight"
)

func main() {
	root := flag.String("root", "", "repository root holding deploy/monitoring (default: found from the working directory)")
	promtoolBin := flag.String("promtool", "", "promtool binary for the rule and unit-test checks (default: promtool on PATH)")
	skipPromtool := flag.Bool("skip-promtool", false, "do not run promtool and accept that the rules and their unit tests stay unchecked")
	asJSON := flag.Bool("json", false, "print the report as JSON")
	strict := flag.Bool("strict", false, "treat warnings and skipped checks as failures")
	flag.Parse()

	report := preflight.RunMonitoring(preflight.MonitoringOptions{
		Root:         *root,
		Promtool:     *promtoolBin,
		SkipPromtool: *skipPromtool,
	})

	if *asJSON {
		data, err := report.JSON()
		if err != nil {
			fmt.Fprintln(os.Stderr, "monitoringcheck:", err)
			os.Exit(1)
		}
		fmt.Println(string(data))
	} else {
		fmt.Print(report.Text())
	}

	if report.Failed() || report.Incomplete {
		os.Exit(1)
	}
	if *strict && report.Warnings() > 0 {
		os.Exit(1)
	}
	if *strict && report.Count(preflight.LevelSkip) > 0 {
		os.Exit(1)
	}
}
