import { TelegramBotCommand } from "puregram/generated";
import { getDayRasp, randArray } from "../../../utils";
import { raspCache } from "../../parser";
import { AbstractCommand, CmdHandlerParams } from "../abstract";
import { StaticKeyboard } from "../keyboard";

export default class extends AbstractCommand {
    public regexp = /(^(!|\/)start)|^(Начать|Start|(Главное\s)?Меню)$/i
    public payloadAction = null;
    public tgCommand: TelegramBotCommand = {
        command: 'start',
        description: 'Запустить бота'
    };

    async handler({ context, chat, keyboard, formatter, actions }: CmdHandlerParams) {
        if (chat.mode !== null) {
            context.cancelInput();
            chat.scene = null;
            if (Object.keys(raspCache.groups.timetable).length == 0 &&
                Object.keys(raspCache.teachers.timetable).length == 0) {
                return context.send('Данные с сервера ещё не загружены, ожидайте...', {
                    keyboard: keyboard.MainMenu
                });
            }

            if (chat.mode === 'student' || chat.mode === 'parent') {
                if (!chat.group) {
                    const randGroup = randArray(Object.keys(raspCache.groups.timetable));
                    return context.send(
                        'Ваша учебная группа не выбрана\n\n' +
                        'Выбрать группу можно командой /setGroup <group>\n' +
                        'Пример:\n' +
                        `/setGroup ${randGroup}`,
                        { keyboard: keyboard.MainMenu }
                    );
                }

                const rasp = raspCache.groups.timetable[chat.group];
                if (!rasp) {
                    return context.send('Данной учебной группы не существует', {
                        keyboard: keyboard.MainMenu
                    });
                }

                actions.deleteLastMsg();
                const message = formatter.formatGroupFull(String(chat.group), {
                    days: getDayRasp(rasp.days)
                });
                actions.deleteUserMsg();

                return context.send(message, {
                    keyboard: keyboard.MainMenu
                }).then(context => actions.handlerLastMsgUpdate(context));
            }

            if (chat.mode === 'teacher') {
                if (!chat.teacher) {
                    const randTeacher = randArray(Object.keys(raspCache.teachers.timetable));
                    return context.send(
                        'Имя преподавателя не выбрано\n\n' +
                        'Выбрать преподавателя можно командой /setTeacher <teacher>\n' +
                        'Пример:\n' +
                        `/setTeacher ${randTeacher}`,
                        { keyboard: keyboard.MainMenu }
                    );
                }

                const rasp = raspCache.teachers.timetable[chat.teacher];
                if (!rasp) {
                    return context.send('Ничего не найдено', {
                        keyboard: keyboard.MainMenu
                    });
                }

                actions.deleteLastMsg();
                const message = formatter.formatTeacherFull(chat.teacher, {
                    days: getDayRasp(rasp.days)
                });
                actions.deleteUserMsg();

                return context.send(message, {
                    keyboard: keyboard.MainMenu
                }).then(context => actions.handlerLastMsgUpdate(context));
            }

            return context.send('Главное меню', {
                keyboard: keyboard.MainMenu
            });
        }

        chat.scene = 'setup';

        return context.send('Кто будет использовать бота?', {
            keyboard: StaticKeyboard.SelectMode
        });
    }
}
