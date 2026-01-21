import { AbstractCommand, CmdHandlerParams } from "../../../abstract";
import { buildCallsMenu } from "./menu";

const sourceLabel = (source: 'site' | 'manual' | 'config') => {
    if (source === 'site') return '\u0441\u0430\u0439\u0442';
    if (source === 'manual') return '\u0432\u0440\u0443\u0447\u043d\u0443\u044e';
    return '\u043a\u043e\u043d\u0444\u0438\u0433';
};

export default class extends AbstractCommand {
    public regexp = /^(\u2705\s)?\u0418\u0441\u0442\u043E\u0447\u043D\u0438\u043A:\s(\u0441\u0430\u0439\u0442|\u0432\u0440\u0443\u0447\u043d\u0443\u044e|\u043a\u043e\u043d\u0444\u0438\u0433)$/i;
    public scene?: string | null = 'settings_calls';
    public payloadAction = null;
    public adminOnly: boolean = true;

    async handler({ context, keyboard, serviceChat }: CmdHandlerParams) {
        const text = context.text.toLowerCase();
        let source: 'site' | 'manual' | 'config';

        if (text.includes('\u0432\u0440\u0443\u0447')) {
            source = 'manual';
        } else if (text.includes('\u043a\u043e\u043d\u0444')) {
            source = 'config';
        } else {
            source = 'site';
        }

        await this.app.getService('parser').setCallsOverride(source);
        const menu = buildCallsMenu(keyboard, serviceChat);
        return context.editOrSend(`\u0418\u0441\u0442\u043E\u0447\u043D\u0438\u043A \u0437\u0432\u043E\u043D\u043A\u043E\u0432 \u043F\u0435\u0440\u0435\u043A\u043B\u044E\u0447\u0435\u043D \u043D\u0430 ${sourceLabel(source)}.\n\n${menu.text}`, {
            keyboard: menu.keyboard
        });
    }
}
