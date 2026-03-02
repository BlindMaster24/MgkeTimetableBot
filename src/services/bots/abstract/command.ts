import type { TelegramBotCommand } from '../types/telegram';
import { ContextDefaultState, MessageContext as VkMessageContext } from 'vk-io';
import { App, AppServiceName } from '../../../app';
import { ScheduleFormatter } from '../../../formatter';
import { raspCache } from '../../parser';
import { AbstractServiceChat, BotChat } from '../chat';
import { Keyboard, StaticKeyboard, withCancelButton } from '../keyboard';
import { Storage } from '../storage';
import { TgChat } from '../tg/chat';
import { TgCommandContext, TgMessageRealContext } from '../tg/context';
import { ViberChat } from '../viber/chat';
import { ViberCommandContext, ViberContext } from '../viber/context';
import { VkChat } from '../vk/chat';
import { VkCommandContext } from '../vk/context';
import { AbstractAction } from './action';
import { AbstractCommandContext } from './context';
import { KeyboardBuilder } from './keyboardBuilder';

export type BotServiceName = 'tg' | 'vk' | 'viber';

export type CmdHandlerParams<C extends AbstractCommand = any> = {
    context: AbstractCommandContext,
    realContext: VkMessageContext<ContextDefaultState> | ViberContext | TgMessageRealContext,
    serviceChat: AbstractServiceChat,
    chat: BotChat,
    regexp?: C['regexp'] extends RegExp ? 'index' : keyof C['regexp'],
    actions: AbstractAction,
    keyboard: Keyboard,
    service: BotServiceName,
    formatter: ScheduleFormatter,
    cache: Storage
} & ({
    service: 'vk',
    context: VkCommandContext,
    realContext: VkMessageContext<ContextDefaultState>,
    serviceChat: VkChat
} | {
    service: 'viber',
    context: ViberCommandContext,
    realContext: ViberContext,
    serviceChat: ViberChat
} | {
    service: 'tg',
    context: TgCommandContext,
    realContext: TgMessageRealContext,
    serviceChat: TgChat
})

export abstract class AbstractCommand {
    /**
    * Ð£Ð½Ð¸ÐºÐ°Ð»ÑŒÐ½Ñ‹Ð¹ Ð¸Ð´ÐµÐ½Ñ‚Ð¸Ñ„Ð¸ÐºÐ°Ñ‚Ð¾Ñ€ ÐºÐ¾Ð¼Ð°Ð½Ð´Ñ‹, ÑƒÑÑ‚Ð°Ð½Ð°Ð²Ð»Ð¸Ð²Ð°ÐµÑ‚ÑÑ Ð²Ð¾ Ð²Ñ€ÐµÐ¼Ñ Ð·Ð°Ð³Ñ€ÑƒÐ·ÐºÐ¸ ÐºÐ¾Ð¼Ð°Ð½Ð´
    **/
    public id?: string;

    /**
    * Ð”Ð¾Ð»Ð¶ÐµÐ½ Ð»Ð¸ Ð±Ñ‹Ñ‚ÑŒ Ñ‡Ð°Ñ‚ Ð¿Ð¾Ð´Ð²ÐµÑ€Ð¶Ð´Ñ‘Ð½Ð½Ñ‹Ð¼, Ñ‡Ñ‚Ð¾Ð±Ñ‹ Ð¸ÑÐ¿Ð¾Ð»ÑŒÐ·Ð¾Ð²Ð°Ñ‚ÑŒ ÑÑ‚Ñƒ ÐºÐ¾Ð¼Ð°Ð½Ð´Ñƒ
    **/
    public acceptRequired: boolean = true;

    /**
    * Ð”Ð¾ÑÑ‚ÑƒÐ¿Ð½Ð° Ð»Ð¸ ÑÑ‚Ð° ÐºÐ¾Ð¼Ð°Ð½Ð´Ð° Ñ‚Ð¾Ð»ÑŒÐºÐ¾ Ð´Ð»Ñ Ð°Ð´Ð¼Ð¸Ð½Ð¾Ð²?
    **/
    public adminOnly: boolean = false;

    /**
    * Ð‘Ð°Ð·Ð¾Ð²Ð°Ñ ÐºÐ¾Ð¼Ð°Ð½Ð´Ð° Ð¸ ÐµÑ‘ Ð¾Ð¿Ð¸ÑÐ°Ð½Ð¸Ñ Ð´Ð»Ñ Ñ€ÐµÐ³Ð¸ÑÑ‚Ñ€Ð°Ñ†Ð¸Ð¸ ÐµÑ‘ Ð² ÑÐ¿Ð¸ÑÐºÐµ ÐºÐ¾Ð¼Ð°Ð½Ð´ Ð² Ð¿Ð¾Ð¼Ð¾Ñ‰Ð¸ (Ð¸ Ð´Ð»Ñ ÑÐ¿Ð¸ÑÐºÐ° Ñ‚ÐµÐ»ÐµÐ³Ð¸)
    **/
    public tgCommand: TelegramBotCommand | TelegramBotCommand[] | null = null;

    /**
    * Ð¡Ð¿Ð¸ÑÐ¾Ðº ÑÐµÑ€Ð²Ð¸ÑÐ¾Ð² Ð±Ð¾Ñ‚Ð¾Ð², Ð² ÐºÐ¾Ñ‚Ð¾Ñ€Ñ‹Ñ… ÐºÐ¾Ð¼Ð°Ð½Ð´Ð° Ð±ÑƒÐ´ÐµÑ‚ Ñ€Ð°Ð±Ð¾Ñ‚Ð°Ñ‚ÑŒ
    * (ÐµÑÐ»Ð¸ undefined, Ð²Ð¾ Ð²ÑÐµÑ… ÑÐµÑ€Ð¸ÑÐ°Ñ…)
    **/
    public services?: BotServiceName[];

    /**
    * Ð¡Ð¿Ð¸ÑÐ¾Ðº ÑÐµÑ€Ð²Ð¸ÑÐ¾Ð², Ð½ÐµÐ¾Ð±Ñ…Ð¾Ð´Ð¸Ð¼Ñ‹Ðµ Ð´Ð»Ñ Ñ€Ð°Ð±Ð¾Ñ‚Ñ‹ ÐºÐ¾Ð¼Ð°Ð½Ð´Ñ‹
    * (ÐµÑÐ»Ð¸ undefined, ÐºÐ¾Ð¼Ð°Ð½Ð´Ð° Ñ€ÐµÐ³Ð¸ÑÑ‚Ñ€Ð¸Ñ€ÑƒÐµÑ‚ÑÑ Ð²ÑÐµÐ³Ð´Ð°, ÐµÑÐ»Ð¸ Ð¶Ðµ ÑƒÐºÐ°Ð·Ð°Ð½Ð½Ñ‹Ð¹ ÑÐµÑ€Ð²Ð¸Ñ Ð½Ðµ Ð·Ð°Ð³Ñ€ÑƒÐ¶ÐµÐ½, Ñ‚Ð¾ Ð¸ ÐºÐ¾Ð¼Ð°Ð½Ð´Ð° Ð½Ðµ Ð±ÑƒÐ´ÐµÑ‚ Ð·Ð°Ð³Ñ€ÑƒÐ¶ÐµÐ½Ð°)
    **/
    public requireServices?: AppServiceName[];

    /**
    * Ð ÐµÐ³ÑƒÐ»ÑÑ€Ð½Ð¾Ðµ Ð²Ñ‹Ñ€Ð°Ð¶ÐµÐ½Ð¸Ðµ Ð´Ð»Ñ ÐºÐ¾Ð¼Ð°Ð½Ð´Ñ‹, Ð¿Ð¾ ÐºÐ¾Ñ‚Ñ€Ð¾Ð¼Ñƒ Ð¾Ð½Ð° Ð±ÑƒÐ´ÐµÑ‚ Ð²Ñ‹Ð·Ñ‹Ð²Ð°Ñ‚ÑŒÑÑ
    **/
    public abstract regexp: { [regexp: string]: RegExp } | RegExp | null;

    /**
     * Ð•ÑÐ»Ð¸ ÑƒÐºÐ°Ð·Ð°Ð½, Ñ‚Ð¾ ÐºÐ¾Ð¼Ð°Ð½Ð´Ð° Ð±ÑƒÐ´ÐµÑ‚ Ð²Ñ‹Ð·Ñ‹Ð²Ð°Ñ‚ÑŒÑÑ Ð¿Ñ€Ð¸ ÑƒÐºÐ°Ð·Ð°Ð½Ð½Ð¾Ð¼ Ð´ÐµÐ¹ÑÑ‚Ð²Ð¸Ð¸
     * ÐµÑÐ»Ð¸ ÑÐ¾Ð²Ð¿Ð°Ð´Ð°ÑŽÑ‚ payload.action
     */
    public abstract payloadAction: string | null;

    /**
     * Ð¡Ñ†ÐµÐ½Ð°, Ð² ÐºÐ¾Ñ‚Ð¾Ñ€Ð¾Ð¹ Ð±ÑƒÐ´ÐµÑ‚ Ñ€Ð°Ð±Ð¾Ñ‚Ð°Ñ‚ÑŒ ÐºÐ¾Ð¼Ð°Ð½Ð´Ð°.
     * (Ð½Ðµ Ñ€Ð°Ð±Ð¾Ñ‚Ð°ÐµÑ‚ Ð´Ð»Ñ payload)
     * 
     * null - Ñ€Ð°Ð±Ð¾Ñ‚Ð° Ñ‚Ð¾Ð»ÑŒÐºÐ¾ Ð² Ð³Ð»Ð°Ð²Ð½Ð¾Ð¹ ÑÑ†ÐµÐ½Ðµ
     * string - Ñ€Ð°Ð±Ð¾Ñ‚Ð° Ð² ÑƒÐºÐ°Ð·Ð°Ð½Ð½Ð¾Ð¹ ÑÑ†ÐµÐ½Ðµ
     * undefined - Ñ€Ð°Ð±Ð¾Ñ‚Ð° Ð² Ð»ÑŽÐ±Ð¾Ð¹ ÑÑ†ÐµÐ½Ðµ
     */
    public scene?: string | null;

    public abstract handler(params: CmdHandlerParams): any | Promise<any>

    constructor(protected app: App) { }

    public preHandle({ service, serviceChat: chat }: CmdHandlerParams) {
        if (this.services && !this.services.includes(service)) {
            return false;
        }

        if (this.adminOnly && !chat.isSuperAdmin()) {
            return false;
        }

        return true;
    }

    protected async findGroup({ context }: CmdHandlerParams, group?: string, errorKeyboard: KeyboardBuilder = StaticKeyboard.Cancel): Promise<false | string> {
        const normalized = group?.replace(/\*+$/g, '') ?? '';

        if (!normalized || isNaN(+normalized)) {
            await context.send('Ð­Ñ‚Ð¾ Ð½Ðµ Ñ‡Ð¸ÑÐ»Ð¾', {
                keyboard: errorKeyboard
            });

            return false;
        }

        if (normalized.length > 3) {
            await context.send('ÐÐ¾Ð¼ÐµÑ€ Ð³Ñ€ÑƒÐ¿Ð¿Ñ‹ Ð²Ð²ÐµÐ´Ñ‘Ð½ Ð½ÐµÐ²ÐµÑ€Ð½Ð¾', {
                keyboard: errorKeyboard
            });

            return false;
        }

        if (!raspCache.groups.timetable[normalized]) {
            await context.send('Ð”Ð°Ð½Ð½Ð¾Ð¹ ÑƒÑ‡ÐµÐ±Ð½Ð¾Ð¹ Ð³Ñ€ÑƒÐ¿Ð¿Ñ‹ Ð½Ðµ ÑÑƒÑ‰ÐµÑÑ‚Ð²ÑƒÐµÑ‚', {
                keyboard: errorKeyboard
            })

            return false;
        }

        return normalized;
    }

    protected async findTeacher({ context, keyboard }: CmdHandlerParams, teacher?: string, errorKeyboard: KeyboardBuilder = StaticKeyboard.Cancel): Promise<false | undefined | string> {
        if (!teacher || teacher.length < 3) {
            await context.send('Ð¤Ð°Ð¼Ð¸Ð»Ð¸Ñ Ð²Ð²ÐµÐ´ÐµÐ½Ð° Ð½ÐµÐºÐ¾Ñ€Ñ€ÐµÐºÑ‚Ð½Ð¾', {
                keyboard: errorKeyboard
            });

            return false;
        }

        const matched: string[] = [];
        const matchLimit: number = 5;

        const shortTeachersList: string[] = Object.keys(raspCache.teachers.timetable);
        for (const sys_teacher of shortTeachersList) {
            const search = teacher.replaceAll('.', '').toLocaleLowerCase();
            const needle = sys_teacher.replaceAll('.', '').toLocaleLowerCase();

            if (needle.search(search) === -1) continue;

            if (needle.toLocaleLowerCase() === search) {
                matched.push(sys_teacher);
                break;
            }

            matched.push(sys_teacher);
            if (matched.length > matchLimit) break;
        }

        for (const sys_teacher in raspCache.team.names) {
            if (matched.length > matchLimit) break;
            if (matched.includes(sys_teacher)) continue;

            const fullTeacher = raspCache.team.names[sys_teacher];

            if (fullTeacher.toLocaleLowerCase().search(teacher.toLocaleLowerCase()) === -1) continue;

            matched.push(sys_teacher);
            if (matched.length > matchLimit) break;
        }

        if (matched.length === 0) {
            await context.send('Ð”Ð°Ð½Ð½Ñ‹Ð¹ Ð¿Ñ€ÐµÐ¿Ð¾Ð´Ð°Ð²Ð°Ñ‚ÐµÐ»ÑŒ Ð½Ðµ Ð½Ð°Ð¹Ð´ÐµÐ½', {
                keyboard: errorKeyboard
            });

            return false;
        }

        if (matched.length > matchLimit) {
            await context.send('Ð¡Ð»Ð¸ÑˆÐºÐ¾Ð¼ Ð¼Ð½Ð¾Ð³Ð¾ Ñ€ÐµÐ·ÑƒÐ»ÑŒÑ‚Ð°Ñ‚Ð¾Ð² Ð´Ð»Ñ Ð²Ñ‹Ð±Ð¾Ñ€ÐºÐ¸.', {
                keyboard: errorKeyboard
            })

            return false;
        }

        if (matched.length > 1) {
            await context.send(
                'ÐÐ°Ð¹Ð´ÐµÐ½Ð¾ Ð½ÐµÑÐºÐ¾Ð»ÑŒÐºÐ¾ Ð¿Ñ€ÐµÐ¿Ð¾Ð´Ð°Ð²Ð°Ñ‚ÐµÐ»ÐµÐ¹.\n' +
                'ÐšÐ°ÐºÐ¾Ð¹ Ð¸Ð¼ÐµÐ½Ð½Ð¾ Ð½ÑƒÐ¶ÐµÐ½?\n\n' +
                matched.join('\n'), {
                keyboard: withCancelButton(keyboard.generateVerticalKeyboard(matched))
            })

            return undefined;
        }

        return matched[0];
    }

    protected buildTeacherPrompt(keyboard: Keyboard, example?: string): string {
        const hasHistory = keyboard.TeacherHistory.buttons.length > 0;
        const base = hasHistory
            ? 'Ð’Ð²ÐµÐ´Ð¸Ñ‚Ðµ Ñ„Ð°Ð¼Ð¸Ð»Ð¸ÑŽ Ð¿Ñ€ÐµÐ¿Ð¾Ð´Ð°Ð²Ð°Ñ‚ÐµÐ»Ñ Ð¸Ð»Ð¸ Ð²Ñ‹Ð±ÐµÑ€Ð¸Ñ‚Ðµ Ð¸Ð· ÑÐ¿Ð¸ÑÐºÐ° Ð½Ð¸Ð¶Ðµ'
            : 'Ð’Ð²ÐµÐ´Ð¸Ñ‚Ðµ Ñ„Ð°Ð¼Ð¸Ð»Ð¸ÑŽ Ð¿Ñ€ÐµÐ¿Ð¾Ð´Ð°Ð²Ð°Ñ‚ÐµÐ»Ñ';
        if (example) {
            return `${base} (Ð½Ð°Ð¿Ñ€Ð¸Ð¼ÐµÑ€, ${example})`;
        }
        return base;
    }
}


