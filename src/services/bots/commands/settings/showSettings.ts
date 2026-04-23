import { AbstractCommand, CmdHandlerParams } from '../../abstract';

export default class extends AbstractCommand {
    public regexp = /^((!|\/)(show|current)Settings)|(Показать текущие)$/i;
    public payloadAction = null;

    handler({ context, serviceChat, chat, service }: CmdHandlerParams) {
        return context.send(
            [
                `Показывать кнопку расписания "📄 На день": ${chat.showDaily ? 'да' : 'нет'}`,
                `Показывать кнопку расписания "📑 На неделю": ${chat.showWeekly ? 'да' : 'нет'}`,
                `Показывать кнопку "🕐 Звонки": ${chat.showCalls ? 'да' : 'нет'}`,
                `Показывать кнопку "💡 О боте": ${chat.showAbout ? 'да' : 'нет'}`,
                `Показывать кнопку "👩‍🎓 Группа": ${chat.showFastGroup ? 'да' : 'нет'}`,
                `Показывать кнопку "👩‍🏫 Преподаватель": ${chat.showFastTeacher ? 'да' : 'нет'}`,

                `\nСкрывать прошедшие дни в расписании на неделю: ${chat.hidePastDays ? 'да' : 'нет'}`,
                `Отображать в сообщении время последней загрузки расписания: ${chat.showParserTime ? 'да' : 'нет'}`,
                `Подсказки под расписанием: ${chat.showHints ? 'да' : 'нет'}`,
                `Раздел "Что изменилось": ${chat.diffEnabled ? 'да' : 'нет'}`,
                `Diff после /week: ${chat.diffAutoInWeek ? 'да' : 'нет'}`,
                `Diff в уведомлениях: ${chat.diffAutoInUpdates ? 'да' : 'нет'}`,
                `Показывать старое -> новое: ${chat.diffShowBeforeAfter ? 'да' : 'нет'}`,
                `Лимит строк diff: ${chat.diffMaxLines}`,

                `\nОповещение о добавлении нового дня: ${chat.noticeChanges ? 'да' : 'нет'}`,
                `Оповещение о добавлении новой недели: ${chat.noticeNextWeek ? 'да' : 'нет'}`,
                `Оповещение об изменении звонков: ${chat.noticeCalls ? 'да' : 'нет'}`,
                `Оповещение об ошибке парсера: ${chat.noticeParserErrors ? 'да' : 'нет'}`,

                ...(service === 'vk'
                    ? [
                          '\n__ ТОЛЬКО ВК __:',
                          `Удалять прошлое сообщения бота о расписании: ${chat.deleteLastMsg ? 'да' : 'нет'}`,
                          `Удалять сообщение пользователя после вызова расписания: ${chat.deleteUserMsg ? 'да' : 'нет'}`,
                          `Разрешено ли одобрить VK приложение: ${service === 'vk' && serviceChat.allowVkAppAccept ? 'да' : 'нет (возможно уже использовано)'}`
                      ]
                    : []),

                '\n~~~ Системные (отладочная информация) ~~~',
                `Режим чата: ${chat.mode} (deactivateSecondaryCheck: ${chat.deactivateSecondaryCheck ? 'да' : 'нет'})`,
                `Выбранная группа: ${chat.group || 'нет'}`,
                `Выбранный учитель: ${chat.teacher || 'нет'}`,
                `ID последнего сообщения: ${chat.lastMsgId}`,
                `Время последенего сообщения: ${chat.lastMsgTime}`,
                `Разрешено ли отправлять боту сообщения: ${chat.allowSendMess ? 'да' : 'нет'}`,
                `Подписка на рассыку: ${chat.subscribeDistribution ? 'да' : 'нет'}`,
                `Требуется ли принудительное обновление кнопок: ${chat.needUpdateButtons ? 'да' : 'нет'}`
            ].join('\n')
        );
    }
}
