import { defines } from '../../../../defines';
import { randArray } from '../../../../utils';
import { raspCache } from '../../../parser';
import { AbstractCommand, CmdHandlerParams } from '../../abstract';
import { StaticKeyboard } from '../../keyboard';

export default class extends AbstractCommand {
    public regexp = /^(👩‍🏫\s)?(Учитель|Преподаватель)$/i;
    public payloadAction = null;
    public scene?: string | null = 'setup';

    async handler(params: CmdHandlerParams) {
        const { context, chat, keyboard, service } = params;

        if (Object.keys(raspCache.teachers.timetable).length == 0) {
            return context.send('Данные с сервера ещё не загружены, ожидайте...');
        }

        const randTeacher = randArray(Object.keys(raspCache.teachers.timetable));

        let teacher: string | null | false | undefined = await context
            .input(`Введите фамилию преподавателя (например, ${randTeacher})`, {
                keyboard: StaticKeyboard.Cancel
            })
            .then<string | undefined>((value) => {
                return value?.text;
            });

        while (true) {
            teacher = await this.findTeacher(params, teacher);

            if (!teacher) {
                teacher = await context.waitInput().then<string | undefined>((value) => {
                    return value?.text;
                });
                continue;
            }

            break;
        }

        chat.teacher = teacher;
        chat.mode = 'teacher';
        chat.group = null;
        chat.scene = null;
        chat.deactivateSecondaryCheck = false;

        return context.send(defines[`${service}.message.about`], {
            keyboard: keyboard.MainMenu
        });
    }
}
