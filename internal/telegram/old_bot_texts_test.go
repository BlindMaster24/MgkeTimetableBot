package telegram

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/apikey"
	"github.com/blindmaster24/MgkeTimetableBot/internal/utils"
)

func deliveredTexts(t *testing.T, caller *recordingCaller, min int) []string {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for {
		calls := recordedCalls(caller, "sendMessage")
		texts := make([]string, 0, len(calls))
		for _, call := range calls {
			if text, ok := call["text"].(string); ok {
				texts = append(texts, text)
			}
		}
		if len(texts) >= min || time.Now().After(deadline) {
			return texts
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func sendOldText(t *testing.T, b *Bot, userID int64, text string) {
	t.Helper()

	u := &Update{Bot: b, ChatID: userID, UserID: userID, Text: text}
	b.handleMessageText(context.Background(), u)
}

func TestOldBot_DevInvitesToTheRepository(t *testing.T) {
	caller := &recordingCaller{}
	b, _ := setupE2EBotWithCaller(t, caller, 4242)

	sendOldText(t, b, 4242, "/dev")

	texts := deliveredTexts(t, caller, 1)
	last := texts[len(texts)-1]
	if !strings.Contains(last, "Хочешь помочь сделать бота лучше? Окей, бегом на гитхаб) Там всё расписано.") {
		t.Fatalf("the dev command lost the invite line: %q", last)
	}
	if !strings.Contains(last, "https://github.com/BlindMaster24/MgkeTimetableBot") {
		t.Fatalf("the dev command lost the repository link: %q", last)
	}
}

func TestOldBot_AdminCommandsStayClosedForUsers(t *testing.T) {
	caller := &recordingCaller{}
	b, _ := setupE2EBotWithCaller(t, caller, 4242)

	parsed := 0
	b.SetParseFunc(func() error {
		parsed++
		return nil
	})

	for _, command := range []string{"/vanish", "/forceparse", "/regexp", "/requireNewButtons"} {
		caller.reset()
		sendOldText(t, b, 777, command)

		texts := deliveredTexts(t, caller, 1)
		if len(texts) == 0 {
			t.Fatalf("%s answered a user with nothing", command)
		}
		last := texts[len(texts)-1]
		if last != "Команда не найдена" {
			t.Fatalf("%s answered a user with %q", command, last)
		}
	}

	if parsed != 0 {
		t.Fatalf("a user triggered the parser %d times", parsed)
	}
}

func TestOldBot_AdminVanishAnswersLikeTheOldBot(t *testing.T) {
	caller := &recordingCaller{}
	b, _ := setupE2EBotWithCaller(t, caller, 4242)

	sendOldText(t, b, 4242, "/vanish")

	texts := deliveredTexts(t, caller, 1)
	last := texts[len(texts)-1]
	if last != "Бд почищена" {
		t.Fatalf("the vanish command answered %q", last)
	}
}

func TestOldBot_ForceParseAnswersQueued(t *testing.T) {
	caller := &recordingCaller{}
	b, _ := setupE2EBotWithCaller(t, caller, 4242)
	b.SetParseFunc(func() error { return nil })

	sendOldText(t, b, 4242, "/forceparse")

	texts := deliveredTexts(t, caller, 3)
	queued := indexOfText(texts, "Запущено")
	done := indexOfText(texts, "Данные успешно обновлены!")
	if queued < 0 {
		t.Fatalf("the forceparse command never answered %q, got %q", "Запущено", texts)
	}
	if done < queued {
		t.Fatalf("the forceparse command never reported the result: %q", texts)
	}
}

func TestOldBot_SetGroupExplainsTheSyntax(t *testing.T) {
	caller := &recordingCaller{}
	b, _ := setupE2EBotWithCaller(t, caller, 4242)

	sendOldText(t, b, 4242, "/setgroup")
	texts := deliveredTexts(t, caller, 1)
	if last := texts[len(texts)-1]; last != "Неправильный синтаксис команды\n\nПример:\n/setGroup 100" {
		t.Fatalf("an empty /setgroup answered %q", last)
	}

	caller.reset()
	sendOldText(t, b, 4242, "/setgroup abc")
	texts = deliveredTexts(t, caller, 1)
	if last := texts[len(texts)-1]; last != "Неправильный синтаксис команды\n\nПример:\n/setGroup 100" {
		t.Fatalf("a broken /setgroup answered %q", last)
	}

	caller.reset()
	sendOldText(t, b, 4242, "/setgroup 101")
	texts = deliveredTexts(t, caller, 1)
	if last := texts[len(texts)-1]; last != "Данной учебной группы не существует" {
		t.Fatalf("an unknown /setgroup answered %q", last)
	}

	caller.reset()
	sendOldText(t, b, 4242, "/setgroup 100")
	texts = deliveredTexts(t, caller, 1)
	if last := texts[len(texts)-1]; last != "Группа этого чата была успешно изменена на '100'" {
		t.Fatalf("a valid /setgroup answered %q", last)
	}
}

func TestOldBot_SetTeacherExplainsTheSyntax(t *testing.T) {
	caller := &recordingCaller{}
	b, _ := setupE2EBotWithCaller(t, caller, 4242)

	sendOldText(t, b, 4242, "/setteacher x")
	texts := deliveredTexts(t, caller, 1)
	if last := texts[len(texts)-1]; last != "Неправильный синтаксис команды\n\nПример:\n/setTeacher Иванов И.И." {
		t.Fatalf("a broken /setteacher answered %q", last)
	}

	caller.reset()
	sendOldText(t, b, 4242, "/setteacher Иванов")
	texts = deliveredTexts(t, caller, 1)
	if last := texts[len(texts)-1]; last != "Преподвателя этого чата был успешно изменен на 'Иванов И.И.'" {
		t.Fatalf("a valid /setteacher answered %q", last)
	}
}

func TestOldBot_SetUpSceneKeepsItsOwnWording(t *testing.T) {
	caller := &recordingCaller{}
	b, _ := setupE2EBotWithCaller(t, caller, 4242)

	chat, err := b.chatRepo.FindOrCreate("telegram", 4242)
	if err != nil {
		t.Fatalf("find chat: %v", err)
	}
	chat.Scene = sceneSetGroup
	if err := b.chatRepo.Save(chat); err != nil {
		t.Fatalf("save chat: %v", err)
	}

	sendOldText(t, b, 4242, "abc")

	texts := deliveredTexts(t, caller, 1)
	if last := texts[len(texts)-1]; last != "Номер группы введён неверно" {
		t.Fatalf("the setup scene answered %q", last)
	}
}

func TestOldBot_IdPrintsChatPeerAndUser(t *testing.T) {
	caller := &recordingCaller{}
	b, repo := setupE2EBotWithCaller(t, caller, 4242)
	chatWithGroup(t, repo, 4242, "100")

	sendOldText(t, b, 4242, "/id")

	texts := deliveredTexts(t, caller, 1)
	last := texts[len(texts)-1]
	lines := strings.Split(last, "\n")
	if len(lines) != 3 {
		t.Fatalf("the id command answered %q", last)
	}
	if !strings.HasPrefix(lines[0], "chat_id: ") {
		t.Fatalf("the id command lost the chat id: %q", last)
	}
	if lines[1] != "peer_id: 4242" {
		t.Fatalf("the id command lost the peer id: %q", last)
	}
	if lines[2] != "user_id: 4242" {
		t.Fatalf("the id command lost the user id: %q", last)
	}
}

func TestOldBot_BrovkaAsksForTheDay(t *testing.T) {
	caller := &recordingCaller{}
	b, _ := setupE2EBotWithCaller(t, caller, 4242)

	sendOldText(t, b, 4242, "/vychetkaDlyaBrovkiDSOnline")

	texts := deliveredTexts(t, caller, 1)
	if last := texts[len(texts)-1]; last != "День, с которого необходимо начать не указан" {
		t.Fatalf("the brovka command answered %q", last)
	}
}

func TestOldBot_DayIndexConverters(t *testing.T) {
	caller := &recordingCaller{}
	b, _ := setupE2EBotWithCaller(t, caller, 4242)

	sendOldText(t, b, 4242, "/indexToDate 1000")
	texts := deliveredTexts(t, caller, 1)
	want := utils.DayIndexToDate(1000).Format("02.01.2006")
	if last := texts[len(texts)-1]; last != want {
		t.Fatalf("the day index converter answered %q, want %q", last, want)
	}

	caller.reset()
	sendOldText(t, b, 4242, "/dayIndexToStrDate 1000")
	texts = deliveredTexts(t, caller, 1)
	if last := texts[len(texts)-1]; last != want {
		t.Fatalf("a day index alias answered %q, want %q", last, want)
	}

	caller.reset()
	sendOldText(t, b, 4242, "/indexToDate")
	texts = deliveredTexts(t, caller, 1)
	if last := texts[len(texts)-1]; last != "Индекс дня не число" {
		t.Fatalf("the day index converter answered %q", last)
	}

	parsed, err := time.Parse("02.01.2006", want)
	if err != nil {
		t.Fatalf("parse %q: %v", want, err)
	}
	index := strconv.Itoa(utils.DayIndexFromDate(parsed))

	caller.reset()
	sendOldText(t, b, 4242, "/dateToIndex "+want)
	texts = deliveredTexts(t, caller, 1)
	if last := texts[len(texts)-1]; last != index {
		t.Fatalf("the date converter answered %q, want %q", last, index)
	}

	caller.reset()
	sendOldText(t, b, 4242, "/strDateToDayIndex "+want)
	texts = deliveredTexts(t, caller, 1)
	if last := texts[len(texts)-1]; last != index {
		t.Fatalf("a date alias answered %q, want %q", last, index)
	}

	caller.reset()
	sendOldText(t, b, 4242, "/dateToIndex")
	texts = deliveredTexts(t, caller, 1)
	if last := texts[len(texts)-1]; last != "День не введён" {
		t.Fatalf("the date converter answered %q", last)
	}
}

func TestOldBot_BangPrefixWorksLikeASlash(t *testing.T) {
	caller := &recordingCaller{}
	b, repo := setupE2EBotWithCaller(t, caller, 4242)
	chatWithGroup(t, repo, 4242, "100")
	b.SetNow(func() time.Time { return time.Date(2026, 8, 31, 10, 0, 0, 0, time.Local) })

	sendOldText(t, b, 4242, "!day")

	texts := deliveredTexts(t, caller, 2)
	last := texts[len(texts)-1]
	if last == "Команда не найдена" {
		t.Fatalf("the bang prefix stopped working: %q", last)
	}
	if !strings.Contains(last, "Математика") {
		t.Fatalf("the bang prefix did not show the timetable: %q", last)
	}

	caller.reset()
	sendOldText(t, b, 4242, "!vanish")
	texts = deliveredTexts(t, caller, 1)
	if last := texts[len(texts)-1]; last != "Бд почищена" {
		t.Fatalf("an admin bang command answered %q", last)
	}
}

func TestOldBot_SqlKeepsTheOldAliases(t *testing.T) {
	caller := &recordingCaller{}
	b, _ := setupE2EBotWithCaller(t, caller, 4242)

	for _, text := range []string{"/sql SELECT 1", "/SQL SELECT 1", "/db SELECT 1", "/dbrun SELECT 1", "/db_run SELECT 1", "/sql_run SELECT 1"} {
		caller.reset()
		sendOldText(t, b, 4242, text)

		texts := deliveredTexts(t, caller, 1)
		last := texts[len(texts)-1]
		if last == "Команда не найдена" {
			t.Fatalf("%s stopped routing to the sql command", text)
		}
		if strings.HasPrefix(last, "❌") {
			t.Fatalf("%s did not reach the query: %q", text, last)
		}
	}
}

func TestOldBot_DebugCountsChatsAndApiKeys(t *testing.T) {
	caller := &recordingCaller{}
	b, repo := setupE2EBotWithCaller(t, caller, 4242)
	chatWithGroup(t, repo, 4242, "100")

	store := apikey.NewStore(repo.DB(), strings.Repeat("k", 32))
	if err := store.EnsureSchema(); err != nil {
		t.Fatalf("api key schema: %v", err)
	}
	key, _, err := store.FindOrCreate(7)
	if err != nil {
		t.Fatalf("create api key: %v", err)
	}
	if err := store.Touch(key.ID, time.Now()); err != nil {
		t.Fatalf("touch api key: %v", err)
	}
	b.SetKeyStore(store)

	sendOldText(t, b, 4242, "/debug")

	texts := deliveredTexts(t, caller, 1)
	last := texts[len(texts)-1]

	total, allowed, err := repo.CountTGChats()
	if err != nil {
		t.Fatalf("count telegram chats: %v", err)
	}
	wantChats := "Чатов бота Telegram: " + itoa(total) + " (allow: " + itoa(allowed) + ")"
	if !strings.Contains(last, wantChats) {
		t.Fatalf("the debug command lost the telegram chat counter, want %q in %q", wantChats, last)
	}
	if !strings.Contains(last, "API ключей: 1 (active: 1)") {
		t.Fatalf("the debug command lost the api key counter: %q", last)
	}
}

func TestOldBot_TeacherHintSpellingFollowsTheOldCommands(t *testing.T) {
	caller := &recordingCaller{}
	b, repo := setupE2EBotWithCaller(t, caller, 4242)
	b.cache.SetTeachers(map[string]any{"Иванов И.И.": map[string]any{"days": []any{}}}, "hint-teachers")

	const userID int64 = 9001
	chat, err := repo.FindOrCreate("telegram", userID)
	if err != nil {
		t.Fatal(err)
	}
	chat.Mode = ModeTeacher
	chat.Accepted = true
	if err := repo.Save(chat); err != nil {
		t.Fatal(err)
	}

	sendOldText(t, b, userID, "/start")
	texts := deliveredTexts(t, caller, 1)
	last := texts[len(texts)-1]
	if !strings.Contains(last, "Выбрать преподавателя можно командой /setTeacher") {
		t.Fatalf("the start hint lost the old wording: %q", last)
	}
	if strings.Contains(last, "преподвателя") {
		t.Fatalf("the start hint must use the correct spelling, got %q", last)
	}

	caller.reset()
	sendOldText(t, b, userID, "/day")
	texts = deliveredTexts(t, caller, 1)
	last = texts[len(texts)-1]
	if !strings.Contains(last, "Выбрать преподвателя можно командой /setTeacher") {
		t.Fatalf("the day hint must keep the old typo verbatim, got %q", last)
	}

	caller.reset()
	sendOldText(t, b, userID, "/week")
	texts = deliveredTexts(t, caller, 1)
	last = texts[len(texts)-1]
	if !strings.Contains(last, "Выбрать преподавателя можно командой /setTeacher") {
		t.Fatalf("the week hint lost the old wording: %q", last)
	}
	if strings.Contains(last, "преподвателя") {
		t.Fatalf("the week hint must use the correct spelling, got %q", last)
	}
}

func indexOfText(texts []string, want string) int {
	for i, text := range texts {
		if text == want {
			return i
		}
	}
	return -1
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	digits := []byte{}
	for value > 0 {
		digits = append([]byte{byte('0' + value%10)}, digits...)
		value /= 10
	}
	return string(digits)
}
