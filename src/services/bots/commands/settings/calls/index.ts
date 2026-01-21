import { AbstractCommand, CmdHandlerParams } from "../../../abstract";
import { buildCallsMenu } from "./menu";

export default class extends AbstractCommand {
    public regexp = /^(\uD83D\uDD50\s)?\u0417\u0432\u043E\u043D\u043A\u0438: \u0443\u043F\u0440\u0430\u0432\u043B\u0435\u043D\u0438\u0435$/i;
    public scene?: string | null = 'settings_schedules';
    public payloadAction = null;

    handler({ context, chat, keyboard, serviceChat }: CmdHandlerParams) {
        chat.scene = 'settings_calls';
        const menu = buildCallsMenu(keyboard, serviceChat);
        return context.send(menu.text, { keyboard: menu.keyboard });
    }
}
