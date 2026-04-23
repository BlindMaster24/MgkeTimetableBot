import { AbstractCommand, CmdHandlerParams } from '../../../abstract';

export default class extends AbstractCommand {
    public regexp = /^(🔇|🔈)\sОповещение о звонках(\:\s(да|нет))?$/i;
    public payloadAction = null;
    public scene?: string | null = 'settings';

    handler({ context, chat, keyboard }: CmdHandlerParams) {
        chat.noticeCalls = !chat.noticeCalls;

        return context.send(
            `Оповещение об изменениях расписания звонков: ${chat.noticeCalls ? 'включено' : 'выключено'}`,
            {
                keyboard: keyboard.SettingsNotice
            }
        );
    }
}
