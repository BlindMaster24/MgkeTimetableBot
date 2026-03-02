import type { TelegramBotCommand } from '../../types/telegram';
import { DayIndex, StringDate, WeekIndex } from "../../../../utils";
import { GroupDay, TeacherDay } from "../../../parser/types";
import { AbstractCommand, CmdHandlerParams } from "../../abstract";
import { StaticKeyboard } from "../../keyboard";

export default class extends AbstractCommand {
    public regexp = /^((!|\/)archive)(\b|$|\s)/i;
    public payloadAction = null;
    public tgCommand: TelegramBotCommand = {
        command: 'archive',
        description: 'Архив расписания за прощедшие дни'
    };
    public scene?: string | null = null;

    async handler({ context, chat, formatter }: CmdHandlerParams) {
        const raw: string | undefined = context.text?.replace(this.regexp, '').trim();
        if (!raw) {
            return context.send('День не указан');
        }

        const archive = this.app.getService('timetable');
        const weekMatch = /^(week|неделя)\s+(\d+)\s*$/i.exec(raw);
        if (weekMatch) {
            const weekNumber = Number(weekMatch[2]);
            if (!Number.isFinite(weekNumber) || weekNumber < 1) {
                return context.send('Неверный номер недели');
            }

            const weekIndex = WeekIndex.fromAcademicWeekNumber(weekNumber);
            const weekRange = weekIndex.getWeekDayIndexRange();
            const [start, end] = weekIndex.getWeekRange();
            const weekLabel = `Учебная неделя №${weekIndex.getAcademicWeekNumber()} (${StringDate.fromDate(start).toStringDate()}-${StringDate.fromDate(end).toStringDate()})`;

            if (chat.mode === 'student' || chat.mode === 'parent') {
                if (chat.group == null) {
                    return context.send(`Для данного чата группа не была выбрана.`);
                }

                const days = await archive.getGroupDaysByRange(weekRange, chat.group);
                if (days.length === 0) {
                    return context.send('Нет данных за указанную неделю');
                }

                const text = formatter.formatGroupFull(chat.group, {
                    showHeader: true,
                    days,
                    weekLabel
                });

                return context.send(text, {
                    keyboard: StaticKeyboard.GetWeekTimetable({ type: 'group', value: chat.group, weekIndex: weekIndex.valueOf() })
                });
            }

            if (chat.mode === 'teacher') {
                if (chat.teacher == null) {
                    return context.send(`Для данного чата учитель не был выбран.`);
                }

                const days = await archive.getTeacherDaysByRange(weekRange, chat.teacher);
                if (days.length === 0) {
                    return context.send('Нет данных за указанную неделю');
                }

                const text = formatter.formatTeacherFull(chat.teacher, {
                    showHeader: true,
                    days,
                    weekLabel
                });

                return context.send(text, {
                    keyboard: StaticKeyboard.GetWeekTimetable({ type: 'teacher', value: chat.teacher, weekIndex: weekIndex.valueOf() })
                });
            }

            return context.send(`Для данного режима чата (${chat.mode}) нельзя автоматически получить группу или учителя.`);
        }

        const day = this.parseDayInput(raw);
        if (!day) {
            return context.send('Неверный формат даты. Пример: /archive 12.02 или /archive 12.02.2026 или /archive week 5');
        }

        const dayIndex: number = DayIndex.fromStringDate(day).valueOf();

        let entry: GroupDay | TeacherDay | null = null;
        let text: string | undefined;
        let type: 'group' | 'teacher';
        let value: string;
        if (chat.mode === 'student' || chat.mode === 'parent') {
            if (chat.group == null) {
                return context.send(`Для данного чата группа не была выбрана.`);
            }

            type = 'group';
            value = chat.group;

            entry = await archive.getGroupDay(dayIndex, chat.group);
            if (entry) {
                text = formatter.formatGroupFull(chat.group, {
                    showHeader: true,
                    days: [entry]
                });
            }
        } else if (chat.mode === 'teacher') {
            if (chat.teacher == null) {
                return context.send(`Для данного чата учитель не был выбран.`);
            }

            type = 'teacher';
            value = chat.teacher;

            entry = await archive.getTeacherDay(dayIndex, chat.teacher);
            if (entry) {
                text = formatter.formatTeacherFull(chat.teacher, {
                    showHeader: true,
                    days: [entry]
                });
            }
        } else {
            //todo get from args
            return context.send(`Для данного режима чата (${chat.mode}) нельзя автоматически получить группу или учителя.`);
        }

        if (!entry || !text) {
            const { min: minDayIndex, max: maxDayIndex } = await archive.getDayIndexBounds();

            if (dayIndex < minDayIndex || dayIndex > maxDayIndex) {
                const fromDay = StringDate.fromDayIndex(minDayIndex).toString();
                const toDay = StringDate.fromDayIndex(maxDayIndex).toString();

                //todo another day format
                return context.send([
                    'Вы указали день, который находится вне периода сохранённых дней.',
                    `В базе хранятся дни, начиная с ${fromDay} по ${toDay}`
                ].join('\n'));
            }

            return context.send('Ничего не найдено на данный день');
        }

        const weekIndex = WeekIndex.fromStringDate(entry.day);

        return context.send(text, {
            keyboard: StaticKeyboard.GetWeekTimetable({ type, value, weekIndex: weekIndex.valueOf() })
        });
    }

    private parseDayInput(raw: string): string | null {
        const parts = raw.split('.').map((value) => value.trim()).filter((value) => value.length > 0);
        if (parts.length < 2 || parts.length > 3) {
            return null;
        }

        const day = Number(parts[0]);
        const month = Number(parts[1]);
        const year = parts.length === 3 ? Number(parts[2]) : new Date().getFullYear();
        if (!Number.isFinite(day) || !Number.isFinite(month) || !Number.isFinite(year)) {
            return null;
        }

        const normalized = `${String(day).padStart(2, '0')}.${String(month).padStart(2, '0')}.${year}`;
        try {
            StringDate.fromStringDate(normalized);
        } catch {
            return null;
        }

        return normalized;
    }
}
