package telegram

import (
	"strings"
	"testing"
)

// dangerousCommands touch the host (restart, exit) or the network on purpose;
// the smoke test sends their plain name and cannot undo that, so they are out.
var dangerousCommands = map[string]bool{
	"/restart":                    true,
	"/dev":                        true,
	"/trigger":                    true,
	"/vanish":                     true,
	"/backup":                     true,
	"/vychetkaDlyaBrovkiDSOnline": true,
	"/error":                      true,
}

// TestEveryCommandSurvivesAnEmptyCache sends each registered command to a fresh
// bot whose cache is empty: a missing timetable must be a hint, never a panic.
func TestEveryCommandSurvivesAnEmptyCache(t *testing.T) {
	const admin = int64(777)

	caller := &capturingCaller{}
	b, _ := setupE2EBotWithCaller(t, caller, admin)
	b.GetRaspCache().Reset()

	for name, command := range b.commands {
		if dangerousCommands[strings.ToLower(name)] {
			continue
		}
		t.Run(name, func(t *testing.T) {
			u := makeUpdate(admin, name)
			u.Bot = b
			if err := command.Handler(t.Context(), u); err != nil {
				t.Fatalf("%s returned %v", name, err)
			}
		})
	}
}
