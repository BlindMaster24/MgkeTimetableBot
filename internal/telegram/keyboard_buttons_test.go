package telegram

import "testing"

func TestTheCalendarButtonsFollowTheChatToggles(t *testing.T) {
	b, _ := setupE2EBot(t)
	b.cfg.Google.OAuth.ClientID = "client-id"
	b.cfg.Calendar.ICS.Enabled = true

	chat := &Chat{Mode: ModeStudent, Group: "100", ShowDaily: true, ShowWeekly: true,
		ShowAbout: true, ShowCalls: true, ShowFastGroup: true, ShowFastTeacher: true,
		ShowGoogleCalendar: true, ShowICS: true}

	has := func(keyboard [][]string, label string) bool {
		for _, row := range keyboard {
			for _, button := range row {
				if button == label {
					return true
				}
			}
		}
		return false
	}
	labels := func(chat *Chat) [][]string {
		markup := replyMainMenu(b, chat)
		var rows [][]string
		for _, row := range markup.Keyboard {
			var texts []string
			for _, button := range row {
				texts = append(texts, button.Text)
			}
			rows = append(rows, texts)
		}
		return rows
	}

	if !has(labels(chat), b.loc("button_google_calendar")) || !has(labels(chat), b.loc("button_ics")) {
		t.Fatal("both calendar buttons must be there by default")
	}

	chat.ShowGoogleCalendar = false
	if has(labels(chat), b.loc("button_google_calendar")) {
		t.Error("the Google Calendar button must hide when the chat turned it off")
	}
	if !has(labels(chat), b.loc("button_ics")) {
		t.Error("hiding Google Calendar must keep ICS")
	}

	chat.ShowICS = false
	if has(labels(chat), b.loc("button_ics")) {
		t.Error("the ICS button must hide when the chat turned it off")
	}
}
