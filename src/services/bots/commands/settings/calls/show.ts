import { DayCall } from "../../../../../../config.scheme";
import { AbstractCommand, CmdHandlerParams } from "../../../abstract";
import { nowInTime } from "../../../../../utils";
import { raspCache } from "../../../../parser";
import { config } from "../../../../../../config";

export default class extends AbstractCommand {
    public regexp = /^\uD83D\uDCCA\s\u041F\u043E\u043A\u0430\u0437\u0430\u0442\u044C$/i;
    public scene?: string | null = 'settings_calls';
    public payloadAction = null;

    async handler({ context }: CmdHandlerParams) {
        const activeSchedule = raspCache.calls.active.schedule.weekdays.length ? raspCache.calls.active.schedule : {
            weekdays: config.timetable.weekdays,
            saturday: config.timetable.saturday
        };

        const message: string[] = [];
        const maxLessons = Math.max(activeSchedule.weekdays.length, activeSchedule.saturday.length);
        const userMaxLessons = maxLessons;

        if (raspCache.calls.active.source === 'manual' && raspCache.calls.manualReason) {
            message.push(`\u041F\u0440\u0438\u0447\u0438\u043D\u0430: ${raspCache.calls.manualReason}\n`);
        }

        message.push('__ \u0417\u0432\u043E\u043D\u043A\u0438 (\u0431\u0443\u0434\u043D\u0438) __');
        message.push(this.getMessage(activeSchedule.weekdays, [1, 2, 3, 4, 5], userMaxLessons));

        message.push('\n__ \u0417\u0432\u043E\u043D\u043A\u0438 (\u0441\u0443\u0431\u0431\u043E\u0442\u0430) __');
        message.push(this.getMessage(activeSchedule.saturday, [6], userMaxLessons));

        return context.send(message.join('\n'));
    }

    private getMessage(calls: DayCall[], includedDays: number[], maxLessons: number) {
        const text: string[] = [];

        for (let i = 0; i < maxLessons; i++) {
            const lesson = calls[i];
            if (!lesson) break;

            const lineStr: string = `${i + 1}. ${lesson[0][0]} - ${lesson[0][1]} | ${lesson[1][0]} - ${lesson[1][1]}`;
            const selected = nowInTime(includedDays, lesson[0][0], lesson[1][1]);
            text.push(selected ? `\uD83D\uDC49 ${lineStr} \uD83D\uDC48` : lineStr);
        }

        return text.join('\n');
    }
}
