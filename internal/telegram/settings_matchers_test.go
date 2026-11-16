package telegram

import (
	"strings"
	"testing"

	"github.com/blindmaster24/MgkeTimetableBot/internal/formatter"
)

func TestBtnToggleTextMatcherAcceptsOldVariants(t *testing.T) {
	cmd := &btnToggleTextCmd{kind: "about"}
	for _, text := range []string{
		`✅ Кнопка "💡 О боте"`,
		`🚫 Кнопка "💡 О боте"`,
		`✅ Кнопка "О боте"`,
		`🚫 Кнопка "О боте"`,
		`✅ Кнопка "о боте"`,
	} {
		if !cmd.MatchText(text) {
			t.Errorf("MatchText(%q) = false", text)
		}
	}
	for _, text := range []string{
		`Кнопка "О боте"`,
		`✅ Кнопка "О боте" extra`,
		`✅ О боте`,
		`✅ Кнопка "💡 О боте" `,
	} {
		if cmd.MatchText(text) {
			t.Errorf("MatchText(%q) = true", text)
		}
	}
}

func TestFormatterSelectMatcherIgnoresCase(t *testing.T) {
	b, _, _ := setupTestBotWithData(t)
	cmd := &formatterSelectTextCmd{bot: b}
	label := formatter.AllFormatters[0].Label(b.fmtOpts(&Chat{}, true))
	upper := strings.ToUpper(label)
	if !cmd.MatchText(upper) {
		t.Errorf("MatchText(%q) = false", upper)
	}
	if !cmd.MatchText(upper + " (выбран)") {
		t.Errorf("MatchText(%q) = false", upper+" (выбран)")
	}
	if cmd.MatchText(upper + " лишнее") {
		t.Error("extra words must not match")
	}
}
