import { AbstractCommand, CmdHandlerParams } from "../../../abstract";
import { buildCallsMenu } from "./menu";

export default class extends AbstractCommand {
    public regexp = /^(\u2705\s)?\u0418\u0441\u0442\u043E\u0447\u043D\u0438\u043A:\s\u0430\u0432\u0442\u043E$/i;
    public scene?: string | null = 'settings_calls';
    public payloadAction = null;
    public adminOnly: boolean = true;

    async handler({ context, keyboard, serviceChat }: CmdHandlerParams) {
        await this.app.getService('parser').setCallsOverride(null);
        const menu = buildCallsMenu(keyboard, serviceChat);
        return context.editOrSend(`\u0418\u0441\u0442\u043E\u0447\u043D\u0438\u043A \u0437\u0432\u043E\u043D\u043A\u043E\u0432 \u043F\u0435\u0440\u0435\u043A\u043B\u044E\u0447\u0435\u043D \u043D\u0430 \u0430\u0432\u0442\u043E.\n\n${menu.text}`, {
            keyboard: menu.keyboard
        });
    }
}
