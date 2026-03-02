import type { TelegramBotCommand } from '../../types/telegram';
import { DayIndex, WeekIndex, getFutureDays } from "../../../../utils";
import { GroupDay, TeacherDay } from "../../../parser/types";
import { AbstractCommand, CmdHandlerParams } from "../../abstract";
import { StaticKeyboard } from "../../keyboard";
import { Subscription } from "../../subscriptions/model";

type SubscriptionEntry = {
    id: number;
    type: 'group' | 'teacher';
    value: string;
};

export default class SubscriptionsTestCommand extends AbstractCommand {
    public regexp = /^((!|\/)subscriptions_test)|^(🧪\s)?Проверить$/i;
    public payloadAction = null;
    public tgCommand: TelegramBotCommand = {
        command: 'subscriptions_test',
        description: 'Тестовое уведомление по подпискам'
    };

    async handler(params: CmdHandlerParams) {
        const { context, keyboard, formatter, chat } = params;
        const list = await this.getSubscriptions(chat.id);
        if (list.length === 0) {
            return context.send('Подписок нет.', { keyboard: keyboard.SubscriptionsMenu });
        }

        const prompt = [
            'Введите номер подписки для проверки:',
            this.formatSubscriptions(list)
        ].join('\n');

        const selected = await context.input(prompt, { keyboard: keyboard.SubscriptionsMenu });
        const input = selected?.text?.trim();
        if (!input) {
            return context.send('Неверный номер подписки.', { keyboard: keyboard.SubscriptionsMenu });
        }

        const index = Number(input);
        let target: SubscriptionEntry | undefined;
        if (Number.isFinite(index) && index >= 1 && index <= list.length) {
            target = list[index - 1];
        } else {
            const normalized = this.normalizeValue(input);
            target = list.find((item) => this.normalizeValue(item.value) === normalized);
        }

        if (!target) {
            return context.send('Неверный номер подписки.', { keyboard: keyboard.SubscriptionsMenu });
        }
        const weekIndex = WeekIndex.getRelevant();
        const weekRange = weekIndex.getWeekDayIndexRange();

        const mode = await this.pickTestMode(context, keyboard);
        if (!mode) {
            return;
        }

        if (target.type === 'group') {
            const days = await this.app.getService('timetable').getGroupDaysByRange(weekRange, target.value);
            if (mode === 'day' || mode === 'both') {
                const day = this.pickGroupDay(days);
                const message = [
                    `📢 Группа ${target.value}: расписание на ${this.getDayPhrase(day?.day)}`,
                    formatter.formatGroupFull(target.value, { showHeader: false, days: day ? [day] : [] })
                ].join('\n');

                await context.send(message, { keyboard: keyboard.SubscriptionsMenu });
                if (mode === 'day') {
                    return;
                }
            }

            return context.send(
                `🆕 Группа ${target.value}: доступно расписание на следующую неделю`,
                {
                    keyboard: StaticKeyboard.GetWeekTimetable({
                        type: 'group',
                        value: target.value,
                        showHeader: false,
                        label: '📃 Показать',
                        weekIndex: weekIndex.valueOf()
                    })
                }
            );
        }

        const days = await this.app.getService('timetable').getTeacherDaysByRange(weekRange, target.value);
        if (mode === 'day' || mode === 'both') {
            const day = this.pickTeacherDay(days);
            const message = [
                `📢 Преподаватель ${target.value}: расписание на ${this.getDayPhrase(day?.day)}`,
                formatter.formatTeacherFull(target.value, { showHeader: false, days: day ? [day] : [] })
            ].join('\n');

            await context.send(message, { keyboard: keyboard.SubscriptionsMenu });
            if (mode === 'day') {
                return;
            }
        }

        return context.send(
            `🆕 Преподаватель ${target.value}: доступно расписание на следующую неделю`,
            {
                keyboard: StaticKeyboard.GetWeekTimetable({
                    type: 'teacher',
                    value: target.value,
                    showHeader: false,
                    label: '📃 Показать',
                    weekIndex: weekIndex.valueOf()
                })
            }
        );
    }

    private async getSubscriptions(chatId: number): Promise<SubscriptionEntry[]> {
        return Subscription.findAll({
            where: {
                chatId: chatId
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

    private pickGroupDay(days: GroupDay[]): GroupDay | undefined {
        const future = getFutureDays(days);
        if (future.length > 0) {
            return future[0];
        }
        return days[0];
    }

    private pickTeacherDay(days: TeacherDay[]): TeacherDay | undefined {
        const future = getFutureDays(days);
        if (future.length > 0) {
            return future[0];
        }
        return days[0];
    }

    private getDayPhrase(day?: string): string {
        if (!day) {
            return 'день';
        }

        const dayIndex = DayIndex.fromStringDate(day);
        if (dayIndex.isToday()) {
            return 'сегодня';
        }

        if (dayIndex.isTomorrow()) {
            return 'завтра';
        }

        if (WeekIndex.fromStringDate(day).isFutureWeek()) {
            return 'следующую неделю';
        }

        return 'день';
    }

    private normalizeValue(value: string): string {
        return value.replaceAll('.', '').replaceAll(' ', '').toLowerCase();
    }

    private async pickTestMode(context: CmdHandlerParams['context'], keyboard: CmdHandlerParams['keyboard']) {
        const prompt = [
            'Что проверить?',
            '1. Оповещение об изменении дня',
            '2. Оповещение о новой неделе',
            '3. Оба варианта'
        ].join('\n');

        const selected = await context.input(prompt, { keyboard: keyboard.SubscriptionsMenu });
        const input = selected?.text?.trim();
        if (!input) {
            return null;
        }

        if (input === '1') {
            return 'day';
        }

        if (input === '2') {
            return 'week';
        }

        if (input === '3') {
            return 'both';
        }

        return null;
    }
}
