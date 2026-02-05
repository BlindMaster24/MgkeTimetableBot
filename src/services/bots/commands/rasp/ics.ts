import { TelegramBotCommand } from "puregram/generated";
import { config } from "../../../../../config";
import { WeekIndex } from "../../../../utils";
import { buildIcs } from "../../../calendar/ics";
import { AbstractCommand, BotServiceName, CmdHandlerParams } from "../../abstract";

export default class IcsCommand extends AbstractCommand {
    public regexp = /^((!|\/)ics|(📅\s*)?ics)$/i;
    public payloadAction = null;
    public services: BotServiceName[] = ['tg'];
    public tgCommand: TelegramBotCommand = {
        command: 'ics',
        description: 'Экспорт расписания в .ics'
    };

    async handler({ context, chat, keyboard }: CmdHandlerParams) {
        if (!config.calendar?.ics?.enabled) {
            return context.send('ICS отключен в конфиге.', { keyboard: keyboard.MainMenu });
        }

        const timetable = this.app.getService('timetable');
        const weekIndex = WeekIndex.getRelevant();
        const weekRange = weekIndex.getWeekDayIndexRange();
        const weekNumber = weekIndex.getAcademicWeekNumber();

        if (chat.mode === 'student' || chat.mode === 'parent') {
            if (!chat.group) {
                return context.send('Группа не выбрана. Используйте /setup.', { keyboard: keyboard.MainMenu });
            }

            const days = await timetable.getGroupDaysByRange(weekRange, chat.group);
            if (days.length === 0) {
                return context.send('Нет расписания за текущую неделю.', { keyboard: keyboard.MainMenu });
            }

            const ics = buildIcs({
                type: 'group',
                value: String(chat.group),
                weekIndex,
                days
            });

            const filename = `schedule-group-${chat.group}-week-${String(weekNumber).padStart(2, '0')}.ics`;
            return context.sendFile(Buffer.from(ics, 'utf8'), filename, { keyboard: keyboard.MainMenu });
        }

        if (chat.mode === 'teacher') {
            if (!chat.teacher) {
                return context.send('Преподаватель не выбран. Используйте /setup.', { keyboard: keyboard.MainMenu });
            }

            const days = await timetable.getTeacherDaysByRange(weekRange, chat.teacher);
            if (days.length === 0) {
                return context.send('Нет расписания за текущую неделю.', { keyboard: keyboard.MainMenu });
            }

            const ics = buildIcs({
                type: 'teacher',
                value: String(chat.teacher),
                weekIndex,
                days
            });

            const filename = `schedule-teacher-${chat.teacher}-week-${String(weekNumber).padStart(2, '0')}.ics`;
            return context.sendFile(Buffer.from(ics, 'utf8'), filename, { keyboard: keyboard.MainMenu });
        }

        return context.send('Режим чата не поддерживает экспорт расписания.', { keyboard: keyboard.MainMenu });
    }
}
