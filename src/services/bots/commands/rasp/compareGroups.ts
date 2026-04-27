import type { TelegramBotCommand } from '../../types/telegram';
import { config } from '../../../../../config';
import { StringDate, WeekIndex, randArray } from '../../../../utils';
import { GroupDay, GroupLesson } from '../../../parser/types';
import { raspCache } from '../../../parser';
import { AbstractCommand, CmdHandlerParams } from '../../abstract';
import { InputInitiator } from '../../input';
import { withCancelButton } from '../../keyboard';

export default class CompareGroupsCommand extends AbstractCommand {
    public regexp =
        /^(((!|\/)(comparegroups|comparegroup|groupscompare))|(\u0421\u0440\u0430\u0432\u043d\u0438\u0442\u044c \u0433\u0440\u0443\u043f\u043f\u044b))(\b|$|\s)/i;
    public payloadAction = null;
    public scene?: string | null = null;
    public tgCommand: TelegramBotCommand[] = [
        {
            command: 'comparegroups',
            description:
                '\u0421\u0440\u0430\u0432\u043d\u0438\u0442\u044c \u0440\u0430\u0441\u043f\u0438\u0441\u0430\u043d\u0438\u044f \u0434\u0432\u0443\u0445 \u0433\u0440\u0443\u043f\u043f'
        }
    ];

    async handler(params: CmdHandlerParams<CompareGroupsCommand>) {
        const { context, keyboard } = params;

        if (Object.keys(raspCache.groups.timetable).length === 0) {
            return context.send(
                '\u0414\u0430\u043d\u043d\u044b\u0435 \u0441 \u0441\u0435\u0440\u0432\u0435\u0440\u0430 \u0435\u0449\u0451 \u043d\u0435 \u0437\u0430\u0433\u0440\u0443\u0436\u0435\u043d\u044b, \u043e\u0436\u0438\u0434\u0430\u0439\u0442\u0435...'
            );
        }

        let initiator: InputInitiator;
        const groupKeys = Object.keys(raspCache.groups.timetable);
        const firstExample = randArray(groupKeys);
        const secondExample = randArray(groupKeys);

        let groupA: string | false | undefined = await context
            .input(
                `\u0412\u0432\u0435\u0434\u0438\u0442\u0435 \u043d\u043e\u043c\u0435\u0440 \u043f\u0435\u0440\u0432\u043e\u0439 \u0433\u0440\u0443\u043f\u043f\u044b (\u043d\u0430\u043f\u0440\u0438\u043c\u0435\u0440, ${firstExample})`,
                {
                    keyboard: withCancelButton(keyboard.GroupHistory)
                }
            )
            .then<string | undefined>((value) => {
                initiator = value?.initiator;
                return value?.text;
            });

        if (!groupA) return;

        let groupB: string | false | undefined = await context
            .input(
                `\u0412\u0432\u0435\u0434\u0438\u0442\u0435 \u043d\u043e\u043c\u0435\u0440 \u0432\u0442\u043e\u0440\u043e\u0439 \u0433\u0440\u0443\u043f\u043f\u044b (\u043d\u0430\u043f\u0440\u0438\u043c\u0435\u0440, ${secondExample})`,
                {
                    keyboard: withCancelButton(keyboard.GroupHistory)
                }
            )
            .then<string | undefined>((value) => {
                initiator = value?.initiator;
                return value?.text;
            });

        if (!groupB) return;

        while (true) {
            groupA = await this.findGroup(params, groupA, keyboard.MainMenu);
            if (!groupA) return;

            groupB = await this.findGroup(params, groupB, keyboard.MainMenu);
            if (!groupB) return;

            if (groupA === groupB) {
                await context.send(
                    '\u0412\u044b\u0431\u0435\u0440\u0438\u0442\u0435 \u0434\u0432\u0435 \u0440\u0430\u0437\u043d\u044b\u0435 \u0433\u0440\u0443\u043f\u043f\u044b',
                    {
                        keyboard: keyboard.MainMenu
                    }
                );

                return;
            }

            break;
        }

        const weekIndex = WeekIndex.getRelevant();
        const weekRange = weekIndex.getWeekDayIndexRange();
        const [start, end] = weekIndex.getWeekRange();
        const timetable = this.app.getService('timetable');
        const daysA = await timetable.getGroupDaysByRange(weekRange, groupA);
        const daysB = await timetable.getGroupDaysByRange(weekRange, groupB);

        const mapA = new Map<string, GroupDay>(daysA.map((d) => [d.day, d]));
        const mapB = new Map<string, GroupDay>(daysB.map((d) => [d.day, d]));

        const header = `\u0421\u0440\u0430\u0432\u043d\u0435\u043d\u0438\u0435 \u0433\u0440\u0443\u043f\u043f ${groupA} \u0438 ${groupB}`;
        const weekLabel = `\u0423\u0447\u0435\u0431\u043d\u0430\u044f \u043d\u0435\u0434\u0435\u043b\u044f \u2116${weekIndex.getAcademicWeekNumber()} (${StringDate.fromDate(start).toStringDate()}-${StringDate.fromDate(end).toStringDate()})`;

        const lines: string[] = [header, weekLabel];

        for (let dayIndex = weekRange[0]; dayIndex <= weekRange[1]; dayIndex += 1) {
            const dayDate = StringDate.fromDayIndex(dayIndex);
            if (dayDate.isSunday()) continue;

            const dayStr = dayDate.toStringDate();
            const dayName = dayDate.getWeekdayName();
            const dayA = mapA.get(dayStr);
            const dayB = mapB.get(dayStr);

            const lessonsA = dayA?.lessons ?? [];
            const lessonsB = dayB?.lessons ?? [];
            const isSaturday = dayDate.isSaturday();
            const calls = isSaturday ? config.timetable.saturday : config.timetable.weekdays;
            const maxPairs = Math.max(lessonsA.length, lessonsB.length, calls.length);

            const overlap: number[] = [];
            const free: number[] = [];
            const overlapDetails: string[] = [];
            const sameSubjects: string[] = [];

            for (let i = 0; i < maxPairs; i += 1) {
                const a = this.hasLesson(lessonsA[i]);
                const b = this.hasLesson(lessonsB[i]);

                if (a && b) {
                    overlap.push(i + 1);
                    const aText = this.lessonText(lessonsA[i]) ?? '\u2014';
                    const bText = this.lessonText(lessonsB[i]) ?? '\u2014';
                    const same = this.getSameSubjects(lessonsA[i], lessonsB[i]);
                    if (same.length) {
                        sameSubjects.push(`${i + 1}) ${same.join(' / ')}`);
                    }
                    overlapDetails.push(
                        `${i + 1}) ${aText} | ${bText}${same.length ? ' \u2014 \u0441\u043e\u0432\u043f\u0430\u0434\u0430\u0435\u0442' : ''}`
                    );
                }

                if (!a && !b && calls[i]) free.push(i + 1);
            }

            lines.push(`\n__ ${dayName}, ${dayStr} __`);
            lines.push(
                `\u0421\u043e\u0432\u043f\u0430\u0434\u0430\u044e\u0449\u0438\u0435 \u043f\u0430\u0440\u044b: ${overlap.length ? overlap.join(', ') : '\u043d\u0435\u0442'}`
            );
            if (overlapDetails.length) {
                lines.push(`\u041f\u0430\u0440\u044b:`);
                lines.push(overlapDetails.join('\n'));
            }
            lines.push(
                `\u041e\u0431\u0449\u0438\u0435 \u043f\u0440\u0435\u0434\u043c\u0435\u0442\u044b: ${sameSubjects.length ? sameSubjects.join(', ') : '\u043d\u0435\u0442'}`
            );
            lines.push(
                `\u041e\u0431\u0449\u0438\u0435 \u043e\u043a\u043d\u0430: ${free.length ? free.map((pair) => this.formatPair(pair, calls[pair - 1])).join(', ') : '\u043d\u0435\u0442'}`
            );
        }

        const message = lines.join('\n');

        if (initiator === 'callback') {
            return context.editOrSend(message, { keyboard: keyboard.MainMenu });
        }

        return context.send(message, { keyboard: keyboard.MainMenu });
    }

    private hasLesson(lesson: GroupLesson | undefined): boolean {
        if (!lesson) return false;
        if (Array.isArray(lesson)) return lesson.some((item) => item && item.lesson);
        return !!lesson.lesson;
    }

    private lessonText(lesson: GroupLesson | undefined): string | null {
        if (!lesson) return null;
        if (Array.isArray(lesson)) {
            const parts = lesson.map((item) => this.lessonExplainText(item)).filter((value) => value);
            return parts.length ? parts.join(' / ') : null;
        }
        return this.lessonExplainText(lesson);
    }

    private lessonExplainText(lesson: any): string | null {
        if (!lesson || !lesson.lesson) return null;
        const subgroup = lesson.subgroup ? `[${lesson.subgroup}] ` : '';
        const type = lesson.type ? ` ${lesson.type}` : '';
        const cabinet = lesson.cabinet ? ` {${lesson.cabinet}}` : '';
        const teacher = lesson.teacher ? ` - ${lesson.teacher}` : '';
        return `${subgroup}${lesson.lesson}${type}${cabinet}${teacher}`;
    }

    private getSameSubjects(a: GroupLesson | undefined, b: GroupLesson | undefined): string[] {
        const aSubjects = this.lessonSubjects(a);
        const bSubjects = this.lessonSubjects(b);
        if (aSubjects.length === 0 || bSubjects.length === 0) return [];
        const bNorm = new Set(bSubjects.map((value) => this.normalizeKey(value)));
        return aSubjects.filter((value) => bNorm.has(this.normalizeKey(value)));
    }

    private lessonSubjects(lesson: GroupLesson | undefined): string[] {
        if (!lesson) return [];
        if (Array.isArray(lesson)) {
            return lesson.map((item) => item?.lesson).filter((value) => value) as string[];
        }
        return lesson.lesson ? [lesson.lesson] : [];
    }

    private normalizeKey(value: string | null): string {
        return (value ?? '').toLowerCase().replace(/\s+/g, ' ').trim();
    }

    private formatPair(pairIndex: number, calls: [[string, string], [string, string]]): string {
        if (!calls || !calls[0] || !calls[1]) {
            return String(pairIndex);
        }

        const start = calls[0][0];
        const end = calls[1][1];
        return `${pairIndex} (${start}-${end})`;
    }
}
