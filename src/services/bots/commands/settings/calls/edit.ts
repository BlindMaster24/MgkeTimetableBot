import { DayCall } from "../../../../../../config.scheme";
import { CallsSchedule } from "../../../../parser/calls";
import { AbstractCommand, CmdHandlerParams, KeyboardBuilder, KeyboardColor } from "../../../abstract";
import { StaticKeyboard, withCancelButton } from "../../../keyboard";

const timeRegex = /\b\d{1,2}[:.]\d{2}\b/g;

const normalizeTime = (value: string) => {
    const [h, m] = value.replace('.', ':').split(':');
    const hh = h.padStart(2, '0');
    return `${hh}:${m}`;
};

const parseRowTimes = (text: string): DayCall | null => {
    const times = Array.from(text.matchAll(timeRegex)).map((match) => normalizeTime(match[0]));
    if (times.length < 4) {
        return null;
    }
    return [[times[0], times[1]], [times[2], times[3]]];
};

const parseSchedule = (input: string): CallsSchedule | null => {
    const weekdays: DayCall[] = [];
    const saturday: DayCall[] = [];
    let target: DayCall[] = weekdays;

    for (const rawLine of input.split(/\r?\n/)) {
        const line = rawLine.trim();
        if (!line) continue;
        const lower = line.toLowerCase();
        if (lower.includes('\u0441\u0443\u0431\u0431\u043e\u0442') || lower.includes('\u0432\u044b\u0445\u043e\u0434')) {
            target = saturday;
            continue;
        }
        if (lower.includes('\u0431\u0443\u0434\u043d')) {
            target = weekdays;
            continue;
        }
        const call = parseRowTimes(line);
        if (call) {
            target.push(call);
        }
    }

    if (!weekdays.length && !saturday.length) {
        return null;
    }

    return {
        weekdays,
        saturday: saturday.length ? saturday : weekdays
    };
};

export default class extends AbstractCommand {
    public regexp = /^\u270F\uFE0F\s\u0418\u0437\u043C\u0435\u043D\u0438\u0442\u044C \u0432\u0440\u0443\u0447\u043D\u0443\u044E$/i;
    public scene?: string | null = 'settings_calls';
    public payloadAction = null;
    public adminOnly: boolean = true;

    async handler({ context }: CmdHandlerParams) {
        const input = await context.input(
            '\u0412\u0432\u0435\u0434\u0438\u0442\u0435 \u0440\u0430\u0441\u043F\u0438\u0441\u0430\u043D\u0438\u0435 \u0437\u0432\u043E\u043D\u043A\u043E\u0432. \u041F\u0440\u0438\u043C\u0435\u0440\n\u0411\u0443\u0434\u043D\u0438\n1 08:30 09:15 09:25 10:10\n2 10:20 11:05 11:15 12:00\n\u0421\u0443\u0431\u0431\u043E\u0442\u0430\n1 09:00 09:45 09:55 10:40',
            { keyboard: withCancelButton(StaticKeyboard.Cancel) }
        );

        const schedule = parseSchedule(input?.text ?? '');
        if (!schedule) {
            return context.send('\u041D\u0435 \u0443\u0434\u0430\u043B\u043E\u0441\u044C \u0440\u0430\u0441\u043F\u043E\u0437\u043D\u0430\u0442\u044C \u0440\u0430\u0441\u043F\u0438\u0441\u0430\u043D\u0438\u0435. \u041F\u0440\u043E\u0432\u0435\u0440\u044C\u0442\u0435 \u0444\u043E\u0440\u043C\u0430\u0442.');
        }

        const reasonKeyboard = withCancelButton(new KeyboardBuilder('CallsReason', true).add({
            text: '\u041F\u0440\u043E\u043F\u0443\u0441\u0442\u0438\u0442\u044C',
            color: KeyboardColor.SECONDARY_COLOR
        }));

        const reasonAnswer = await context.input(
            '\u0423\u043A\u0430\u0436\u0438\u0442\u0435 \u043F\u0440\u0438\u0447\u0438\u043D\u0443 \u0438\u0437\u043C\u0435\u043D\u0435\u043D\u0438\u044F \u0438\u043B\u0438 \u043D\u0430\u0436\u043C\u0438\u0442\u0435 \u041F\u0440\u043E\u043F\u0443\u0441\u0442\u0438\u0442\u044C',
            { keyboard: reasonKeyboard }
        );

        const reasonText = reasonAnswer?.text?.trim() ?? '';
        const reason = !reasonText || /^(\u043F\u0440\u043E\u043F\u0443\u0441\u0442\u0438\u0442\u044C|\u0441\u043A\u0438\u043F)$/i.test(reasonText) ? null : reasonText;

        const confirmKeyboard = withCancelButton(new KeyboardBuilder('CallsConfirm', true).add({
            text: '\u041E\u0442\u043F\u0440\u0430\u0432\u0438\u0442\u044C',
            color: KeyboardColor.POSITIVE_COLOR
        }).add({
            text: '\u041D\u0435 \u043E\u0442\u043F\u0440\u0430\u0432\u043B\u044F\u0442\u044C',
            color: KeyboardColor.SECONDARY_COLOR
        }));

        const confirmAnswer = await context.input(
            '\u041E\u0442\u043F\u0440\u0430\u0432\u0438\u0442\u044C \u0443\u0432\u0435\u0434\u043E\u043C\u043B\u0435\u043D\u0438\u0435 \u0432\u0441\u0435\u043C?',
            { keyboard: confirmKeyboard }
        );

        const confirmText = (confirmAnswer?.text ?? '').toLowerCase();
        const notifyNow = /(\u043E\u0442\u043F\u0440\u0430\u0432\u0438\u0442\u044C)/i.test(confirmText);

        await this.app.getService('parser').setManualCalls(schedule, reason, notifyNow);

        return context.send(notifyNow
            ? '\u0420\u0430\u0441\u043F\u0438\u0441\u0430\u043D\u0438\u0435 \u0437\u0432\u043E\u043D\u043A\u043E\u0432 \u043E\u0431\u043D\u043E\u0432\u043B\u0435\u043D\u043E \u0438 \u0443\u0432\u0435\u0434\u043E\u043C\u043B\u0435\u043D\u0438\u0435 \u043E\u0442\u043F\u0440\u0430\u0432\u043B\u0435\u043D\u043E.'
            : '\u0420\u0430\u0441\u043F\u0438\u0441\u0430\u043D\u0438\u0435 \u0437\u0432\u043E\u043D\u043A\u043E\u0432 \u043E\u0431\u043D\u043E\u0432\u043B\u0435\u043D\u043E \u0432\u0440\u0443\u0447\u043D\u0443\u044E.');
    }
}
