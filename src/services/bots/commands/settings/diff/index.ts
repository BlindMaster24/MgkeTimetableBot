import { TelegramBotCommand } from "puregram/generated";
import { AbstractCommand, CmdHandlerParams } from "../../../abstract";

export default class extends AbstractCommand {
    public regexp = /^((!|\/)diff(settings)?)|(📊\s)?(Сравнение)$/i
    public payloadAction = null;
    public scene?: string | null = 'settings';
    public tgCommand: TelegramBotCommand = {
        command: 'diff',
        description: 'Настройки отображения изменений расписания'
    };

    handler({ context, keyboard, chat }: CmdHandlerParams) {
        chat.scene = 'settings';

        return context.send('Меню настроек раздела "Что изменилось".', {
            keyboard: keyboard.SettingsDiff
        });
    }
}
