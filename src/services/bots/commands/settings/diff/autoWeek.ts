import { AbstractCommand, CmdHandlerParams } from "../../../abstract";

export default class extends AbstractCommand {
    public regexp = /^(✅|🚫)\sПоказывать diff после \/week$/i
    public payloadAction = null;
    public scene?: string | null = 'settings';

    handler({ context, keyboard, chat }: CmdHandlerParams) {
        chat.diffAutoInWeek = !chat.diffAutoInWeek;

        return context.send(
            [
                `Показывать diff после /week? Установлено: '${chat.diffAutoInWeek ? 'да' : 'нет'}'`,
                'Если включено, после недельного расписания бот сразу добавляет блок изменений.'
            ].join('\n'),
            {
                keyboard: keyboard.SettingsDiffAdvanced
            }
        );
    }
}
