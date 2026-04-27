import { AbstractCommand, CmdHandlerParams } from '../../../abstract';

export default class extends AbstractCommand {
    public regexp = /^(✅|🚫)\sКнопка "(📄\s)?На день"$/i;
    public payloadAction = null;
    public scene?: string | null = 'settings';

    handler({ context, chat, keyboard }: CmdHandlerParams) {
        chat.showDaily = !chat.showDaily;

        return context.send(`Показывать кнопку "📄 На день"? Установлено: '${chat.showDaily ? 'да' : 'нет'}'`, {
            keyboard: keyboard.SettingsButtons
        });
    }
}
