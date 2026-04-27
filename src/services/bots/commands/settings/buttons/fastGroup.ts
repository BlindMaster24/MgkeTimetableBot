import { AbstractCommand, CmdHandlerParams } from '../../../abstract';

export default class extends AbstractCommand {
    public regexp = /^(✅|🚫)\sКнопка "(👩‍🎓\s)?Группа"$/i;
    public payloadAction = null;
    public scene?: string | null = 'settings';

    handler({ context, chat, keyboard }: CmdHandlerParams) {
        chat.showFastGroup = !chat.showFastGroup;

        return context.send(`Показывать кнопку "👩‍🎓 Группа"? Установлено: '${chat.showFastGroup ? 'да' : 'нет'}'`, {
            keyboard: keyboard.SettingsButtons
        });
    }
}
