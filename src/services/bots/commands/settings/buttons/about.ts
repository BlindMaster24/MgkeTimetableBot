import { AbstractCommand, CmdHandlerParams } from '../../../abstract';

export default class extends AbstractCommand {
    public regexp = /^(✅|🚫)\sКнопка "(💡\s)?О боте"$/i;
    public payloadAction = null;
    public scene: string = 'settings';

    handler({ context, chat, keyboard }: CmdHandlerParams) {
        chat.showAbout = !chat.showAbout;

        return context.send(`Показывать кнопку "💡 О боте"? Установлено: '${chat.showAbout ? 'да' : 'нет'}'`, {
            keyboard: keyboard.SettingsButtons
        });
    }
}
