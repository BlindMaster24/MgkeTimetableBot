import type { TelegramBotCommand } from '../../types/telegram';
import { AbstractCommand, CmdHandlerParams } from '../../abstract';
import CallsCallback from '../../callbacks/calls';

export default class extends AbstractCommand {
    public regexp = /^((!|\/)(get)?(times|calls)|(🕐\s)?звонки)$/i;
    public payloadAction = null;
    public tgCommand: TelegramBotCommand = {
        command: 'calls',
        description: 'Расписание звонков'
    };

    handler(params: CmdHandlerParams) {
        const callback: CallsCallback = this.app.getService('bot').getCallbackById('calls');

        return callback.handler(params);
    }
}
