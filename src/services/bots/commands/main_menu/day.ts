import type { TelegramBotCommand } from '../../types/telegram';
import { getDayRasp, randArray } from "../../../../utils";
import { raspCache } from '../../../parser';
import { AbstractCommand, CmdHandlerParams } from "../../abstract";
import { StaticKeyboard } from '../../keyboard';

export default class extends AbstractCommand {
    public regexp = /^((!|\/)(get)?(rasp)?day|(📄\s)?(расписание\s)?на день)$/i
    public payloadAction = null;
    public tgCommand: TelegramBotCommand = {
        command: 'day',
        description: 'Ваше расписание на день'
    };

    async handler(params: CmdHandlerParams) {
        const { context, chat } = params;

        if (Object.keys(raspCache.groups.timetable).length == 0 && Object.keys(raspCache.teachers.timetable).length == 0) {
            return context.send('Данные с сервера ещё не загружены, ожидайте...');
        }

        if (chat.mode == 'student' || chat.mode == 'parent') return this.groupRasp(params);
        if (chat.mode == 'teacher') return this.teacherRasp(params);

        return context.send('Первоначальная настройка ещё не была произведена', {
            keyboard: StaticKeyboard.StartButton
        });
    }

    private async groupRasp({ chat, context, actions, formatter }: CmdHandlerParams) {
        if (chat.group == null) {
            const randGroup = randArray(Object.keys(raspCache.groups.timetable))

            return context.send(
                'Ваша учебная группа не выбрана\n\n' +
                'Выбрать группу можно командой /setGroup <group>\n' +
                'Пример:\n' +
                `/setGroup ${randGroup}`
            )
        }

        const rasp = raspCache.groups.timetable[chat.group]
        if (rasp === undefined) return context.send('Данной учебной группы не существует');

        actions.deleteLastMsg()

        const message = formatter.formatGroupFull(String(chat.group), {
            days: getDayRasp(rasp.days)
        })

        actions.deleteUserMsg()

        return context.send(message).then(context => actions.handlerLastMsgUpdate(context))
    }

    private async teacherRasp({ chat, context, actions, formatter }: CmdHandlerParams) {
        if (chat.teacher == null) {
            const randTeacher = randArray(Object.keys(raspCache.teachers.timetable))

            return context.send(
                'Имя преподавателя не выбрано\n\n' +
                'Выбрать преподвателя можно командой /setTeacher <teacher>\n' +
                'Пример:\n' +
                `/setTeacher ${randTeacher}`
            )
        }

        if (Object.keys(raspCache.teachers.timetable).length == 0) return context.send('Данные с сервера ещё не загружены, ожидайте...')

        const rasp = raspCache.teachers.timetable[chat.teacher]
        if (rasp === undefined) return context.send('Ничего не найдено');

        actions.deleteLastMsg()

        const message = formatter.formatTeacherFull(chat.teacher, {
            days: getDayRasp(rasp.days)
        })

        actions.deleteUserMsg()

        return context.send(message).then(context => actions.handlerLastMsgUpdate(context))
    }
}