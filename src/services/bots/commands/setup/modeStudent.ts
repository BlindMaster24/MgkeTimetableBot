import { defines } from '../../../../defines';
import { randArray } from '../../../../utils';
import { raspCache } from '../../../parser';
import { AbstractCommand, CmdHandlerParams } from '../../abstract';
import { StaticKeyboard } from '../../keyboard';

export default class extends AbstractCommand {
    public regexp = /^((👩‍🎓\s)?(Ученик|Учащийся)|(👨‍👩‍👦\s)?Родитель)$/i;
    public payloadAction = null;
    public scene?: string | null = 'setup';

    async handler(params: CmdHandlerParams) {
        const { context, chat, keyboard, service } = params;

        if (Object.keys(raspCache.groups.timetable).length == 0) {
            return context.send('Данные с сервера ещё не загружены, ожидайте...');
        }

        const randGroup = randArray(Object.keys(raspCache.groups.timetable));

        let group: string | number | false | undefined = await context
            .input(`Введите номер своей группы (например, ${randGroup})`, {
                keyboard: StaticKeyboard.Cancel
            })
            .then<string | undefined>((value) => {
                return value?.text;
            });

        while (true) {
            group = await this.findGroup(params, group);

            if (!group) {
                group = await context.waitInput().then<string | undefined>((value) => {
                    return value?.text;
                });
                continue;
            }

            break;
        }

        if (context.text.match(/^(👨‍👩‍👦\s)?Родитель$/i)) {
            chat.mode = 'parent';
        } else {
            chat.mode = 'student';
        }

        chat.group = group;
        chat.teacher = null;
        chat.scene = null;
        chat.deactivateSecondaryCheck = false;

        return context.send(defines[`${service}.message.about`], {
            keyboard: keyboard.MainMenu
        });
    }
}
