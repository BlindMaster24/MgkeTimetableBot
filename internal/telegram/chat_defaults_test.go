package telegram

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/mymmrac/telego"
)

func TestRepositoryNewChatShowsMainButtons(t *testing.T) {
	repo, err := New(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	chat, err := repo.FindOrCreate("telegram", 555)
	if err != nil {
		t.Fatalf("find or create: %v", err)
	}

	if !chat.ShowDaily || !chat.ShowWeekly || !chat.ShowCalls {
		t.Errorf("a new chat must show the schedule buttons: %+v", chat)
	}
	if !chat.ShowAbout || !chat.ShowFastGroup || !chat.ShowFastTeacher {
		t.Errorf("a new chat must show the remaining main menu buttons: %+v", chat)
	}
}

func TestRepositoryEnablesButtonsOfChatsDisabledByTheOldDefault(t *testing.T) {
	path := t.TempDir() + "/test.db"

	repo, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.FindOrCreate("telegram", 700); err != nil {
		t.Fatalf("find or create: %v", err)
	}
	if _, err := repo.db.Exec(`UPDATE bot_chats SET
			show_about = 0, show_daily = 0, show_weekly = 0,
			show_calls = 0, show_fast_group = 0, show_fast_teacher = 0`); err != nil {
		t.Fatalf("disable buttons: %v", err)
	}
	if _, err := repo.db.Exec(`DELETE FROM bot_state WHERE key = ?`, "migrate.show_buttons_default"); err != nil {
		t.Fatalf("drop migration marker: %v", err)
	}
	repo.Close()

	reopened, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()

	chat, err := reopened.FindOrCreate("telegram", 700)
	if err != nil {
		t.Fatalf("find or create: %v", err)
	}
	if !chat.ShowDaily || !chat.ShowWeekly || !chat.ShowCalls || !chat.ShowAbout || !chat.ShowFastGroup || !chat.ShowFastTeacher {
		t.Errorf("the buttons of a chat that never customized them must be enabled: %+v", chat)
	}
}

func TestRepositoryKeepsCustomizedButtons(t *testing.T) {
	path := t.TempDir() + "/test.db"

	repo, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.FindOrCreate("telegram", 800); err != nil {
		t.Fatalf("find or create: %v", err)
	}
	if _, err := repo.db.Exec(`UPDATE bot_chats SET show_about = 0`); err != nil {
		t.Fatalf("customize buttons: %v", err)
	}
	if _, err := repo.db.Exec(`DELETE FROM bot_state WHERE key = ?`, "migrate.show_buttons_default"); err != nil {
		t.Fatalf("drop migration marker: %v", err)
	}
	repo.Close()

	reopened, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()

	chat, err := reopened.FindOrCreate("telegram", 800)
	if err != nil {
		t.Fatalf("find or create: %v", err)
	}
	if chat.ShowAbout {
		t.Error("a disabled button must stay disabled")
	}
	if !chat.ShowDaily {
		t.Error("the other buttons must stay enabled")
	}
}

func TestANewChatHidesPassedDaysLikeTheOldBot(t *testing.T) {
	repo, err := New(filepath.Join(t.TempDir(), "chats.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { repo.Close() })

	chat, err := repo.FindOrCreate("telegram", 777001)
	if err != nil {
		t.Fatal(err)
	}
	if !chat.HidePastDays {
		t.Fatal("a new chat must hide the passed days, the old bot did")
	}
}

func TestScheduleMessageCleanupFollowsTheChatSettings(t *testing.T) {
	b, caller, _, userID := setupArchiveBot(t, ModeStudent, "100", "")

	chat, err := b.chatRepo.FindOrCreate("telegram", userID)
	if err != nil {
		t.Fatal(err)
	}
	chat.DeleteLastMsg = true
	chat.DeleteUserMsg = true
	chat.LastMsgID = 55
	if err := b.chatRepo.Save(chat); err != nil {
		t.Fatal(err)
	}

	u := makeUpdate(userID, "/day")
	u.Bot = b
	u.Message = &telego.Message{MessageID: 900, Chat: telego.Chat{ID: userID}, From: &telego.User{ID: userID}}
	b.handleMessageText(context.Background(), u)

	deleted := caller.deletedMessages()
	if len(deleted) != 2 || deleted[0] != 55 || deleted[1] != 900 {
		t.Fatalf("deleted = %v, want the previous bot message and the user message", deleted)
	}

	saved, err := b.chatRepo.FindOrCreate("telegram", userID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.LastMsgID == 55 || saved.LastMsgID == 0 {
		t.Fatalf("LastMsgID = %d, want the id of the fresh timetable message", saved.LastMsgID)
	}
}

func TestViewTogglesRememberTheMessageCleanup(t *testing.T) {
	b, _, _, userID := setupArchiveBot(t, ModeStudent, "100", "")
	chat, err := b.chatRepo.FindOrCreate("telegram", userID)
	if err != nil {
		t.Fatal(err)
	}
	chat.Scene = sceneSettings
	if err := b.chatRepo.Save(chat); err != nil {
		t.Fatal(err)
	}

	toggle := &viewToggleTextCmd{bot: b, kind: "delete_last_msg"}
	if !toggle.MatchText(b.noYesSmile(true, b.loc("kbtext_53"))) {
		t.Fatal("the toggle must match its own button label")
	}

	u := makeUpdate(userID, b.noYesSmile(true, b.loc("kbtext_53")))
	u.Bot = b
	if err := toggle.Handler(context.Background(), u); err != nil {
		t.Fatal(err)
	}

	saved, err := b.chatRepo.FindOrCreate("telegram", userID)
	if err != nil {
		t.Fatal(err)
	}
	if !saved.DeleteLastMsg {
		t.Fatal("the toggle must switch DeleteLastMsg on")
	}
}
