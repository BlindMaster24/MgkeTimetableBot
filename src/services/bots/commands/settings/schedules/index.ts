import { AbstractCommand, CmdHandlerParams } from '../../../abstract';

export default class extends AbstractCommand {
    public regexp = /^(🗓️\s)?Управление расписаниями$/i;
    public payloadAction = null;
    public scene?: string | null = 'settings';

    handler({ context, chat, keyboard }: CmdHandlerParams) {
        chat.scene = 'settings_schedules';

        return context.send('Управление расписаниями.', {
            keyboard: keyboard.SettingsSchedules
        });
    }
}
