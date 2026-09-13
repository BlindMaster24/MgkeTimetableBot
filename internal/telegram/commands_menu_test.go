package telegram

import (
	"regexp"
	"strings"
	"testing"
)

var telegramCommandPattern = regexp.MustCompile(`^[a-z0-9_]{1,32}$`)

func commandIsAdmin(cmd Command) bool {
	ac, ok := cmd.(AdminCommand)
	return ok && ac.AdminOnly()
}

func commandIsHidden(cmd Command) bool {
	hc, ok := cmd.(HiddenCommand)
	return ok && hc.Hidden()
}

func TestBotCommandsAreTelegramSafe(t *testing.T) {
	b := setupTestBot(t)

	for _, cmd := range b.botCommands(true) {
		if !telegramCommandPattern.MatchString(cmd.Command) {
			t.Errorf("command %q is not a valid Telegram command name", cmd.Command)
		}
		if cmd.Description == "" {
			t.Errorf("command %q has no description", cmd.Command)
		}
	}
}

func TestBotCommandsSkipAdminsInTheDefaultScope(t *testing.T) {
	b := setupTestBot(t)

	public := make(map[string]bool)
	for _, cmd := range b.botCommands(false) {
		public[cmd.Command] = true
	}

	expected := 0
	for _, cmd := range b.commandOrder {
		if commandIsHidden(cmd) || commandIsAdmin(cmd) {
			continue
		}
		expected++
		if !public[strings.ToLower(strings.TrimPrefix(cmd.Name(), "/"))] {
			t.Errorf("%s is missing from the default scope", cmd.Name())
		}
	}

	if len(public) != expected {
		t.Errorf("default scope = %d commands, expected %d", len(public), expected)
	}

	for _, cmd := range b.botCommands(true) {
		if public[cmd.Command] {
			continue
		}
		registered := b.commandByName(cmd.Command)
		if registered == nil || !commandIsAdmin(registered) {
			t.Errorf("%s is neither public nor an admin command", cmd.Command)
		}
	}
}

func TestBotCommandsMarkAdmins(t *testing.T) {
	b := setupTestBot(t)

	marked := 0
	for _, cmd := range b.botCommands(true) {
		registered := b.commandByName(cmd.Command)
		if registered == nil || !commandIsAdmin(registered) {
			if strings.HasPrefix(cmd.Description, "[адм] ") {
				t.Errorf("%s is not an admin command but carries the prefix", cmd.Command)
			}
			continue
		}

		marked++
		if !strings.HasPrefix(cmd.Description, "[адм] ") {
			t.Errorf("admin command %s lacks the [адм] prefix: %q", cmd.Command, cmd.Description)
		}
	}

	if marked == 0 {
		t.Fatal("expected at least one admin command")
	}
	if len(b.botCommands(true)) <= len(b.botCommands(false)) {
		t.Error("the admin scope must extend the default one")
	}
}

func TestBotCommandsAreDeduplicated(t *testing.T) {
	b := setupTestBot(t)

	seen := make(map[string]bool)
	for _, cmd := range b.botCommands(true) {
		if seen[cmd.Command] {
			t.Errorf("command %s is listed twice", cmd.Command)
		}
		seen[cmd.Command] = true
	}
}
