package parity

import (
	"regexp"
	"sort"
	"strings"
)

type Surface struct {
	Commands  []string `json:"commands"`
	Callbacks []string `json:"callbacks"`
	Buttons   []string `json:"buttons"`
	Texts     []string `json:"texts,omitempty"`
}

type Decision struct {
	Section string `json:"section"`
	Kind    string `json:"kind"`
	Value   string `json:"value"`
	Reason  string `json:"reason"`
}

type Allowlist struct {
	Decisions []Decision `json:"decisions"`
}

func (a Allowlist) allows(section, kind, value string) bool {
	for _, decision := range a.Decisions {
		if decision.Section == section && decision.Kind == kind && decision.Value == value {
			return true
		}
	}
	return false
}

type Diff struct {
	Section string
	Kind    string
	Value   string
}

const (
	KindMissingInGo = "missing_in_go"
	KindExtraInGo   = "extra_in_go"
)

const (
	SectionCommands  = "commands"
	SectionCallbacks = "callbacks"
	SectionButtons   = "buttons"
)

var (
	templateExpr   = regexp.MustCompile(`\$\{[^}]*\}`)
	formatVerbExpr = regexp.MustCompile(`%[a-zA-Z]`)
	digitsExpr     = regexp.MustCompile(`\d+`)
	spacesExpr     = regexp.MustCompile(`\s+`)
	markerExpr     = regexp.MustCompile(`[\p{L}\p{N}\p{So}\p{Sk}]`)
)

var markerPrefixes = []string{"✅", "🚫", "🔈", "🔇", "❌"}

func NormalizeButton(text string) string {
	text = templateExpr.ReplaceAllString(text, "")
	text = formatVerbExpr.ReplaceAllString(text, "")
	text = digitsExpr.ReplaceAllString(text, "")
	text = spacesExpr.ReplaceAllString(text, " ")
	text = strings.TrimSpace(text)
	for {
		trimmed := false
		for _, marker := range markerPrefixes {
			if strings.HasPrefix(text, marker) {
				text = strings.TrimSpace(strings.TrimPrefix(text, marker))
				trimmed = true
			}
		}
		if !trimmed {
			break
		}
	}
	text = strings.TrimSuffix(strings.TrimSpace(text), "(выбран)")
	text = strings.TrimSpace(text)
	for _, suffix := range []string{": Да", ": Нет"} {
		text = strings.TrimSuffix(text, suffix)
	}
	text = strings.TrimSuffix(strings.TrimSpace(text), ":")
	return strings.TrimSpace(spacesExpr.ReplaceAllString(text, " "))
}

func IsButtonLabel(text string) bool {
	return markerExpr.MatchString(NormalizeButton(text))
}

func normalizeSurface(s Surface) Surface {
	return Surface{
		Commands:  normalizeCommands(s.Commands),
		Callbacks: normalizeCallbacks(s.Callbacks),
		Buttons:   normalizeButtons(s.Buttons),
		Texts:     normalizeTexts(s.Texts),
	}
}

func NormalizeCommand(name string) string {
	return strings.ToLower(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(name), "/")))
}

func NormalizeCallback(prefix string) string {
	prefix = strings.TrimSpace(prefix)
	if idx := strings.IndexAny(prefix, ":_"); idx >= 0 {
		prefix = prefix[:idx]
	}
	return prefix
}

func normalizeCommands(values []string) []string {
	return normalizeWith(values, NormalizeCommand)
}

func normalizeCallbacks(values []string) []string {
	return normalizeWith(values, NormalizeCallback)
}

func normalizeWith(values []string, normalize func(string) string) []string {
	seen := map[string]bool{}
	var out []string
	for _, v := range values {
		if normalize != nil {
			v = normalize(v)
		}
		v = strings.TrimSpace(v)
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

func normalizeButtons(values []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, v := range values {
		v = NormalizeButton(v)
		if v == "" || !IsButtonLabel(v) || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

func Compare(ts, got Surface, allow Allowlist) []Diff {
	ts = normalizeSurface(ts)
	got = normalizeSurface(got)

	var diffs []Diff
	diffs = append(diffs, compareSets(SectionCommands, ts.Commands, got.Commands, allow)...)
	diffs = append(diffs, compareSets(SectionCallbacks, ts.Callbacks, got.Callbacks, allow)...)
	diffs = append(diffs, compareSets(SectionButtons, ts.Buttons, got.Buttons, allow)...)
	return diffs
}

func compareSets(section string, ts, got []string, allow Allowlist) []Diff {
	var diffs []Diff
	for _, value := range ts {
		if contains(got, value) || allow.allows(section, KindMissingInGo, value) {
			continue
		}
		diffs = append(diffs, Diff{Section: section, Kind: KindMissingInGo, Value: value})
	}
	for _, value := range got {
		if contains(ts, value) || allow.allows(section, KindExtraInGo, value) {
			continue
		}
		diffs = append(diffs, Diff{Section: section, Kind: KindExtraInGo, Value: value})
	}
	return diffs
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
