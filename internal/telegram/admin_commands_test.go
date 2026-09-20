package telegram

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/archive"
	"github.com/blindmaster24/MgkeTimetableBot/internal/utils"
	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegoapi"
)

func TestTriggerRunsTheDayNotifications(t *testing.T) {
	const admin = int64(777)
	b, _ := setupE2EBot(t, admin)

	var triggered []int
	b.SetNoticeDayFunc(func(index int) { triggered = append(triggered, index) })

	caller := &capturingCaller{}
	b.client = botWithCaller(t, caller)

	u := makeUpdate(admin, "/trigger NextDayUpdater 3")
	u.Bot = b
	if err := (&triggerCmd{bot: b}).Handler(context.Background(), u); err != nil {
		t.Fatal(err)
	}

	if caller.last() != "ok" {
		t.Fatalf("the trigger must answer ok, got %q", caller.last())
	}
	if len(triggered) != 1 || triggered[0] != 2 {
		t.Fatalf("the trigger must run the day before the given pair, got %v", triggered)
	}

	caller.reset()
	u = makeUpdate(admin, "/trigger NextDayUpdater nope")
	u.Bot = b
	if err := (&triggerCmd{bot: b}).Handler(context.Background(), u); err != nil {
		t.Fatal(err)
	}
	if caller.last() != "index is not a number" {
		t.Fatalf("a bad index must be reported, got %q", caller.last())
	}

	caller.reset()
	u = makeUpdate(admin, "/trigger Unknown")
	u.Bot = b
	if err := (&triggerCmd{bot: b}).Handler(context.Background(), u); err != nil {
		t.Fatal(err)
	}
	if caller.last() != "not found" {
		t.Fatalf("an unknown trigger must be reported, got %q", caller.last())
	}
}

func TestTriggerIsAdminOnly(t *testing.T) {
	caller := &capturingCaller{}
	b, _ := setupE2EBotWithCaller(t, caller, 777)
	u := makeUpdate(4242, "/trigger NextDayUpdater 1")
	u.Bot = b
	if err := (&triggerCmd{bot: b}).Handler(context.Background(), u); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(caller.last(), "Доступ запрещён") {
		t.Fatalf("a stranger must be refused, got %q", caller.last())
	}
}

func TestFlushCachePushesEveryCachedDayIntoTheArchive(t *testing.T) {
	const admin = int64(777)
	b, archiveRepo := setupE2EBotWithArchive(t, admin)
	caller := &capturingCaller{}
	b.client = botWithCaller(t, caller)

	u := makeUpdate(admin, "/flushcache")
	u.Bot = b
	if err := (&flushCacheCmd{bot: b}).Handler(context.Background(), u); err != nil {
		t.Fatal(err)
	}

	texts := caller.texts()
	if len(texts) != 2 || texts[0] != "Сброс начат..." || texts[1] != "Сброс закончен" {
		t.Fatalf("the flush must report its start and finish, got %#v", texts)
	}

	if b.archive == nil {
		t.Fatal("the archive must be wired")
	}

	days, err := archiveRepo.GroupDays("100", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(days) != 2 {
		t.Fatalf("the cached days must reach the archive, got %d", len(days))
	}
	if days[0].Day != "31.08.2026" || days[1].Day != "01.09.2026" {
		t.Fatalf("the archived days must keep their order, got %q, %q", days[0].Day, days[1].Day)
	}

	teachers, err := archiveRepo.Teachers()
	if err != nil {
		t.Fatal(err)
	}
	if len(teachers) != 1 || teachers[0] != "Иванов И.И." {
		t.Fatalf("the cached teachers must reach the archive, got %#v", teachers)
	}
}

func TestFlushCacheRewritesTheSameDay(t *testing.T) {
	const admin = int64(777)
	b, repo := setupE2EBotWithArchive(t, admin)
	caller := &capturingCaller{}
	b.client = botWithCaller(t, caller)

	if err := repo.AppendDays([]archive.AppendDay{{
		Type:  "group",
		Value: "100",
		Day:   map[string]any{"day": "31.08.2026", "lessons": []any{map[string]any{"lesson": "Старое"}}},
	}}); err != nil {
		t.Fatal(err)
	}

	u := makeUpdate(admin, "/flushcache")
	u.Bot = b
	if err := (&flushCacheCmd{bot: b}).Handler(context.Background(), u); err != nil {
		t.Fatal(err)
	}

	days, err := repo.GroupDaysByRange(int64(utils.DayIndexFromDate(time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC))), int64(utils.DayIndexFromDate(time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC))), "100")
	if err != nil {
		t.Fatal(err)
	}
	if len(days) != 1 {
		t.Fatalf("the flushed day must stay single, got %d", len(days))
	}

	raw := days[0].Lessons
	if len(raw) != 2 {
		t.Fatalf("the flushed day must keep the cached pairs, got %d", len(raw))
	}

	var names []string
	for _, lesson := range raw {
		parsed, ok := lesson.(map[string]any)
		if !ok {
			t.Fatalf("the flushed pair has an unexpected shape: %#v", lesson)
		}
		name, _ := parsed["lesson"].(string)
		names = append(names, name)
	}
	if strings.Join(names, ",") != "Математика,Физика" {
		t.Fatalf("the flushed day must be rewritten from the cache, got %#v", names)
	}
}

func TestChatsAreAcceptedByDefault(t *testing.T) {
	_, repo := setupE2EBot(t)

	chat, err := repo.FindOrCreate("telegram", 4242)
	if err != nil {
		t.Fatal(err)
	}
	if !chat.Accepted {
		t.Fatal("a fresh chat must be accepted by default")
	}
}

func TestNewChatsWaitForAccessWhenItIsDisabled(t *testing.T) {
	caller := &capturingCaller{}
	b, _ := setupE2EBotWithCaller(t, caller)
	b.chatRepo.SetDefaultAccepted(false)

	u := makeUpdate(4242, "/day")
	u.Bot = b
	b.handleMessageText(context.Background(), u)

	text := caller.last()
	if !strings.Contains(text, "У вас нет доступа") || !strings.Contains(text, "4242") {
		t.Fatalf("the refused chat must get the access hint with its id, got %q", text)
	}

	chat, err := b.chatRepo.FindOrCreate("telegram", 4242)
	if err != nil {
		t.Fatal(err)
	}
	if chat.Accepted {
		t.Fatal("the chat must stay unaccepted")
	}
}

func TestAcceptBotGrantsAccess(t *testing.T) {
	const admin = int64(777)
	caller := &capturingCaller{}
	b, _ := setupE2EBotWithCaller(t, caller, admin)
	b.chatRepo.SetDefaultAccepted(false)

	if _, err := b.chatRepo.FindOrCreate("telegram", 4242); err != nil {
		t.Fatal(err)
	}

	u := makeUpdate(admin, "/acceptBot 4242")
	u.Bot = b
	if err := (&acceptBotCmd{bot: b}).Handler(context.Background(), u); err != nil {
		t.Fatal(err)
	}
	if caller.last() != "ok:4242" {
		t.Fatalf("the accept must be confirmed, got %q", caller.last())
	}

	chat, err := b.chatRepo.FindOrCreate("telegram", 4242)
	if err != nil {
		t.Fatal(err)
	}
	if !chat.Accepted {
		t.Fatal("the chat must be accepted after the command")
	}

	caller.reset()
	u = makeUpdate(admin, "/acceptBot 9999")
	u.Bot = b
	if err := (&acceptBotCmd{bot: b}).Handler(context.Background(), u); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(caller.last(), "не найден") {
		t.Fatalf("an unknown chat must be reported, got %q", caller.last())
	}

	caller.reset()
	u = makeUpdate(admin, "/acceptBot abc")
	u.Bot = b
	if err := (&acceptBotCmd{bot: b}).Handler(context.Background(), u); err != nil {
		t.Fatal(err)
	}
	if caller.last() != "это не число" {
		t.Fatalf("a bad id must be reported, got %q", caller.last())
	}
}

func TestAcceptedChatRunsCommands(t *testing.T) {
	caller := &capturingCaller{}
	b, _ := setupE2EBotWithCaller(t, caller)

	u := makeUpdate(4242, "/help")
	u.Bot = b
	b.handleMessageText(context.Background(), u)

	if !strings.Contains(caller.last(), "Список команд бота:") {
		t.Fatalf("an accepted chat must get the command output, got %q", caller.last())
	}
}

func setupE2EBotWithArchive(t *testing.T, adminIDs ...int64) (*Bot, *archive.Repository) {
	t.Helper()

	b, _ := setupE2EBot(t, adminIDs...)
	repo, err := archive.New(filepath.Join(t.TempDir(), "archive.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { repo.Close() })
	b.archive = repo

	return b, repo
}

func botWithCaller(t *testing.T, caller telegoapi.Caller) *telego.Bot {
	t.Helper()

	client, err := telego.NewBot("123456789:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", telego.WithAPICaller(caller), telego.WithDiscardLogger())
	if err != nil {
		t.Fatal(err)
	}
	return client
}
