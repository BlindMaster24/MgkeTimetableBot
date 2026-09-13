package telegram

import (
	"context"
	"strings"
	"testing"

	"github.com/blindmaster24/MgkeTimetableBot/internal/cache"
)

func withCampusVariants(t *testing.T, b *Bot) {
	t.Helper()

	b.cache.SetCallsSiteVariants([]cache.CallsVariant{
		{
			Name: "Казинца",
			Schedule: cache.CallsSchedule{
				Weekdays: [][2][2]string{{{"08:00", "08:45"}, {"08:55", "09:40"}}},
				Saturday: [][2][2]string{{{"09:00", "09:45"}, {"09:55", "10:40"}}},
			},
		},
		{
			Name: "Кнорина",
			Schedule: cache.CallsSchedule{
				Weekdays: [][2][2]string{{{"09:00", "09:45"}, {"09:55", "10:40"}}},
				Saturday: [][2][2]string{{{"09:00", "09:45"}, {"09:55", "10:40"}}},
			},
		},
	})
}

func containsText(rows [][]string, text string) bool {
	for _, row := range rows {
		for _, cell := range row {
			if cell == text {
				return true
			}
		}
	}
	return false
}

func TestCallsScheduleFollowsTheChatCampus(t *testing.T) {
	b, _, c := setupTestBotWithData(t)
	withCampusVariants(t, b)

	c.SetCallsNotify(cache.Schedule{
		Weekdays: [][2][2]string{{{"08:00", "08:45"}, {"08:55", "09:40"}}},
		Saturday: [][2][2]string{{{"08:00", "08:45"}, {"08:55", "09:40"}}},
	}, cache.Schedule{}, "site", "")

	defaultChat := &Chat{}
	if got := b.callsScheduleFor(defaultChat).Weekdays[0][0][0]; got != "08:00" {
		t.Errorf("default campus start = %s", got)
	}

	knorina := &Chat{CallsCampus: "Кнорина"}
	if got := b.callsScheduleFor(knorina).Weekdays[0][0][0]; got != "09:00" {
		t.Errorf("Кнорина start = %s", got)
	}

	unknown := &Chat{CallsCampus: "Нет такого"}
	if got := b.callsScheduleFor(unknown).Weekdays[0][0][0]; got != "08:00" {
		t.Errorf("an unknown campus must fall back to the site schedule, got %s", got)
	}
}

func TestCallsScheduleIgnoresCampusForManualSource(t *testing.T) {
	b, _, c := setupTestBotWithData(t)
	withCampusVariants(t, b)

	calls := c.GetCalls()
	calls.Manual = cache.CallsSource{
		Schedule:  cache.CallsSchedule{Weekdays: [][2][2]string{{{"10:00", "10:45"}, {"10:55", "11:40"}}}},
		UpdatedAt: 1,
	}
	calls.Active = cache.CallsActive{
		Source:   "manual",
		Schedule: calls.Manual.Schedule,
	}
	c.SetCallsFromCache(calls)

	chat := &Chat{CallsCampus: "Кнорина"}
	if got := b.callsScheduleFor(chat).Weekdays[0][0][0]; got != "10:00" {
		t.Errorf("manual source must win over the campus, got %s", got)
	}
}

func TestCallsCampusKeyboardOffersEveryCampus(t *testing.T) {
	b, _, _ := setupTestBotWithData(t)

	plain := replyRows(b.replyCallsSettings(&Chat{}, false))
	if containsText(plain, campusButtonPrefix+"Казинца") {
		t.Error("no campus buttons before the site reports several schedules")
	}

	withCampusVariants(t, b)
	rows := replyRows(b.replyCallsSettings(&Chat{CallsCampus: "Кнорина"}, false))

	if !containsText(rows, "✅ "+campusButtonPrefix+"Кнорина") {
		t.Errorf("the selected campus must be checked: %v", rows)
	}
	if !containsText(rows, campusButtonPrefix+"Казинца") {
		t.Errorf("the other campus must be offered: %v", rows)
	}
	if !containsText(rows, campusButtonPrefix+campusAutoLabel) {
		t.Errorf("an explicit choice must be resettable: %v", rows)
	}
}

func TestCallsCampusTextCommandStoresTheChoice(t *testing.T) {
	b, repo, _ := setupTestBotWithData(t)
	withCampusVariants(t, b)

	handler := &callsCampusTextCmd{bot: b}
	if !handler.MatchText(campusButtonText("Кнорина", false)) {
		t.Fatal("the campus button must be matched")
	}
	if !handler.MatchText(campusButtonText("Кнорина", true)) {
		t.Fatal("the checked campus button must be matched")
	}
	if handler.MatchText("Источник: сайт") {
		t.Fatal("other calls buttons must not match")
	}

	chat, err := repo.FindOrCreate("telegram", 777)
	if err != nil {
		t.Fatal(err)
	}

	if label := campusButtonLabel(campusButtonText("Кнорина", true)); label != "Кнорина" {
		t.Errorf("label = %q", label)
	}

	chat.CallsCampus = "Кнорина"
	if err := repo.Save(chat); err != nil {
		t.Fatal(err)
	}

	stored, err := repo.FindOrCreate("telegram", 777)
	if err != nil {
		t.Fatal(err)
	}
	if stored.CallsCampus != "Кнорина" {
		t.Errorf("campus was not persisted: %q", stored.CallsCampus)
	}
}

func TestE2E_CampusButtonStoresTheChoice(t *testing.T) {
	b, repo := setupE2EBot(t, 999)
	withCampusVariants(t, b)

	userID := int64(4100)
	open := makeUpdate(userID, "🕐 Звонки: управление")
	open.Bot = b
	b.handleMessageText(context.Background(), open)

	saved, _ := repo.FindOrCreate("telegram", userID)
	if saved.Scene != sceneSettingsCalls {
		t.Fatalf("scene = %q", saved.Scene)
	}

	u := makeUpdate(userID, campusButtonText("Кнорина", false))
	u.Bot = b
	b.handleMessageText(context.Background(), u)

	saved, _ = repo.FindOrCreate("telegram", userID)
	if saved.CallsCampus != "Кнорина" {
		t.Errorf("campus was not saved: %q", saved.CallsCampus)
	}

	auto := makeUpdate(userID, campusButtonPrefix+campusAutoLabel)
	auto.Bot = b
	b.handleMessageText(context.Background(), auto)

	saved, _ = repo.FindOrCreate("telegram", userID)
	if saved.CallsCampus != "" {
		t.Errorf("the auto entry must reset the campus, got %q", saved.CallsCampus)
	}
}

func TestCallsCampusConfirmationMentionsTheCampus(t *testing.T) {
	b, _, _ := setupTestBotWithData(t)
	withCampusVariants(t, b)

	text := b.campusConfirmation(&Chat{CallsCampus: "Кнорина"})
	if !strings.Contains(text, "Кнорина") {
		t.Errorf("confirmation = %q", text)
	}
	if !strings.Contains(strings.ToLower(b.callsMenuText(&Chat{CallsCampus: "Кнорина"}, false)), "кнорина") {
		t.Error("the calls menu should mention the selected campus")
	}
}
