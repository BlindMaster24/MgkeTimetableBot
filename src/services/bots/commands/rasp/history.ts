import type { TelegramBotCommand } from "puregram/generated";
import { StringDate, WeekIndex } from "../../../../utils";
import { raspCache } from "../../../parser";
import { AbstractCommand, CmdHandlerParams, MessageOptions } from "../../abstract";
import { InputInitiator } from "../../input";
import { StaticKeyboard, withCancelButton } from "../../keyboard";

export default class HistoryTeacherCommand extends AbstractCommand {
    public regexp = /^((!|\/)history|(📚\s)?История)(\b|$|\s)/i;
    public payloadAction = null;
    public scene?: string | null = null;
    public tgCommand: TelegramBotCommand = {
        command: 'history',
        description: 'История расписания по преподавателю и учебной неделе'
    };

    async handler(params: CmdHandlerParams<HistoryTeacherCommand>) {
        const { context, keyboard, formatter } = params;

        if (Object.keys(raspCache.teachers.timetable).length == 0) {
            return context.send('Данные с сервера ещё не загружены, ожидайте...');
        }

        let initiator: InputInitiator;
        let teacher: string | false | undefined = await context.input(this.buildTeacherPrompt(keyboard), {
            keyboard: withCancelButton(keyboard.TeacherHistory)
        }).then<string | undefined>(value => {
            initiator = value?.initiator;
            return value?.text;
        });

        while (true) {
            teacher = await this.findTeacher(params, teacher, keyboard.MainMenu);
            if (!teacher) {
                if (teacher === undefined) {
                    teacher = await context.waitInput().then<string | undefined>(value => {
                        initiator = value?.initiator;
                        return value?.text;
                    });
                    continue;
                }
                return;
            }
            break;
        }

        const requested = await context.input('Введите номер учебной недели или дату (дд.мм или дд.мм.гггг)', {
            keyboard: withCancelButton(keyboard.TeacherHistory)
        }).then<string | undefined>(value => {
            initiator = value?.initiator;
            return value?.text;
        });

        const weekIndex = this.parseWeekIndex(requested);
        if (!weekIndex) {
            return context.send('Не удалось определить учебную неделю');
        }

        const { min, max } = await this.app.getService('timetable').getWeekIndexBounds();
        if (weekIndex.valueOf() < min || weekIndex.valueOf() > max) {
            return context.send('Нет данных за указанную неделю');
        }

        const weekRange = weekIndex.getWeekDayIndexRange();
        const days = await this.app.getService('timetable').getTeacherDaysByRange(weekRange, teacher);
        const weekLabel = this.getAcademicWeekLabel(weekIndex);

        const message = formatter.formatTeacherFull(teacher, {
            showHeader: true,
            days: days,
            weekLabel
        });

        const options: MessageOptions = {
            keyboard: await keyboard.WeekControl('teacher', teacher, weekIndex.valueOf(), false)
        };

        if (initiator === 'callback') {
            return context.editOrSend(message, options);
        }

        return context.send(message, options);
    }

    private parseWeekIndex(input: string | undefined): WeekIndex | null {
        if (!input) {
            return WeekIndex.getRelevant();
        }

        const value = input.trim().toLowerCase();
        if (!value) {
            return WeekIndex.getRelevant();
        }

        if (/^\d+$/.test(value)) {
            const weekNumber = Number(value);
            if (!weekNumber || weekNumber < 1) {
                return null;
            }
            return WeekIndex.fromAcademicWeekNumber(weekNumber);
        }

        const dateMatch = value.match(/^(\d{1,2})\.(\d{1,2})(?:\.(\d{2,4}))?$/);
        if (!dateMatch) {
            return null;
        }

        const day = Number(dateMatch[1]);
        const month = Number(dateMatch[2]);
        const yearRaw = dateMatch[3];
        const year = yearRaw ? Number(yearRaw.length === 2 ? `20${yearRaw}` : yearRaw) : new Date().getFullYear();
        if (!day || !month || !year) {
            return null;
        }

        const str = `${String(day).padStart(2, '0')}.${String(month).padStart(2, '0')}.${year}`;
        const date = StringDate.fromStringDate(str).toDate();
        return WeekIndex.fromDate(date);
    }

    private getAcademicWeekLabel(weekIndex: WeekIndex): string {
        const [start, end] = weekIndex.getWeekRange();
        const weekNumber = weekIndex.getAcademicWeekNumber();
        return `Учебная неделя №${weekNumber} (${StringDate.fromDate(start).toStringDate()}-${StringDate.fromDate(end).toStringDate()})`;
    }
}
