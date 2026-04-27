import { AbstractCommand, CmdHandlerParams } from '../../../abstract';

export default class extends AbstractCommand {
    public regexp = /^(✅|🚫)\sПоказывать подсказки(\:\s(да|нет))?$/i;
    public payloadAction = null;
    public scene?: string | null = 'settings';

    handler({ context, chat, keyboard }: CmdHandlerParams) {
        chat.showHints = !chat.showHints;

        return context.send(
            `Показывать ли подсказки под расписанием? Установлено: '${chat.showHints ? 'да' : 'нет'}'`,
            {
                keyboard: keyboard.SettingsView
            }
        );
    }
}
