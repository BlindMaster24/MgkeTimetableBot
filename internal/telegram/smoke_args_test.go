package telegram

import (
	"strings"
	"testing"
)

// hostileArguments are the shapes that used to break string handling: markup,
// format verbs, template actions, huge numbers, invalid dates and long input.
func hostileArguments() []string {
	return []string{
		"", " ", "\t", "<b>", "</b>", "&amp;", "%s", "%s%s", "{{.X}}",
		"999999999999999999999999", "0", "-1", "31.02.2026", "10:99-25:61",
		"👍", strings.Repeat("я", 3000), "' OR 1=1 --", "/etc/passwd", "../..",
	}
}

// TestEveryCommandSurvivesHostileArguments feeds the arguments through every
// command that only reads state. Admin commands that broadcast or touch the host
// are excluded for the same reason as in TestEveryCommandSurvivesAnEmptyCache.
func TestEveryCommandSurvivesHostileArguments(t *testing.T) {
	const admin = int64(777)

	skip := map[string]bool{"/send": true, "/sql": true, "/db": true}
	for name := range dangerousCommands {
		skip[strings.ToLower(name)] = true
	}

	caller := &capturingCaller{}
	b, _ := setupE2EBotWithCaller(t, caller, admin)
	b.GetRaspCache().Reset()

	for name, command := range b.commands {
		if skip[strings.ToLower(name)] {
			continue
		}
		for _, arg := range hostileArguments() {
			u := makeUpdate(admin, strings.TrimSpace(name+" "+arg))
			u.Bot = b
			if err := command.Handler(t.Context(), u); err != nil {
				t.Fatalf("%s with %q returned %v", name, arg, err)
			}
		}
	}
}
