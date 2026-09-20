package telegram

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestSurfaceDump(t *testing.T) {
	path := os.Getenv("SURFACE_DUMP")
	if path == "" {
		t.Skip("set SURFACE_DUMP to a file path to dump the command surface")
	}

	b, _ := setupE2EBot(t, 999)

	var out strings.Builder
	for _, cmd := range b.commandOrder {
		hidden := false
		if h, ok := cmd.(HiddenCommand); ok && h.Hidden() {
			hidden = true
		}
		admin := false
		if a, ok := cmd.(AdminCommand); ok && a.AdminOnly() {
			admin = true
		}
		text := "callback"
		if _, ok := cmd.(TextMatcher); ok {
			text = "text"
		}
		fmt.Fprintf(&out, "%s\t%s\thidden=%v\tadmin=%v\t%s\n", cmd.Name(), cmd.Description(), hidden, admin, text)
	}

	if err := os.WriteFile(path, []byte(out.String()), 0o644); err != nil {
		t.Fatalf("write dump: %v", err)
	}
}
