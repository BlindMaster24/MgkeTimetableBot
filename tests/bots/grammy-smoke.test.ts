import { Bot } from 'grammy';
import type { Update } from 'grammy/types';
import { describe, expect, it } from 'vitest';

type ApiCall = { method: string; payload: Record<string, unknown> };

const BOT_INFO = {
    id: 1234567890,
    is_bot: true as const,
    first_name: 'MgkeTest',
    username: 'mgke_test_bot',
    can_join_groups: true,
    can_read_all_group_messages: false,
    supports_inline_queries: false,
    can_connect_to_business: false,
    has_main_web_app: false
};

const makeBot = () => {
    const calls: ApiCall[] = [];
    const bot = new Bot('1234567890:TEST', { botInfo: BOT_INFO as any });

    bot.api.config.use(((_prev: unknown, method: string, payload: Record<string, unknown>) => {
        calls.push({ method, payload });
        const text = typeof payload.text === 'string' ? payload.text : '';
        return Promise.resolve({
            ok: true,
            result: {
                message_id: calls.length,
                date: Math.floor(Date.now() / 1000),
                chat: { id: 42, type: 'private', first_name: 'Tester' },
                from: { id: BOT_INFO.id, is_bot: true, first_name: BOT_INFO.first_name },
                text
            }
        });
    }) as any);

    return { bot, calls };
};

const textUpdate = (text: string): Update => {
    const isCommand = /^\/[A-Za-z_]+/.test(text);
    const entities = isCommand
        ? [{ type: 'bot_command' as const, offset: 0, length: text.split(/\s/)[0].length }]
        : undefined;
    return {
        update_id: Math.floor(Math.random() * 1e9),
        message: {
            message_id: 10,
            date: Math.floor(Date.now() / 1000),
            chat: { id: 42, type: 'private', first_name: 'Tester' },
            from: { id: 42, is_bot: false, first_name: 'Tester' },
            text,
            ...(entities ? { entities } : {})
        }
    };
};

describe('grammY integration smoke', () => {
    it('routes a /start update to its hears handler and sends a reply through the mocked transport', async () => {
        const { bot, calls } = makeBot();

        bot.hears(/^\/start$/, (ctx) => ctx.reply('pong'));
        await bot.init();
        await bot.handleUpdate(textUpdate('/start'));

        expect(calls).toHaveLength(1);
        expect(calls[0].method).toBe('sendMessage');
        expect(calls[0].payload.chat_id).toBe(42);
        expect(calls[0].payload.text).toBe('pong');
    });

    it('routes a non-matching text update to nothing', async () => {
        const { bot, calls } = makeBot();

        bot.hears(/^\/start$/, (ctx) => ctx.reply('pong'));
        await bot.init();
        await bot.handleUpdate(textUpdate('hello'));

        expect(calls).toHaveLength(0);
    });

    it('surfaces middleware errors as BotError with the original cause attached', async () => {
        const { bot, calls } = makeBot();

        bot.hears(/^\/boom$/, () => {
            throw new Error('synthetic');
        });
        await bot.init();

        await expect(bot.handleUpdate(textUpdate('/boom'))).rejects.toMatchObject({
            error: expect.objectContaining({ message: 'synthetic' })
        });
        expect(calls).toHaveLength(0);
    });

    it('supports command handlers for bot commands', async () => {
        const { bot, calls } = makeBot();

        bot.command('ics', (ctx) => ctx.reply('ics-reply'));
        await bot.init();
        await bot.handleUpdate(textUpdate('/ics'));

        expect(calls).toHaveLength(1);
        expect(calls[0].payload.text).toBe('ics-reply');
    });
});
