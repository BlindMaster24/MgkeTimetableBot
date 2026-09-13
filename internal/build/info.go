package build

import (
	"fmt"
	"runtime"
	"strings"
	"time"
)

const (
	unknownVersion = "dev"
	unknownCommit  = "unknown"
	unknownDate    = "unknown"
)

type Info struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Date    string `json:"date"`
	Go      string `json:"go"`
	OS      string `json:"os"`
	Arch    string `json:"arch"`
}

func New(version, commit, date string) Info {
	return Info{
		Version: valueOr(version, unknownVersion),
		Commit:  valueOr(commit, unknownCommit),
		Date:    valueOr(date, unknownDate),
		Go:      runtime.Version(),
		OS:      runtime.GOOS,
		Arch:    runtime.GOARCH,
	}
}

func (i Info) ShortCommit() string {
	commit := strings.TrimSpace(i.Commit)
	if len(commit) <= 7 || commit == unknownCommit {
		return commit
	}
	return commit[:7]
}

func (i Info) Summary() string {
	return fmt.Sprintf("%s (%s, %s)", i.Version, i.ShortCommit(), i.Date)
}

func (i Info) BuiltAt() string {
	parsed, err := time.Parse(time.RFC3339, i.Date)
	if err != nil {
		return ""
	}
	return parsed.UTC().Format(time.RFC3339)
}

func valueOr(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
