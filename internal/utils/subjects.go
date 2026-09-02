package utils

import (
	_ "embed"
	"strings"
)

const bom = "\uFEFF"

//go:embed subjects.csv
var subjectsCSV string

var (
	subjectsByShort = map[string]string{}
	subjectsByFull  = map[string]string{}
)

func init() {
	for _, line := range strings.Split(subjectsCSV, "\n") {
		line = strings.TrimSpace(line)
		line = strings.TrimPrefix(line, bom)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ";", 2)
		if len(parts) != 2 {
			continue
		}
		full := strings.TrimSpace(parts[0])
		short := strings.TrimSpace(parts[1])
		subjectsByShort[short] = full
		subjectsByFull[full] = short
	}
}

func GetFullSubjectName(subject string) string {
	if full, ok := subjectsByShort[subject]; ok {
		return full
	}
	return subject
}

func GetShortSubjectName(subject string) string {
	if short, ok := subjectsByFull[subject]; ok {
		return short
	}
	return subject
}
