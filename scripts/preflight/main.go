package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/preflight"
)

func main() {
	configPath := flag.String("config", "configs/config.yaml", "path to the bot config to check")
	groupsURL := flag.String("groups-url", "", "override parser.endpoints.timetable_group")
	teachersURL := flag.String("teachers-url", "", "override parser.endpoints.timetable_teacher")
	callsURL := flag.String("calls-url", "", "override parser.endpoints.bell_schedule")
	skipSite := flag.Bool("skip-site", false, "check only local state: config, keys, locale, storage and the cached data")
	skipImage := flag.Bool("skip-image", false, "do not render a timetable image")
	imageDir := flag.String("image-dir", "", "keep the checked PNG in this directory instead of a temporary one")
	asJSON := flag.Bool("json", false, "print the report as JSON")
	strict := flag.Bool("strict", false, "treat warnings as failures")
	timeout := flag.Duration("timeout", 45*time.Second, "per request timeout for the live site checks")
	flag.Parse()

	report := preflight.Run(preflight.Options{
		ConfigPath: *configPath,
		Endpoints: preflight.Endpoints{
			Groups:   *groupsURL,
			Teachers: *teachersURL,
			Calls:    *callsURL,
		},
		HTTPClient: &http.Client{Timeout: *timeout},
		ImageDir:   *imageDir,
		SkipSite:   *skipSite,
		SkipImage:  *skipImage,
	})

	if *asJSON {
		data, err := report.JSON()
		if err != nil {
			fmt.Fprintln(os.Stderr, "preflight:", err)
			os.Exit(1)
		}
		fmt.Println(string(data))
	} else {
		fmt.Print(report.Text())
	}

	if report.Failed() {
		os.Exit(1)
	}
	if *strict && report.Warnings() > 0 {
		os.Exit(1)
	}
}
