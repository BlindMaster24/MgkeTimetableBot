import { z } from "zod";
import { config } from "../../../../config";
import { DayCall } from "../../../../config.scheme";
import { nowInTime } from "../../../utils";
import { raspCache } from "../../parser";
import { AbstractCallback, ButtonType, CbHandlerParams, CmdHandlerParams } from "../abstract";

export default class CallsCallback extends AbstractCallback {
    public payloadAction: string = 'calls';

    async handler({ context, chat, keyboard }: CbHandlerParams | CmdHandlerParams) {
        let [showFull] = z.tuple([
            z.coerce.boolean()
        ]).parse(context.payload ?? [false]);

        const message: string[] = [];

        const activeSchedule = raspCache.calls.active.schedule.weekdays.length ? raspCache.calls.active.schedule : {
            weekdays: config.timetable.weekdays,
            saturday: config.timetable.saturday
        };

        let maxLessons: number = Math.max(
            activeSchedule.saturday.length,
            activeSchedule.weekdays.length
        );

        let current: number | undefined;

        if ((chat.mode === 'parent' || chat.mode === 'student') && chat.group) {
            current = Math.max(...raspCache.groups
                .timetable[chat.group]?.days
                .map(_ => _.lessons.length) || []
            );
        } else if (chat.mode === 'teacher' && chat.teacher) {
            current = Math.max(...raspCache.teachers
                .timetable[chat.teacher]?.days
                .map(_ => _.lessons.length) || []
            );
        }

        let userMaxLessons: number;

        if (!showFull && current) {
            userMaxLessons = current;
        } else {
            userMaxLessons = maxLessons;
        }

        if (raspCache.calls.active.source === 'manual' && raspCache.calls.manualReason) {
            message.push(`\u041F\u0440\u0438\u0447\u0438\u043D\u0430: ${raspCache.calls.manualReason}\n`);
        }
        
        message.push('__ \u0417\u0432\u043E\u043D\u043A\u0438 (\u0431\u0443\u0434\u043D\u0438) __');
        message.push(this._getMessage(activeSchedule.weekdays, [1, 2, 3, 4, 5], userMaxLessons, showFull));

        message.push('\n__ \u0417\u0432\u043E\u043D\u043A\u0438 (\u0441\u0443\u0431\u0431\u043E\u0442\u0430) __');
        message.push(this._getMessage(activeSchedule.saturday, [6], userMaxLessons, showFull));

        if (current && current >= maxLessons) {
            showFull = true;
        }

        return context.editOrSend(message.join('\n'), {
            keyboard: showFull ? undefined : keyboard.getKeyboardBuilder('CallsFull', true).add({
                type: ButtonType.Callback,
                text: '\u041F\u043E\u043A\u0430\u0437\u0430\u0442\u044C \u043F\u043E\u043B\u043D\u043E\u0441\u0442\u044C\u044E',
                payload: this.payloadAction + JSON.stringify([
                    Number(true)
                ])
            })
        });
    }

    private setSelected(text: string, selected: boolean): string {
        if (!selected) {
            return text;
        }

        return `\uD83D\uDC49 ${text} \uD83D\uDC48`;
    }

    private _getMessage(calls: DayCall[], includedDays: number[], maxLessons: number, showFull: boolean) {
        const text: string[] = [];

        for (let i = 0; i < maxLessons; i++) {
            const lesson = calls[i];
            if (!lesson) break;

            const lineStr: string = `${i + 1}. ${lesson[0][0]} - ${lesson[0][1]} | ${lesson[1][0]} - ${lesson[1][1]}`;

            text.push(this.setSelected(lineStr, !showFull && nowInTime(includedDays, lesson[0][0], lesson[1][1])));
        }

        return text.join('\n');
    }
}
