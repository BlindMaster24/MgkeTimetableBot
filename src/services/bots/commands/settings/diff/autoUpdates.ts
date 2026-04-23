import { AbstractCommand, CmdHandlerParams } from '../../../abstract';

export default class extends AbstractCommand {
    public regexp = /^(✅|🚫)\sПоказывать diff в уведомлениях$/i;
    public payloadAction = null;
    public scene?: string | null = 'settings';

    handler({ context, keyboard, chat }: CmdHandlerParams) {
        chat.diffAutoInUpdates = !chat.diffAutoInUpdates;

        return context.send(
            [
                `Показывать diff в уведомлениях? Установлено: '${chat.diffAutoInUpdates ? 'да' : 'нет'}'`,
                'Если включено, в автоуведомлениях о сменах будет краткий список изменений.'
            ].join('\n'),
            {
                keyboard: keyboard.SettingsDiffAdvanced
            }
        );
    }
}
