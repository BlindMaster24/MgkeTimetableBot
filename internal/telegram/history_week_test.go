package telegram

import "testing"

func TestParseWeekIndexRejectsImpossibleDates(t *testing.T) {
	bot := &Bot{}

	for _, text := range []string{
		"31.02.2026",
		"32.13.2026",
		"00.00.0000",
		"29.02.2025",
		"31.04",
		"0.0",
		"nonsense",
	} {
		if got := bot.parseWeekIndex(text); got != nil {
			t.Errorf("parseWeekIndex(%q) = %v, want nil", text, got.Value())
		}
	}
}

func TestParseWeekIndexAcceptsRealDates(t *testing.T) {
	bot := &Bot{}

	for _, text := range []string{"01.09.2026", "1.9.2026", "29.02.2024", "01.09"} {
		if got := bot.parseWeekIndex(text); got == nil {
			t.Errorf("parseWeekIndex(%q) = nil, want a week", text)
		}
	}
}
