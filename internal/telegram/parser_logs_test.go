package telegram

import (
	"context"
	"strings"
	"testing"
)

func TestParserLogsRefusesAStranger(t *testing.T) {
	caller := &capturingCaller{}
	b, _ := setupE2EBotWithCaller(t, caller, 424242)

	u := makeUpdate(1000, "/parserlogs")
	u.Bot = b
	if err := (&parserLogsCmd{bot: b}).Handler(context.Background(), u); err != nil {
		t.Fatal(err)
	}
	if got := caller.last(); got != b.loc("admcmd_6") {
		t.Fatalf("a stranger must be refused, got %q", got)
	}
}

func TestParserLogsSaysWhenThereIsNothingToShow(t *testing.T) {
	caller := &capturingCaller{}
	b, _ := setupE2EBotWithCaller(t, caller, 424242)

	u := makeUpdate(424242, "/parserlogs")
	u.Bot = b
	if err := (&parserLogsCmd{bot: b}).Handler(context.Background(), u); err != nil {
		t.Fatal(err)
	}
	if got := caller.last(); got != b.loc("parser_logs_no_logs") {
		t.Fatalf("an empty log must say so, got %q", got)
	}
}

func TestParserLogsListsSuccessesAndFailures(t *testing.T) {
	caller := &capturingCaller{}
	b, _ := setupE2EBotWithCaller(t, caller, 424242)

	b.AddParseLog(true, "группы обновлены")
	b.AddParseLog(false, "преподаватели упали")

	u := makeUpdate(424242, "/getUpdaterLogs")
	u.Bot = b
	if err := (&parserLogsCmd{bot: b}).Handler(context.Background(), u); err != nil {
		t.Fatal(err)
	}

	text := caller.last()
	if !strings.Contains(text, "✅") || !strings.Contains(text, "группы обновлены") {
		t.Fatalf("a success line is missing: %q", text)
	}
	if !strings.Contains(text, "❌") || !strings.Contains(text, "преподаватели упали") {
		t.Fatalf("a failure line is missing: %q", text)
	}
}

func TestParserLogsKeepsTheNewestTailOfALongAnswer(t *testing.T) {
	caller := &capturingCaller{}
	b, _ := setupE2EBotWithCaller(t, caller, 424242)

	b.AddParseLog(true, strings.Repeat("старое ", 700))
	b.AddParseLog(true, "последняя строка")

	u := makeUpdate(424242, "/parserlogs")
	u.Bot = b
	if err := (&parserLogsCmd{bot: b}).Handler(context.Background(), u); err != nil {
		t.Fatal(err)
	}

	text := caller.last()
	if len(text) > 4096 {
		t.Fatalf("the answer must fit one message, got %d characters", len(text))
	}
	if !strings.Contains(text, "последняя строка") {
		t.Fatal("the tail must keep the newest line")
	}
}
