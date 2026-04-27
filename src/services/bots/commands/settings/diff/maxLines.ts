import { AbstractCommand, CmdHandlerParams } from '../../../abstract';

const PRESETS = [10, 20, 30, 50];

export default class extends AbstractCommand {
    public regexp = /^🧾\sЛимит строк:\s\d+$/i;
    public payloadAction = null;
    public scene?: string | null = 'settings';

    handler({ context, keyboard, chat }: CmdHandlerParams) {
        const current = PRESETS.includes(chat.diffMaxLines) ? chat.diffMaxLines : 20;
        const nextIndex = (PRESETS.indexOf(current) + 1) % PRESETS.length;
        chat.diffMaxLines = PRESETS[nextIndex];

        return context.send(
            [
                `Лимит строк diff: ${chat.diffMaxLines}`,
                'Когда изменений больше лимита, бот покажет только первые строки и общий остаток.'
            ].join('\n'),
            {
                keyboard: keyboard.SettingsDiff
            }
        );
    }
}
