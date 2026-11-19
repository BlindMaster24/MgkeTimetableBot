package telegram

import (
	"regexp"
	"testing"
)

func TestSetMyCommandsSendsTheDefaultAndTheAdminMenus(t *testing.T) {
	b, caller, _, _ := setupArchiveBot(t, ModeStudent, "100", "")
	b.cfg.Telegram.AdminIDs = []int64{7001, 7002}

	if err := b.SetMyCommands(); err != nil {
		t.Fatalf("set commands: %v", err)
	}

	payloads := caller.commandPayloads()
	if len(payloads) != 3 {
		t.Fatalf("payloads = %d, want one default and two chat scopes", len(payloads))
	}
	if payloads[0].Scope != "default" {
		t.Fatalf("first scope = %q, want default", payloads[0].Scope)
	}
	if payloads[1].Scope != "chat" || payloads[2].Scope != "chat" {
		t.Fatalf("admin scopes = %q, %q, want chat", payloads[1].Scope, payloads[2].Scope)
	}

	name := regexp.MustCompile(`^[a-z0-9_]{1,32}$`)
	for index, payload := range payloads {
		if len(payload.Commands) == 0 {
			t.Fatalf("payload %d carries no commands", index)
		}
		for _, command := range payload.Commands {
			if !name.MatchString(command.Command) {
				t.Errorf("payload %d: command %q breaks the Bot API rules", index, command.Command)
			}
			if len(command.Description) < 3 || len(command.Description) > 256 {
				t.Errorf("payload %d: description %q is %d chars", index, command.Description, len(command.Description))
			}
		}
	}
}

func TestSetMyCommandsReportsARejectedMenu(t *testing.T) {
	b, caller, _, _ := setupArchiveBot(t, ModeStudent, "100", "")
	caller.failSetMyCommands = true

	if err := b.SetMyCommands(); err == nil {
		t.Fatal("a rejected menu must surface as an error")
	}
}
