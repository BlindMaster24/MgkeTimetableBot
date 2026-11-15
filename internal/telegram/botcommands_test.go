package telegram

import (
	"strings"
	"testing"

	"github.com/mymmrac/telego"
)

func TestValidateBotCommandsAcceptsTheRegisteredSurface(t *testing.T) {
	commands := make([]telego.BotCommand, 0, len(SurfaceOfCommands()))
	for _, name := range SurfaceOfCommands() {
		commands = append(commands, telego.BotCommand{
			Command:     strings.ToLower(strings.TrimPrefix(name, "/")),
			Description: "расписание",
		})
	}
	if len(commands) == 0 {
		t.Fatal("the bot registered no commands")
	}
	if err := validateBotCommands(commands); err != nil {
		t.Fatalf("a registered command would make setMyCommands fail: %v", err)
	}
}

func TestValidateBotCommandsRejectsWhatTelegramRejects(t *testing.T) {
	cases := []struct {
		name        string
		command     string
		description string
	}{
		{"empty name", "", "расписание"},
		{"uppercase", "Day", "расписание"},
		{"dash", "day-time", "расписание"},
		{"space", "day time", "расписание"},
		{"too long", strings.Repeat("a", 33), "расписание"},
		{"short description", "day", "ok"},
		{"long description", "day", strings.Repeat("я", 257)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateBotCommands([]telego.BotCommand{{Command: tc.command, Description: tc.description}})
			if err == nil {
				t.Fatalf("telegram rejects %q / %q, the validator accepted it", tc.command, tc.description)
			}
		})
	}
}
