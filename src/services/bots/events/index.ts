import { InferAttributes, ModelStatic, Op, WhereOptions } from "sequelize";
import { config } from "../../../../config";
import { App } from "../../../app";
import { createScheduleFormatter } from "../../../formatter";
import { DayIndex, StringDate, WeekIndex, getFutureDays, prepareError } from "../../../utils";
import { CallsUpdateEvent, GroupDayEvent, TeacherDayEvent } from "../../parser";
import { raspCache, saveCache } from "../../parser/raspCache";
import { GroupDay, TeacherDay } from "../../parser/types";
import { DayCall } from "../../../../config.scheme";
import { MessageOptions } from "../abstract";
import { BotServiceName } from "../abstract/command";
import { AbstractServiceChat, BotChat, ChatMode } from "../chat";
import { StaticKeyboard } from "../keyboard";
import { Subscription, SubscriptionType } from "../subscriptions/model";

function getDayPhrase(day: string, nextDayPhrase: string = 'день'): string {
    if (WeekIndex.fromStringDate(day).isFutureWeek()) {
        return 'следующую неделю';
    }

    const dayIndex = DayIndex.fromStringDate(day);

    if (dayIndex.isToday()) {
        return 'сегодня';
    }

    if (dayIndex.isTomorrow()) {
        return 'завтра';
    }

    return nextDayPhrase;
}

export type ProgressCallback = (data: {
    position: number,
    count: number
}) => void

export type CronDay = {
    index: number,
    latest?: boolean
}

export abstract class AbstractBotEventListener {
    protected abstract _model: ModelStatic<AbstractServiceChat>;
    public readonly abstract service: BotServiceName;

    constructor(protected app: App) { }

    // protected abstract createChat(chat: DbChat): T;
    public abstract sendMessage(chat: BotChat, message: string, options?: MessageOptions): Promise<any>;

    protected getBotEventControlller() {
        return this.app.getService('bot').events;
    }

    protected async sendMessages(chats: BotChat | BotChat[], message: string, options?: MessageOptions, cb?: ProgressCallback): Promise<void> {
        if (!Array.isArray(chats)) {
            chats = [chats];
        }

        if (cb) {
            cb({ position: 0, count: chats.length })
        }

        for (const i in chats) {
            const chat = chats[i];

            await this.sendMessage(chat, message, options);

            if (cb) {
                cb({ position: +i + 1, count: chats.length })
            }
        }
    }

    protected async getChats(where?: WhereOptions<InferAttributes<BotChat>>): Promise<BotChat[]> {
        return BotChat.findAll({
            where: Object.assign({
                accepted: true,
                allowSendMess: true,
                service: this.service,
                ...(config.dev ? {
                    noticeParserErrors: true
                } : {})
            }, where),
            include: {
                association: BotChat.associations[this._model.name],
                required: true
            },
        }).then(chats => {
            return chats.map(chat => {
                chat.serviceChat = (chat as any)[this._model.name];

                return chat;
            });
        });
    }

    protected getAdminPeerIds(): Array<string | number> {
        return [];
    }

    protected async getAdminChats(): Promise<BotChat[]> {
        const adminIds = this.getAdminPeerIds();
        if (!adminIds.length) {
            return [];
        }

        return BotChat.findAll({
            where: {
                accepted: true,
                allowSendMess: true,
                service: this.service,
                [`$${this._model.name}.peerId$`]: {
                    [Op.in]: adminIds
                }
            } as any,
            include: {
                association: BotChat.associations[this._model.name],
                required: true
            },
        }).then(chats => {
            return chats.map(chat => {
                chat.serviceChat = (chat as any)[this._model.name];

                return chat;
            });
        });
    }

    protected async getSubscribedChats(type: SubscriptionType, value: string, where?: WhereOptions<InferAttributes<BotChat>>): Promise<BotChat[]> {
        const subscriptions = await Subscription.findAll({
            attributes: ['chatId'],
            where: {
                type,
                value
            }
        });

        const chatIds = subscriptions.map((sub) => sub.chatId);
        if (chatIds.length === 0) {
            return [];
        }

        return this.getChats(Object.assign({
            id: chatIds
        }, where));
    }

    protected mergeChats(base: BotChat[], extra: BotChat[]): BotChat[] {
        if (extra.length === 0) {
            return base;
        }

        const known = new Set(base.map((chat) => chat.id));
        for (const chat of extra) {
            if (!known.has(chat.id)) {
                base.push(chat);
            }
        }

        return base;
    }

    protected async getGroupsChats<T>(group: string | string[], where?: WhereOptions<InferAttributes<BotChat>>): Promise<BotChat[]> {
        return this.getChats(Object.assign({
            group: group,
            [Op.or]: {
                deactivateSecondaryCheck: true,
                mode: ['student', 'parent']
            },
        }, where));
    }

    protected getTeachersChats<T>(teacher: string | string[], where?: WhereOptions<InferAttributes<BotChat>>): Promise<BotChat[]> {
        return this.getChats(Object.assign({
            teacher: teacher,
            [Op.or]: {
                deactivateSecondaryCheck: true,
                mode: 'teacher'
            },
        }, where));
    }

    public async cronGroupDay({ index, latest }: CronDay) {
        const groups: string[] = Object.entries(raspCache.groups.timetable)
            .map(([group, { days }]): [string, GroupDay | undefined] => {
                const todayDay = days.find((day) => {
                    return DayIndex.fromStringDate(day.day).isToday();
                });

                return [group, todayDay];
            }).filter(([, day]): boolean => {
                if (!day) return false;

                return (latest ? (day.lessons.length >= index + 1) : (day.lessons.length === index + 1)) ||
                    (day.lessons.length === 0 && index + 1 === config.parser.lessonIndexIfEmpty);
            }).map(([group]): string => {
                return group;
            });

        const chats: BotChat[] = await this.getGroupsChats(groups, { noticeChanges: true });
        if (chats.length === 0) return;

        const chatsKeyed: { [group: string]: BotChat[] } = chats.reduce<{ [group: string]: BotChat[] }>((obj, chat: BotChat) => {
            const group: string = String(chat.group!);

            if (!obj[group]) {
                obj[group] = [];
            }

            obj[group].push(chat);

            return obj;
        }, {});

        for (const group in chatsKeyed) {
            const groupEntry = raspCache.groups.timetable[group];
            const chats: BotChat[] = chatsKeyed[group];

            const nextDays = getFutureDays(groupEntry.days);
            if (!nextDays.length) continue;

            //если дальше всё расписание пустое, то больше не оповещаем
            const isEmpty: boolean = nextDays.every(day => day.lessons.length === 0);
            if (isEmpty) continue;

            const day = nextDays[0];

            const dayIndex = DayIndex.fromStringDate(day.day).valueOf();
            if (groupEntry.lastNoticedDay && dayIndex <= groupEntry.lastNoticedDay) {
                continue;
            }

            this.getBotEventControlller().deferFunction(`updateLastGroupNoticedDay_${group}`, async () => {
                groupEntry.lastNoticedDay = dayIndex;
                await saveCache();
            })

            const phrase: string = getDayPhrase(day.day, 'следующий день');

            for (const chat of chats) {
                const formatter = createScheduleFormatter(this.service, this.app, raspCache, chat);

                const message: string = [
                    `📢 Группа ${group}: расписание на ${phrase}\n`,
                    formatter.formatGroupFull(group, {
                        showHeader: false,
                        days: [day]
                    })
                ].join('\n');

                await this.sendMessage(chat, message);
            }
        }
    }

    public async addGroupDay({ day, group }: GroupDayEvent) {
        const baseChats: BotChat[] = await this.getGroupsChats(group, { noticeChanges: true });
        const subscriptionChats = await this.getSubscribedChats('group', group, { noticeChanges: true });
        const chats = this.mergeChats(baseChats, subscriptionChats);
        if (chats.length === 0) return;

        const phrase: string = getDayPhrase(day.day, 'следующий день');

        for (const chat of chats) {
            const formatter = createScheduleFormatter(this.service, this.app, raspCache, chat);

            const message: string = [
                `📢 Группа ${group}: расписание на ${phrase}\n`,
                formatter.formatGroupFull(group, {
                    showHeader: false,
                    days: [day]
                })
            ].join('\n');

            await this.sendMessage(chat, message);
        }
    }

    public async updateGroupDay({ day, group }: GroupDayEvent) {
        const baseChats: BotChat[] = await this.getGroupsChats(group, { noticeChanges: true });
        const subscriptionChats = await this.getSubscribedChats('group', group, { noticeChanges: true });
        const chats = this.mergeChats(baseChats, subscriptionChats);
        if (chats.length === 0) return;

        const phrase: string = getDayPhrase(day.day);

        for (const chat of chats) {
            const formatter = createScheduleFormatter(this.service, this.app, raspCache, chat);

            const message: string = [
                `🆕 Группа ${group}: изменено расписание ${phrase}\n`,
                formatter.formatGroupFull(group, {
                    showHeader: false,
                    days: [day]
                })
            ].join('\n');

            await this.sendMessage(chat, message);
        }
    }

    public async cronTeacherDay({ index, latest }: CronDay) {
        const teachers: string[] = Object.entries(raspCache.teachers.timetable)
            .map(([teacher, { days }]): [string, TeacherDay | undefined] => {
                const todayDay = days.find((day) => {
                    return DayIndex.fromStringDate(day.day).isToday();
                });

                return [teacher, todayDay];
            }).filter(([, day]): boolean => {
                if (!day) return false;

                return (latest ? (day.lessons.length >= index + 1) : (day.lessons.length === index + 1)) ||
                    (day.lessons.length === 0 && index + 1 === config.parser.lessonIndexIfEmpty);
            }).map(([teacher]): string => {
                return teacher;
            });

        const chats: BotChat[] = await this.getTeachersChats(teachers, { noticeChanges: true });
        if (chats.length === 0) return;

        const chatsKeyed: { [teacher: string]: BotChat[] } = chats.reduce<{ [teacher: string]: BotChat[] }>((obj, chat: BotChat) => {
            const teacher: string = String(chat.teacher!);

            if (!obj[teacher]) {
                obj[teacher] = [];
            }

            obj[teacher].push(chat);

            return obj;
        }, {});

        for (const teacher in chatsKeyed) {
            const teacherEntry = raspCache.teachers.timetable[teacher];
            const chats: BotChat[] = chatsKeyed[teacher];

            const nextDays = getFutureDays(teacherEntry.days);
            if (!nextDays.length) continue;

            //если дальше всё расписание пустое, то больше не оповещаем
            const isEmpty: boolean = nextDays.every(day => day.lessons.length === 0);
            if (isEmpty) continue;

            const day = nextDays[0];

            const dayIndex = DayIndex.fromStringDate(day.day).valueOf();
            if (teacherEntry.lastNoticedDay && dayIndex <= teacherEntry.lastNoticedDay) {
                continue;
            }

            this.getBotEventControlller().deferFunction(`updateLastTeacherNoticedDay_${teacher}`, async () => {
                teacherEntry.lastNoticedDay = dayIndex;
                await saveCache();
            })

            const phrase: string = getDayPhrase(day.day, 'следующий день');

            for (const chat of chats) {
                const formatter = createScheduleFormatter(this.service, this.app, raspCache, chat);

                const message: string = [
                    `📢 Преподаватель ${teacher}: расписание на ${phrase}\n`,
                    formatter.formatTeacherFull(teacher, {
                        showHeader: false,
                        days: [day]
                    })
                ].join('\n');

                await this.sendMessage(chat, message);
            }
        }
    }

    public async addTeacherDay({ day, teacher }: TeacherDayEvent) {
        const baseChats: BotChat[] = await this.getTeachersChats(teacher, { noticeChanges: true });
        const subscriptionChats = await this.getSubscribedChats('teacher', teacher, { noticeChanges: true });
        const chats = this.mergeChats(baseChats, subscriptionChats);
        if (chats.length === 0) return;

        const phrase: string = getDayPhrase(day.day, 'следующий день');

        for (const chat of chats) {
            const formatter = createScheduleFormatter(this.service, this.app, raspCache, chat);

            const message: string = [
                `📢 Преподаватель ${teacher}: расписание на ${phrase}\n`,
                formatter.formatTeacherFull(teacher, {
                    showHeader: false,
                    days: [day]
                })
            ].join('\n');

            await this.sendMessage(chat, message);
        }
    }

    public async updateTeacherDay({ day, teacher }: TeacherDayEvent) {
        const baseChats: BotChat[] = await this.getTeachersChats(teacher, { noticeChanges: true });
        const subscriptionChats = await this.getSubscribedChats('teacher', teacher, { noticeChanges: true });
        const chats = this.mergeChats(baseChats, subscriptionChats);
        if (chats.length === 0) return;

        const phrase: string = getDayPhrase(day.day);

        for (const chat of chats) {
            const formatter = createScheduleFormatter(this.service, this.app, raspCache, chat);

            const message: string = [
                `🆕 Преподаватель ${teacher}: изменено расписание ${phrase}\n`,
                formatter.formatTeacherFull(teacher, {
                    showHeader: false,
                    days: [day]
                })
            ].join('\n');

            await this.sendMessage(chat, message);
        }
    }

    public async updateWeek(chatMode: ChatMode, weekIndex: number) {
        const firstWeekDay = WeekIndex.fromWeekIndexNumber(weekIndex).getFirstDayDate();

        let chats: BotChat[] | undefined;

        const message: string = '🆕 Доступно расписание на следующую неделю';

        switch (chatMode) {
            case 'student': {
                const groups: string[] = Object.entries(raspCache.groups.timetable).map(([group, { days }]): [string, GroupDay[]] => {
                    const daysOfWeek = days.filter((day) => {
                        return StringDate.fromStringDate(day.day).toDate() >= firstWeekDay && day.lessons.length > 0;
                    });

                    return [group, daysOfWeek];
                }).filter(([, days]): boolean => {
                    return days.length > 0;
                }).map(([group]): string => {
                    return group;
                });

                const baseChats = await this.getGroupsChats(groups, { noticeNextWeek: true });
                const baseIds = new Set(baseChats.map((chat) => chat.id));

                for (const chat of baseChats) {
                    await this.sendMessage(chat, message, {
                        keyboard: chat.group ? StaticKeyboard.GetWeekTimetable({
                            type: 'group',
                            value: chat.group,
                            showHeader: false,
                            label: '📃 Показать',
                            weekIndex
                        }) : undefined
                    });
                }

                for (const group of groups) {
                    const subscriptionChats = await this.getSubscribedChats('group', group, { noticeNextWeek: true });
                    for (const chat of subscriptionChats) {
                        if (baseIds.has(chat.id)) {
                            continue;
                        }

                        const scopedMessage = `🆕 Группа ${group}: доступно расписание на следующую неделю`;
                        await this.sendMessage(chat, scopedMessage, {
                            keyboard: StaticKeyboard.GetWeekTimetable({
                                type: 'group',
                                value: group,
                                showHeader: false,
                                label: '📃 Показать',
                                weekIndex
                            })
                        });
                    }
                }

                return;
            }

            case 'teacher': {
                const teachers: string[] = Object.entries(raspCache.teachers.timetable).map(([group, { days }]): [string, TeacherDay[]] => {
                    const daysOfWeek = days.filter((day) => {
                        return StringDate.fromStringDate(day.day).toDate() >= firstWeekDay && day.lessons.length > 0;
                    });

                    return [group, daysOfWeek];
                }).filter(([, days]): boolean => {
                    return days.length > 0;
                }).map(([teacher]): string => {
                    return teacher;
                });

                const baseChats = await this.getTeachersChats(teachers, { noticeNextWeek: true });
                const baseIds = new Set(baseChats.map((chat) => chat.id));

                for (const chat of baseChats) {
                    await this.sendMessage(chat, message, {
                        keyboard: chat.teacher ? StaticKeyboard.GetWeekTimetable({
                            type: 'teacher',
                            value: chat.teacher,
                            showHeader: false,
                            label: '📃 Показать',
                            weekIndex
                        }) : undefined
                    });
                }

                for (const teacher of teachers) {
                    const subscriptionChats = await this.getSubscribedChats('teacher', teacher, { noticeNextWeek: true });
                    for (const chat of subscriptionChats) {
                        if (baseIds.has(chat.id)) {
                            continue;
                        }

                        const scopedMessage = `🆕 Преподаватель ${teacher}: доступно расписание на следующую неделю`;
                        await this.sendMessage(chat, scopedMessage, {
                            keyboard: StaticKeyboard.GetWeekTimetable({
                                type: 'teacher',
                                value: teacher,
                                showHeader: false,
                                label: '📃 Показать',
                                weekIndex
                            })
                        });
                    }
                }

                return;
            }
        }
    }

    public async sendDistribution(message: string, cb?: ProgressCallback) {
        const chats: BotChat[] = await this.getChats({
            subscribeDistribution: true
        });

        return this.sendMessages(chats, message, undefined, cb);
    }

    public async sendError(error: Error) {
        const baseChats: BotChat[] = await this.getChats({
            noticeParserErrors: true
        });

        const adminChats = await this.getAdminChats();
        const chats = this.mergeChats(baseChats, adminChats);
        if (chats.length === 0) {
            return;
        }

        return this.sendMessages(chats, [
            'Parser error\n',
            prepareError(error)
        ].join('\n'));
    }

    public async updateCalls(data: CallsUpdateEvent) {
        if (!data.weekdaysChanged && !data.saturdayChanged) {
            return;
        }

        const chats: BotChat[] = await this.getChats({
            noticeCalls: true,
            [Op.or]: [
                { group: { [Op.ne]: null } },
                { teacher: { [Op.ne]: null } }
            ]
        });
        if (chats.length === 0) return;

        const parts: string[] = [];
        parts.push('\uD83D\uDD14 \u0418\u0437\u043C\u0435\u043D\u0435\u043D\u043E \u0440\u0430\u0441\u043F\u0438\u0441\u0430\u043D\u0438\u0435 \u0437\u0432\u043E\u043D\u043A\u043E\u0432');
        if (data.reason) {
            parts.push(`\u041F\u0440\u0438\u0447\u0438\u043D\u0430: ${data.reason}`);
        }

        if (data.weekdaysChanged) {
            parts.push('\n__ \u0417\u0432\u043E\u043D\u043A\u0438 (\u0431\u0443\u0434\u043D\u0438) __');
            parts.push(this.formatCalls(data.schedule.weekdays));
        }

        if (data.saturdayChanged) {
            parts.push('\n__ \u0417\u0432\u043E\u043D\u043A\u0438 (\u0441\u0443\u0431\u0431\u043E\u0442\u0430) __');
            parts.push(this.formatCalls(data.schedule.saturday));
        }

        parts.push('\n\u041F\u043E\u043B\u043D\u043E\u0435 \u0440\u0430\u0441\u043F\u0438\u0441\u0430\u043D\u0438\u0435: \u043D\u0430\u0436\u043C\u0438\u0442\u0435 \u041F\u043E\u043A\u0430\u0437\u0430\u0442\u044C');
        const message = parts.join('\n');
        await this.sendMessages(chats, message, { keyboard: StaticKeyboard.GetCalls() });
    }

    private formatCalls(calls: DayCall[]): string {
        const text: string[] = [];
        for (let i = 0; i < calls.length; i++) {
            const lesson = calls[i];
            if (!lesson) break;
            const lineStr: string = `${i + 1}. ${lesson[0][0]} - ${lesson[0][1]} | ${lesson[1][0]} - ${lesson[1][1]}`;
            text.push(lineStr);
        }
        return text.join('\n');
    }
}
