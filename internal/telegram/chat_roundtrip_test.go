package telegram

import (
	"reflect"
	"testing"
)

// TestChatRoundTripKeepsEveryColumn guards the storage contract: a setting the
// settings screen can change must survive Save and come back unchanged.
func TestChatRoundTripKeepsEveryColumn(t *testing.T) {
	repo, err := New(t.TempDir() + "/chats.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { repo.Close() })

	chat, err := repo.FindOrCreate("telegram", 6409603928)
	if err != nil {
		t.Fatal(err)
	}
	chat.Accepted = true
	chat.EULA = true
	chat.Scene = "settings"
	chat.Mode = ModeParent
	chat.Group = "167"
	chat.Teacher = "Иванов И.И."
	chat.GoogleEmail = "user@example.test"
	chat.Formatter = 2
	chat.ShowAbout = false
	chat.ShowDaily = false
	chat.ShowWeekly = false
	chat.ShowCalls = false
	chat.ShowFastGroup = false
	chat.ShowFastTeacher = false
	chat.ShowGoogleCalendar = true
	chat.ShowICS = true
	chat.HidePastDays = false
	chat.DeleteLastMsg = true
	chat.DeleteUserMsg = true
	chat.LastMsgID = 4242
	chat.AllowSendMess = false
	chat.NoticeChanges = false
	chat.NoticeNextWeek = false
	chat.NoticeCalls = false
	chat.NoticeParserErrors = false
	chat.ReminderLeadMinutes = 30
	chat.ShowParserTime = true
	chat.ShowHints = false
	chat.DiffEnabled = false
	chat.DiffAutoInWeek = false
	chat.DiffAutoInUpdates = false
	chat.DiffShowBeforeAfter = false
	chat.DiffMaxLines = 40
	chat.LastMsgTime = 1763700000000
	chat.SubscribeDistribution = false
	chat.NeedUpdateButtons = true
	chat.DeactivateSecondaryCheck = true
	chat.CallsEditInput = "10:00-11:30"
	chat.CallsEditReason = "замена"
	chat.TextFormat = "compact"
	chat.CallsCampus = "Кн"
	chat.Ref = "ref-1"
	chat.HistoryGroup = []string{"167", "106"}
	chat.HistoryTeacher = []string{"Иванов И.И."}

	if err := repo.Save(chat); err != nil {
		t.Fatalf("Save: %v", err)
	}

	reloaded, err := repo.FindOrCreate("telegram", 6409603928)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(chat, reloaded) {
		t.Fatalf("a saved chat came back different:\n saved: %+v\nloaded: %+v", chat, reloaded)
	}
}
