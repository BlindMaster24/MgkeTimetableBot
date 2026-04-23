import { Bot, GrammyError } from 'grammy';
import { CreationAttributes } from 'sequelize';
import StatusCode from 'status-code-enum';
import { config } from '../../../../config';
import { App, AppService } from '../../../app';
import { createScheduleFormatter } from '../../../formatter';
import { FromType, InputRequestKey } from '../../../key';
import { newTraceId, runWithLogContext } from '../../../logging';
import { raspCache } from '../../parser';
import { AbstractBot, AbstractCommand, AbstractCommandContext } from '../abstract';
import { BotChat } from '../chat';
import { AbstractBotEventListener } from '../events';
import { Keyboard } from '../keyboard';
import { TgBotAction } from './action';
import { TgChat } from './chat';
import { TgCallbackContext, TgCallbackRealContext, TgCommandContext, TgMessageRealContext } from './context';
import { TgEventListener } from './event';

type TgUser = {
    id: number,
    is_bot?: boolean
};

type TgChatInfo = {
    id: number,
    type: string,
    username?: string
};

type TgMyChatMemberContext = {
    update: {
        update_id: number,
        my_chat_member?: {
            chat: TgChatInfo,
            from?: TgUser,
            new_chat_member: {
                status: string
            }
        }
    },
    api: Bot['api']
};

function parseStartPayload(messageText?: string): string | undefined {
    if (!messageText) {
        return;
    }

    const [, payload] = messageText.match(/^\/start(?:@\w+)?(?:\s+([\s\S]+))?$/i) || [];

    return payload?.trim();
}

export class TgBot extends AbstractBot implements AppService {
    public tg: Bot;
    public event: AbstractBotEventListener;

    constructor(app: App) {
        super(app, 'tg');

        this.tg = new Bot(config.telegram.token);

        this.event = new TgEventListener(this.app, this.tg);
    }

    public async run() {
        this.tg.catch((err) => {
            const { ctx, error } = err;

            this.logger.error('tg_update_handler_error', {
                error,
                updateId: ctx.update.update_id,
                peerId: ctx.chat?.id,
                userId: ctx.from?.id,
                messageId: ctx.msg?.message_id
            });
        });

        this.tg.on('message', (context) => runWithLogContext({
            traceId: newTraceId(),
            service: 'tg',
            event: 'message',
            updateId: context.update.update_id,
            chatId: context.chat?.id,
            peerId: context.chat?.id,
            userId: context.from?.id,
            messageId: context.msg?.message_id
        }, () => this.messageHandler(context as TgMessageRealContext)));

        this.tg.on('my_chat_member', (context) => runWithLogContext({
            traceId: newTraceId(),
            service: 'tg',
            event: 'my_chat_member',
            updateId: context.update.update_id,
            chatId: context.chat?.id,
            peerId: context.chat?.id,
            userId: context.from?.id
        }, () => this.myChatMember(context as unknown as TgMyChatMemberContext)));

        this.tg.on('callback_query:data', (context) => runWithLogContext({
            traceId: newTraceId(),
            service: 'tg',
            event: 'callback_query',
            updateId: context.update.update_id,
            chatId: context.callbackQuery.message?.chat.id,
            peerId: context.callbackQuery.message?.chat.id,
            userId: context.from?.id,
            messageId: context.callbackQuery.message?.message_id
        }, () => this.callbackHandler(context as TgCallbackRealContext)));

        if (config.telegram.noticer) {
            this.getBotService().events.registerListener(this.event);
        }

        await this.getBotService().init();

        await this.setBotCommands().catch((e) => {
            this.logger.error('tg_set_commands_error', { error: e });
        });

        const stopPolling = () => this.tg.stop();
        process.once('SIGINT', stopPolling);
        process.once('SIGTERM', stopPolling);

        await this.tg.start({
            drop_pending_updates: false,
            onStart: () => {
                this.logger.info('tg_start_polling');
            }
        }).catch(err => {
            this.logger.error('tg_polling_error', { error: err });
        });
    }

    public async getChat(peerId: number, creationDefaults?: Partial<CreationAttributes<BotChat>>): Promise<BotChat<TgChat>> {
        return BotChat.findByServicePeerId(TgChat, peerId, creationDefaults);
    }

    private async setBotCommands() {
        const cmdPromises: Promise<boolean>[] = [];

        cmdPromises.push(this.tg.api.setMyCommands(
            this.getBotService().getBotCommands(),
            {
                scope: {
                    type: 'default'
                }
            }
        ));

        const adminCommands = this.getBotService().getBotCommands(true);
        for (const adminId of config.telegram.admin_ids) {
            cmdPromises.push(this.tg.api.setMyCommands(
                adminCommands,
                {
                    scope: {
                        type: 'chat',
                        chat_id: adminId
                    }
                }
            ));
        }

        const result = await Promise.all(cmdPromises);

        this.logger.info('tg_commands_set');

        return result;
    }

    private async messageHandler(context: TgMessageRealContext) {
        if (context.from?.is_bot || (context.msg as any).via_bot) {
            return;
        }

        const _context = new TgCommandContext(this, context);

        const chat = await this.getChat(context.chat.id, this._defaultCreationParams(_context));
        await chat.serviceChat.updateChat(context.chat as any, context.from as any);

        if (chat.ref === null) {
            chat.ref = parseStartPayload(context.msg.text) || 'none';
        }

        this.handleMessage({
            context: _context,
            chat: chat,
            serviceChat: chat.serviceChat,
            actions: new TgBotAction(this, context, chat),
            keyboard: new Keyboard(this.app, chat, _context),
            service: 'tg',
            realContext: context,
            formatter: createScheduleFormatter('tg', this.app, raspCache, chat),
            cache: this.cache
        });
    }

    protected override handleMessageError(cmd: AbstractCommand, context: AbstractCommandContext, err: Error): void {
        if (err instanceof GrammyError && [StatusCode.ClientErrorTooManyRequests, StatusCode.ClientErrorForbidden].includes(err.error_code)) {
            return;
        }

        return super.handleMessageError(cmd, context, err);
    }

    private async callbackHandler(context: TgCallbackRealContext) {
        if (context.from?.is_bot) return;
        if (!context.callbackQuery.data || !context.callbackQuery.message) return;

        const _context = new TgCallbackContext(this, context);

        const chat = await this.getChat(context.callbackQuery.message.chat.id, this._defaultCreationParams(_context));
        await chat.serviceChat.updateChat(context.callbackQuery.message.chat as any, context.callbackQuery.message.from as any);

        return this.handleCallback({
            service: 'tg',
            context: _context,
            realContext: context,
            chat: chat,
            serviceChat: chat.serviceChat,
            keyboard: new Keyboard(this.app, chat, _context),
            scheduleFormatter: createScheduleFormatter('tg', this.app, raspCache, chat),
            cache: this.cache
        });
    }

    protected _getAcceptKeyParams(context: TgCommandContext): InputRequestKey {
        return {
            from: FromType.TelegramBot,
            peer_id: context.peerId,
            sender_id: context.userId,
            time: Date.now()
        }
    }

    protected _defaultCreationParams(context: TgCommandContext | TgCallbackContext): Partial<CreationAttributes<BotChat>> {
        return {
            accepted: context.isChat ? config.accept.room : config.accept.private
        }
    }

    private async myChatMember(context: TgMyChatMemberContext) {
        const update = context.update.my_chat_member;

        if (!update) {
            return;
        }

        const chat = await this.getChat(update.chat.id, {
            accepted: config.accept.room
        });

        switch (update.new_chat_member.status) {
            case 'kicked':
                chat.allowSendMess = false;
                break;

            case 'member':
                chat.allowSendMess = true;
                break;
        }

        await chat.save();
    }
}
