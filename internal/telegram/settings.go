package telegram

import (
	"fmt"
	"strings"
)

func (b *Bot) sendCallsShow(u *Update) error {
	chat, _ := b.chatRepo.FindOrCreate("telegram", u.UserID)
	b.displayCalls(u, chat, true)
	return nil
}

func (b *Bot) currentSettingsText(chat *Chat) string {
	yesNo := func(v bool) string {
		if v {
			return "да"
		}
		return "нет"
	}
	lines := []string{
		fmt.Sprintf(`Показывать кнопку расписания "📄 На день": %s`, yesNo(chat.ShowDaily)),
		fmt.Sprintf(`Показывать кнопку расписания "📑 На неделю": %s`, yesNo(chat.ShowWeekly)),
		fmt.Sprintf(`Показывать кнопку "🕐 Звонки": %s`, yesNo(chat.ShowCalls)),
		fmt.Sprintf(`Показывать кнопку "💡 О боте": %s`, yesNo(chat.ShowAbout)),
		fmt.Sprintf(`Показывать кнопку "👩‍🎓 Группа": %s`, yesNo(chat.ShowFastGroup)),
		fmt.Sprintf(`Показывать кнопку "👩‍🏫 Преподаватель": %s`, yesNo(chat.ShowFastTeacher)),
		"",
		fmt.Sprintf("Скрывать прошедшие дни в расписании на неделю: %s", yesNo(chat.HidePastDays)),
		fmt.Sprintf("Отображать в сообщении время последней загрузки расписания: %s", yesNo(chat.ShowParserTime)),
		fmt.Sprintf("Подсказки под расписанием: %s", yesNo(chat.ShowHints)),
		fmt.Sprintf(`Раздел "Что изменилось": %s`, yesNo(chat.DiffEnabled)),
		fmt.Sprintf("Diff после /week: %s", yesNo(chat.DiffAutoInWeek)),
		fmt.Sprintf("Diff в уведомлениях: %s", yesNo(chat.DiffAutoInUpdates)),
		fmt.Sprintf(`Показывать старое -> новое: %s`, yesNo(chat.DiffShowBeforeAfter)),
		fmt.Sprintf("Лимит строк diff: %d", chat.DiffMaxLines),
		"",
		fmt.Sprintf("Оповещение о добавлении нового дня: %s", yesNo(chat.NoticeChanges)),
		fmt.Sprintf("Оповещение о добавлении новой недели: %s", yesNo(chat.NoticeNextWeek)),
		fmt.Sprintf("Оповещение об изменении звонков: %s", yesNo(chat.NoticeCalls)),
		fmt.Sprintf("Оповещение об ошибке парсера: %s", yesNo(chat.NoticeParserErrors)),
		"",
		"~~~ Системные (отладочная информация) ~~~",
		fmt.Sprintf("Режим чата: %s (deactivateSecondaryCheck: %s)", string(chat.Mode), yesNo(chat.DeactivateSecondaryCheck)),
		fmt.Sprintf("Выбранная группа: %s", chat.Group),
		fmt.Sprintf("Выбранный учитель: %s", chat.Teacher),
		fmt.Sprintf("ID последнего сообщения: %d", chat.LastMsgID),
		fmt.Sprintf("Время последенего сообщения: %d", chat.LastMsgTime),
		fmt.Sprintf("Разрешено ли отправлять боту сообщения: %s", yesNo(chat.AllowSendMess)),
		fmt.Sprintf("Подписка на рассылку: %s", yesNo(chat.SubscribeDistribution)),
		fmt.Sprintf("Требуется ли принудительное обновление кнопок: %s", yesNo(chat.NeedUpdateButtons)),
	}
	return strings.Join(lines, "\n")
}
