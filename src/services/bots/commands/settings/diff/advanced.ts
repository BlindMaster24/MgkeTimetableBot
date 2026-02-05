import { AbstractCommand, CmdHandlerParams } from "../../../abstract";

export default class extends AbstractCommand {
    public regexp = /^(⚙️\s)?Расширенные$/i
    public payloadAction = null;
    public scene?: string | null = 'settings';

    handler({ context, keyboard, chat }: CmdHandlerParams) {
        chat.scene = 'settings';

        return context.send('Расширенные настройки раздела "Что изменилось".', {
            keyboard: keyboard.SettingsDiffAdvanced
        });
    }
}
