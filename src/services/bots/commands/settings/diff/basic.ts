import { AbstractCommand, CmdHandlerParams } from '../../../abstract';

export default class extends AbstractCommand {
    public regexp = /^(⬅️\s)?Базовые настройки$/i;
    public payloadAction = null;
    public scene?: string | null = 'settings';

    handler({ context, keyboard, chat }: CmdHandlerParams) {
        chat.scene = 'settings';

        return context.send('Базовые настройки раздела "Что изменилось".', {
            keyboard: keyboard.SettingsDiff
        });
    }
}
