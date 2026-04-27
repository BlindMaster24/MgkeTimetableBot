import type { TelegramBotCommand } from '../../types/telegram';
import { StringDate, WeekIndex, randArray, removePastDays } from '../../../../utils';
import { raspCache } from '../../../parser';
import { AbstractCommand, CmdHandlerParams } from '../../abstract';
import { StaticKeyboard } from '../../keyboard';

export default class extends AbstractCommand {
    public regexp = /^((!|\/)(get)?(rasp)?week|(📑\s)?(расписание\s)?на неделю)$/i;
    public payloadAction = null;
    public tgCommand: TelegramBotCommand = {
        command: 'week',
        description: 'Ваше расписание на неделю'
    };

    async handler(params: CmdHandlerParams) {
        const { context, chat } = params;

        if (
            Object.keys(raspCache.groups.timetable).length == 0 &&
            Object.keys(raspCache.teachers.timetable).length == 0
        ) {
            return context.send('Данные с сервера ещё не загружены, ожидайте...');
        }

        if (chat.mode == 'student' || chat.mode == 'parent') return this.groupRasp(params);
        if (chat.mode == 'teacher') return this.teacherRasp(params);

        return context.send('Первоначальная настройка ещё не была произведена', {
            keyboard: StaticKeyboard.StartButton
        });
    }

    private async groupRasp({ context, chat, actions, formatter, keyboard }: CmdHandlerParams) {
        if (chat.group == null) {
            const randGroup = randArray(Object.keys(raspCache.groups.timetable));

            return context.send(
                'Ваша учебная группа не выбрана\n\n' +
                    'Выбрать группу можно командой /setGroup <group>\n' +
                    'Пример:\n' +
                    `/setGroup ${randGroup}`
            );
        }

        const group = raspCache.groups.timetable[chat.group];
        if (group === undefined) return context.send('Данной учебной группы не существует');

        let weekIndex = WeekIndex.getRelevant();
        let weekRange = weekIndex.getWeekDayIndexRange();

        const weekDays = await this.app.getService('timetable').getGroupDaysByRange(weekRange, chat.group);
        let days = weekDays;
        if (chat.hidePastDays) {
            days = removePastDays(days);
            if (days.length === 0 && weekDays.length > 0) {
                weekIndex = weekIndex.getNextWeekIndex();
                weekRange = weekIndex.getWeekDayIndexRange();
                days = await this.app.getService('timetable').getGroupDaysByRange(weekRange, chat.group);
            }
        }

        actions.deleteLastMsg();

        const message = formatter.formatGroupFull(String(chat.group), {
            showHeader: false,
            days: days,
            weekLabel: this.getAcademicWeekLabel(weekIndex)
        });

        actions.deleteUserMsg();

        return context
            .send(message, {
                keyboard: await keyboard.WeekControl(
                    'group',
                    String(chat.group),
                    weekIndex.valueOf(),
                    chat.hidePastDays
                )
            })
            .then((id) => {
                actions.handlerLastMsgUpdate(context);
                return id;
            });
    }

    private async teacherRasp({ context, chat, actions, formatter, keyboard }: CmdHandlerParams) {
        if (chat.teacher == null) {
            const randTeacher = randArray(Object.keys(raspCache.teachers.timetable));

            return context.send(
                'Имя преподавателя не выбрано\n\n' +
                    'Выбрать преподавателя можно командой /setTeacher <teacher>\n' +
                    'Пример:\n' +
                    `/setTeacher ${randTeacher}`
            );
        }

        const teacher = raspCache.teachers.timetable[chat.teacher];
        if (teacher === undefined) return context.send('Данного преподавателя не существует');

        const weekIndex = WeekIndex.getRelevant();
        const weekRange = weekIndex.getWeekDayIndexRange();

        let days = await this.app.getService('timetable').getTeacherDaysByRange(weekRange, chat.teacher);
        if (chat.hidePastDays) {
            days = removePastDays(days);
        }

        actions.deleteLastMsg();

        const message = formatter.formatTeacherFull(chat.teacher, {
            showHeader: false,
            days: days,
            weekLabel: this.getAcademicWeekLabel(weekIndex)
        });

        actions.deleteUserMsg();

        return context
            .send(message, {
                keyboard: await keyboard.WeekControl('teacher', chat.teacher, weekIndex.valueOf(), chat.hidePastDays)
            })
            .then((context) => actions.handlerLastMsgUpdate(context));
    }

    private getAcademicWeekLabel(weekIndex: WeekIndex): string {
        const [start, end] = weekIndex.getWeekRange();
        const weekNumber = weekIndex.getAcademicWeekNumber();
        return `Учебная неделя №${weekNumber} (${StringDate.fromDate(start).toStringDate()}-${StringDate.fromDate(end).toStringDate()})`;
    }
}
