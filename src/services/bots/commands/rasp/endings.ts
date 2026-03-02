import type { TelegramBotCommand } from '../../types/telegram';
import { getDayRasp } from "../../../../utils";
import { raspCache } from "../../../parser";
import { AbstractCommand, CmdHandlerParams } from "../../abstract";

export default class extends AbstractCommand {
    public regexp = /^((!|\/)?endings)$/i;
    public payloadAction = null;
    public tgCommand: TelegramBotCommand = {
        command: 'endings',
        description: 'Отображает сколько групп заканчивают к определённой паре'
    };
    public scene?: string | null = null;

    async handler({ context }: CmdHandlerParams) {
        const groups = raspCache.groups.timetable;

        const stat: {
            [day: string]: {
                [lesson: number]: number
            }
        } = {}

        for (const groupIndex in groups) {
            const group = groups[groupIndex];

            const days = getDayRasp(group.days, false);
            //const days = group.days.slice(0, 2); //test

            for (const entry of days) {
                const day = entry.day;
                let lastLessonIndex = -1;
                for (let i = 0; i < entry.lessons.length; i += 1) {
                    const lesson = entry.lessons[i];
                    if (!lesson) continue;
                    if (Array.isArray(lesson)) {
                        if (lesson.some((item) => item && item.lesson)) {
                            lastLessonIndex = i;
                        }
                    } else if (lesson.lesson) {
                        lastLessonIndex = i;
                    }
                }

                if (lastLessonIndex === -1) {
                    //нет пар у этой группы
                    continue;
                }

                if (stat[day] === undefined) {
                    stat[day] = {};
                }

                const lessons = lastLessonIndex + 1;
                if (stat[day][lessons] === undefined) {
                    stat[day][lessons] = 0;
                }

                stat[day][lessons] += 1;
            }
        }

        if (Object.keys(stat).length === 0) {
            return context.send('Нет данных для отображения');
        }

        //await context.send(JSON.stringify(stat, null, 1));

        const message: string[] = [];
        for (const day in stat) {
            const part: string[] = [];

            part.push(`__ ${day} __`);

            for (const lesson in stat[day]) {
                const count = stat[day][lesson];

                part.push(`${count} групп заканчивают к ${lesson} паре`);
            }

            message.push(part.join('\n'));
        }

        return context.send(message.join('\n\n'))
    }
}
