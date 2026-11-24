package telegram

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	telego "github.com/mymmrac/telego"

	"github.com/blindmaster24/MgkeTimetableBot/internal/archive"
	"github.com/blindmaster24/MgkeTimetableBot/internal/utils"
)

func callbackFor(userID int64) *telego.CallbackQuery {
	return &telego.CallbackQuery{ID: "cb", From: telego.User{ID: userID}}
}

// weekDaysFor returns three dated days of the given week, starting on its
// Monday, so the view always has one passed day, today and one future day.
func weekDaysFor(week utils.WeekIndex, lessons int) []any {
	start, _ := week.WeekRange()
	days := make([]any, 0, 3)
	for offset := 0; offset < 3; offset++ {
		list := make([]any, 0, lessons)
		for i := 0; i < lessons; i++ {
			list = append(list, map[string]any{"lesson": "Предмет"})
		}
		days = append(days, map[string]any{
			"day":     start.AddDate(0, 0, offset).Format("02.01.2006"),
			"lessons": list,
		})
	}
	return days
}

// seedWeek replaces the helper's fixtures: the group and the teacher cache get
// the same Monday..Wednesday of the requested week, so the "relevant" week the
// bot clamps to is the one the test talks about.
func seedWeek(bot *Bot, week utils.WeekIndex) {
	bot.cache.Reset()
	days := weekDaysFor(week, 2)
	bot.cache.SetGroups(context.Background(), map[string]any{"167": map[string]any{"days": days}}, "h")
	bot.cache.SetTeachers(context.Background(), map[string]any{
		"Иванов И.И.": map[string]any{"days": days},
	}, "h")
}

// seedArchive puts the same Monday..Wednesday into the archive, because the
// timetable callback renders archive days rather than the cache.
func seedArchive(t *testing.T, bot *Bot, week utils.WeekIndex) {
	t.Helper()
	repo, err := archive.New(filepath.Join(t.TempDir(), "archive.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { repo.Close() })

	entries := make([]archive.AppendDay, 0, 6)
	for _, day := range weekDaysFor(week, 2) {
		entry, _ := day.(map[string]any)
		entries = append(entries,
			archive.AppendDay{Type: "group", Value: "167", Day: entry},
			archive.AppendDay{Type: "teacher", Value: "Иванов И.И.", Day: entry},
		)
	}
	if err := repo.AppendDays(context.Background(), entries); err != nil {
		t.Fatal(err)
	}
	bot.archive = repo
}

func TestStaleCallbackPayloadFollowsTheChatSetting(t *testing.T) {
	bot, caller, _, userID := setupArchiveBot(t, ModeStudent, "167", "")
	moment := time.Date(2026, 11, 24, 2, 50, 0, 0, time.Local)
	bot.SetNow(func() time.Time { return moment })
	week := utils.WeekIndexFromDate(moment)

	chat, err := bot.chatRepo.FindOrCreate("telegram", userID)
	if err != nil {
		t.Fatal(err)
	}
	chat.HidePastDays = true
	bot.saveChat(chat)
	seedWeek(bot, week)
	seedArchive(t, bot, week)
	u := &Update{UserID: userID, ChatID: userID, Bot: bot, Data: "timetable_g:167:" + itoa(week.Value()) + ":0:1"}
	u.Callback = callbackFor(userID)
	if err := bot.handleTimetableCb(context.Background(), u, chat, "group", u.Data); err != nil {
		t.Fatal(err)
	}

	text := caller.last()
	monday := weekDaysFor(week, 2)[0].(map[string]any)["day"].(string)
	if strings.Contains(text, "Понедельник") {
		t.Fatalf("a stale payload reopened %s:\n%s", monday, text)
	}
}

func TestShowAllButtonRestoresThePassedDays(t *testing.T) {
	bot, caller, _, userID := setupArchiveBot(t, ModeStudent, "167", "")
	moment := time.Date(2026, 11, 24, 2, 50, 0, 0, time.Local)
	bot.SetNow(func() time.Time { return moment })
	week := utils.WeekIndexFromDate(moment)

	chat, err := bot.chatRepo.FindOrCreate("telegram", userID)
	if err != nil {
		t.Fatal(err)
	}
	chat.HidePastDays = true
	bot.saveChat(chat)
	seedWeek(bot, week)
	seedArchive(t, bot, week)

	u := &Update{UserID: userID, ChatID: userID, Bot: bot, Data: "timetable_g:167:" + itoa(week.Value()) + ":a:1"}
	u.Callback = callbackFor(userID)
	if err := bot.handleTimetableCb(context.Background(), u, chat, "group", u.Data); err != nil {
		t.Fatal(err)
	}

	text := caller.last()
	monday := weekDaysFor(week, 2)[0].(map[string]any)["day"].(string)
	if !strings.Contains(text, "Понедельник") {
		t.Fatalf("the show-all button must keep %s:\n%s", monday, text)
	}
}

func TestWeekViewHidesPassedDaysForGroupsAndTeachers(t *testing.T) {
	moment := time.Date(2026, 11, 24, 2, 50, 0, 0, time.Local)
	week := utils.WeekIndexFromDate(moment)
	monday := weekDaysFor(week, 2)[0].(map[string]any)["day"].(string)

	tests := []struct {
		name  string
		setup func() (*Bot, *capturingCaller, *Chat)
	}{
		{"group", func() (*Bot, *capturingCaller, *Chat) {
			bot, caller, _, userID := setupArchiveBot(t, ModeStudent, "167", "")
			chat, _ := bot.chatRepo.FindOrCreate("telegram", userID)
			seedWeek(bot, week)
			return bot, caller, chat
		}},
		{"teacher", func() (*Bot, *capturingCaller, *Chat) {
			bot, caller, _, userID := setupArchiveBot(t, ModeTeacher, "", "Иванов И.И.")
			chat, _ := bot.chatRepo.FindOrCreate("telegram", userID)
			seedWeek(bot, week)
			return bot, caller, chat
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			bot, caller, chat := tc.setup()
			bot.SetNow(func() time.Time { return moment })
			chat.HidePastDays = true
			bot.saveChat(chat)

			u := &Update{UserID: chat.PeerID, ChatID: chat.PeerID, Bot: bot}
			if err := bot.showWeekSchedule(context.Background(), u, chat); err != nil {
				t.Fatal(err)
			}
			if text := caller.last(); strings.Contains(text, "Понедельник") {
				t.Fatalf("%s view kept the passed day %s:\n%s", tc.name, monday, text)
			}

			chat.HidePastDays = false
			bot.saveChat(chat)
			if err := bot.showWeekSchedule(context.Background(), u, chat); err != nil {
				t.Fatal(err)
			}
			if text := caller.last(); !strings.Contains(text, "Понедельник") {
				t.Fatalf("%s view must show the passed day when hiding is off:\n%s", tc.name, text)
			}
		})
	}
}

func TestWeekKeyboardCarriesTheShowAllMarker(t *testing.T) {
	bot, _, _, _ := setupArchiveBot(t, ModeStudent, "167", "")
	moment := time.Date(2026, 11, 24, 2, 50, 0, 0, time.Local)
	bot.SetNow(func() time.Time { return moment })
	week := utils.WeekIndexFromDate(moment)

	kb := bot.weekControlKeyboardHeader(context.Background(), "group", "167", week.Value(), true, true)
	var markers []string
	for _, row := range kb.InlineKeyboard {
		for _, button := range row {
			if strings.Contains(button.CallbackData, ":a:") {
				markers = append(markers, button.CallbackData)
			}
		}
	}
	if len(markers) != 1 || !strings.HasSuffix(markers[0], ":a:1") {
		t.Fatalf("the show-all button must carry the a marker, got %v", markers)
	}
}
