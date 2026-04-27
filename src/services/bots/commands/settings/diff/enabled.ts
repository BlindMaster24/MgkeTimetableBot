import { AbstractCommand, CmdHandlerParams } from '../../../abstract';

export default class extends AbstractCommand {
    public regexp = /^(✅|🚫)\sВключить раздел "Что изменилось"$/i;
    public payloadAction = null;
    public scene?: string | null = 'settings';

    handler({ context, keyboard, chat }: CmdHandlerParams) {
        chat.diffEnabled = !chat.diffEnabled;

        return context.send(
            [
                `Включить раздел "Что изменилось"? Установлено: '${chat.diffEnabled ? 'да' : 'нет'}'`,
                'Если отключено, кнопки/блоки diff пользователю не показываются.'
            ].join('\n'),
            {
                keyboard: keyboard.SettingsDiff
            }
        );
    }
}
