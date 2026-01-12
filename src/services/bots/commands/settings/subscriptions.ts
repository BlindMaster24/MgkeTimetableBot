import { TelegramBotCommand } from "puregram/generated";
import { randArray } from "../../../../utils";
import { raspCache } from "../../../parser";
import { AbstractCommand, CmdHandlerParams } from "../../abstract";
import { Subscription } from "../../subscriptions/model";

type SubscriptionEntry = {
    id: number;
    type: 'group' | 'teacher';
    value: string;
};

const MAX_SUBSCRIPTIONS = 5;

export default class SubscriptionsCommand extends AbstractCommand {
    public regexp = {
        index: /^((!|\/)subscriptions)|^(🔔\s)?Подписки$/i,
        addGroup: /^➕\s?Группа$/i,
        addTeacher: /^➕\s?Преподаватель$/i,
        list: /^📋\s?Мои подписки$/i,
        remove: /^❌\s?Удалить подписку$/i
    };
    public payloadAction = null;
    public tgCommand: TelegramBotCommand = {
        command: 'subscriptions',
        description: 'Управление подписками на другие группы/преподавателей'
    };

    async handler(params: CmdHandlerParams<SubscriptionsCommand>) {
        const { context, keyboard, regexp } = params;

        if (regexp === 'index') {
            return context.send(
                'Подписки позволяют получать уведомления об изменениях расписания другой группы или преподавателя.',
                { keyboard: keyboard.SubscriptionsMenu }
            );
        }

        if (regexp === 'list') {
            const list = await this.getSubscriptions(params);
            if (list.length === 0) {
                return context.send('Подписок нет.', { keyboard: keyboard.SubscriptionsMenu });
            }

            return context.send(this.formatSubscriptions(list), { keyboard: keyboard.SubscriptionsMenu });
        }

        if (regexp === 'remove') {
            const list = await this.getSubscriptions(params);
            if (list.length === 0) {
                return context.send('Подписок нет.', { keyboard: keyboard.SubscriptionsMenu });
            }

            const prompt = [
                'Введите номер подписки для удаления:',
                this.formatSubscriptions(list)
            ].join('\n');

            const selected = await context.input(prompt, { keyboard: keyboard.SubscriptionsMenu });
            const index = selected?.text ? Number(selected.text) : NaN;
            if (!Number.isFinite(index) || index < 1 || index > list.length) {
                return context.send('Неверный номер подписки.', { keyboard: keyboard.SubscriptionsMenu });
            }

            const target = list[index - 1];
            await Subscription.destroy({
                where: {
                    id: target.id,
                    chatId: params.chat.id
                }
            });

            return context.send('Подписка удалена.', { keyboard: keyboard.SubscriptionsMenu });
        }

        if (regexp === 'addGroup') {
            return this.addGroupSubscription(params);
        }

        if (regexp === 'addTeacher') {
            return this.addTeacherSubscription(params);
        }

        return context.send('Неизвестная команда.', { keyboard: keyboard.SubscriptionsMenu });
    }

    private async addGroupSubscription(params: CmdHandlerParams) {
        const { context, keyboard } = params;
        if (Object.keys(raspCache.groups.timetable).length === 0) {
            return context.send('Данные с сервера ещё не загружены, ожидайте...', { keyboard: keyboard.SubscriptionsMenu });
        }

        const count = await Subscription.count({ where: { chatId: params.chat.id } });
        if (count >= MAX_SUBSCRIPTIONS) {
            return context.send(`Достигнут лимит подписок (${MAX_SUBSCRIPTIONS}).`, { keyboard: keyboard.SubscriptionsMenu });
        }

        const randGroup = randArray(Object.keys(raspCache.groups.timetable));
        let group: string | undefined = await context.input(
            `Введите номер группы, на которую хотите подписаться (например, ${randGroup})`,
            { keyboard: keyboard.GroupHistory }
        ).then(value => value?.text);

        while (true) {
            const selected = await this.findGroup(params, group, keyboard.SubscriptionsMenu);
            if (selected === false) {
                return;
            }
            if (!selected) {
                return;
            }
            group = selected;
            break;
        }

        const [record, created] = await this.retrySqliteBusy(() => {
            return Subscription.findOrCreate({
                where: {
                    chatId: params.chat.id,
                    type: 'group',
                    value: group
                },
                defaults: {
                    chatId: params.chat.id,
                    type: 'group',
                    value: group
                }
            });
        });

        if (!created && record) {
            return context.send('Такая подписка уже существует.', { keyboard: keyboard.SubscriptionsMenu });
        }

        return context.send(`Подписка на группу ${group} добавлена.`, { keyboard: keyboard.SubscriptionsMenu });
    }

    private async addTeacherSubscription(params: CmdHandlerParams) {
        const { context, keyboard } = params;
        if (Object.keys(raspCache.teachers.timetable).length === 0) {
            return context.send('Данные с сервера ещё не загружены, ожидайте...', { keyboard: keyboard.SubscriptionsMenu });
        }

        const count = await Subscription.count({ where: { chatId: params.chat.id } });
        if (count >= MAX_SUBSCRIPTIONS) {
            return context.send(`Достигнут лимит подписок (${MAX_SUBSCRIPTIONS}).`, { keyboard: keyboard.SubscriptionsMenu });
        }

        const randTeacher = randArray(Object.keys(raspCache.teachers.timetable));
        let teacher: string | undefined = await context.input(
            `Введите фамилию преподавателя (например, ${randTeacher})`,
            { keyboard: keyboard.TeacherHistory }
        ).then(value => value?.text);

        while (true) {
            const selected = await this.findTeacher(params, teacher, keyboard.SubscriptionsMenu);
            if (selected === false) {
                return;
            }
            if (!selected) {
                return;
            }
            teacher = selected;
            break;
        }

        const [record, created] = await this.retrySqliteBusy(() => {
            return Subscription.findOrCreate({
                where: {
                    chatId: params.chat.id,
                    type: 'teacher',
                    value: teacher
                },
                defaults: {
                    chatId: params.chat.id,
                    type: 'teacher',
                    value: teacher
                }
            });
        });

        if (!created && record) {
            return context.send('Такая подписка уже существует.', { keyboard: keyboard.SubscriptionsMenu });
        }

        return context.send(`Подписка на преподавателя ${teacher} добавлена.`, { keyboard: keyboard.SubscriptionsMenu });
    }

    private async getSubscriptions({ chat }: CmdHandlerParams): Promise<SubscriptionEntry[]> {
        return Subscription.findAll({
            where: {
                chatId: chat.id
            },
            attributes: ['id', 'type', 'value'],
            order: [['id', 'ASC']]
        }).then((items) => items.map((item) => item.get({ plain: true }) as SubscriptionEntry));
    }

    private formatSubscriptions(list: SubscriptionEntry[]): string {
        return list.map((item, index) => {
            if (item.type === 'group') {
                return `${index + 1}. Группа ${item.value}`;
            }
            return `${index + 1}. Преподаватель ${item.value}`;
        }).join('\n');
    }

    private async retrySqliteBusy<T>(action: () => Promise<T>, attempts: number = 3): Promise<T> {
        let lastError: unknown;
        for (let i = 0; i < attempts; i++) {
            try {
                return await action();
            } catch (error: any) {
                const message = typeof error?.message === 'string' ? error.message : '';
                if (message.includes('SQLITE_BUSY')) {
                    lastError = error;
                    await new Promise(resolve => setTimeout(resolve, 200 * (i + 1)));
                    continue;
                }
                throw error;
            }
        }
        throw lastError;
    }
}
