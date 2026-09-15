package telegram

import "testing"

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
