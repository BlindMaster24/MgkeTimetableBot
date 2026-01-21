import { AbstractCommand, CmdHandlerParams } from "../../../abstract";

export default class extends AbstractCommand {
    public regexp = /^\u2705\s\u041E\u0431\u043D\u043E\u0432\u0438\u0442\u044C \u0441 \u0441\u0430\u0439\u0442\u0430$/i;
    public scene?: string | null = 'settings_calls';
    public payloadAction = null;
    public adminOnly: boolean = true;

    async handler({ context }: CmdHandlerParams) {
        const report = await this.app.getService('parser').refreshCallsNow();

        const sourceLabel = (value: 'site' | 'manual' | 'config') => {
            if (value === 'site') return '\u0441\u0430\u0439\u0442';
            if (value === 'manual') return '\u0432\u0440\u0443\u0447\u043d\u0443\u044e';
            return '\u043a\u043e\u043d\u0444\u0438\u0433';
        };

        const lines: string[] = [];
        lines.push('\uD83D\uDD04 \u041E\u0431\u043D\u043E\u0432\u043B\u0435\u043D\u0438\u0435 \u0437\u0432\u043E\u043D\u043A\u043E\u0432');

        if (report.error) {
            lines.push('\u26A0\uFE0F \u041E\u0448\u0438\u0431\u043A\u0430 \u043F\u0430\u0440\u0441\u0435\u0440\u0430');
        } else if (!report.siteParsed) {
            lines.push('\u26A0\uFE0F \u0421\u0430\u0439\u0442 \u043E\u0442\u0434\u0430\u043B \u043F\u0443\u0441\u0442\u043E');
        } else {
            lines.push('\u2705 \u0421\u0430\u0439\u0442 \u0443\u0441\u043F\u0435\u0448\u043D\u043E \u0441\u043F\u0430\u0440\u0441\u0435\u043D');
        }

        lines.push(`\uD83E\uDDEA \u0420\u0435\u0437\u0443\u043B\u044C\u0442\u0430\u0442 \u043F\u0430\u0440\u0441\u0438\u043D\u0433\u0430: ${report.siteParsed ? 'OK' : 'EMPTY'}`);

        if (report.updatedAtRaw) {
            lines.push(`\uD83D\uDCC5 \u0414\u0430\u0442\u0430 \u043D\u0430 \u0441\u0430\u0439\u0442\u0435: ${report.updatedAtRaw}`);
        }

        const active = report.override ?? report.active;
        lines.push(`\uD83D\uDCCC \u0410\u043A\u0442\u0438\u0432\u043D\u044B\u0439 \u0438\u0441\u0442\u043E\u0447\u043D\u0438\u043A: ${sourceLabel(active)}`);

        if (report.reason && report.active === 'manual') {
            lines.push(`\u270D\uFE0F \u041F\u0440\u0438\u0447\u0438\u043D\u0430: ${report.reason}`);
        }

        if (report.error) {
            lines.push(`\uD83E\uDEA0 ${report.error.message}`);
        }

        if (report.schedule && report.siteParsed) {
            lines.push('\n__ \u0417\u0432\u043E\u043D\u043A\u0438 (\u0431\u0443\u0434\u043D\u0438) __');
            lines.push(this.formatCalls(report.schedule.weekdays));
            lines.push('\n__ \u0417\u0432\u043E\u043D\u043A\u0438 (\u0441\u0443\u0431\u0431\u043E\u0442\u0430) __');
            lines.push(this.formatCalls(report.schedule.saturday));
        }

        return context.send(lines.join('\n'));
    }

    private formatCalls(calls: [string, string][][]): string {
        const text: string[] = [];
        for (let i = 0; i < calls.length; i++) {
            const lesson = calls[i];
            if (!lesson) break;
            const lineStr: string = `${i + 1}. ${lesson[0][0]} - ${lesson[0][1]} | ${lesson[1][0]} - ${lesson[1][1]}`;
            text.push(lineStr);
        }
        return text.join('\n');
    }
}
