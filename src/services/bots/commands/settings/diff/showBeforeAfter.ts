import { AbstractCommand, CmdHandlerParams } from "../../../abstract";

export default class extends AbstractCommand {
    public regexp = /^(✅|🚫)\sПоказывать "старое -> новое"$/i
    public payloadAction = null;
    public scene?: string | null = 'settings';

    handler({ context, keyboard, chat }: CmdHandlerParams) {
        chat.diffShowBeforeAfter = !chat.diffShowBeforeAfter;

        return context.send(
            [
                `Показывать старое -> новое для изменённых пар? Установлено: '${chat.diffShowBeforeAfter ? 'да' : 'нет'}'`,
                'Если включено, бот покажет обе версии пары в строках с типом "~".'
            ].join('\n'),
            {
                keyboard: keyboard.SettingsDiffAdvanced
            }
        );
    }
}
