import { CreationAttributes } from 'sequelize';
import { format } from 'util';
import { config } from '../../../../config';
import { App } from '../../../app';
import { sequelize } from '../../../db';
import { defines } from '../../../defines';
import { InputRequestKey, RequestKey } from '../../../key';
import { setLogContext } from '../../../logging';
import { Logger } from '../../../logger';
import { BotChat } from '../chat';
import { AbstractBotEventListener } from '../events';
import { BotInput, InputCancel } from '../input';
import { StaticKeyboard } from '../keyboard';
import { Storage } from '../storage';
import { AbstractCallback, CbHandlerParams } from './callback';
import { AbstractCommand, BotServiceName, CmdHandlerParams } from './command';
import { AbstractCallbackContext, AbstractCommandContext } from './context';

export type HandleMessageOptions = {
    /** Есть ли упоминание бота в сообщении (для ВК) */
    selfMention?: boolean;

    /** Сообщение отправлено из чата (используется чтобы не отправлять "неизвестную команду", если люди просто общаются в чате) */
    isFromChat?: boolean;
};

export abstract class AbstractBot {
    protected abstract _getAcceptKeyParams(context: AbstractCommandContext): InputRequestKey;
    public abstract getChat(
        peerId: number | string,
        creationDefaults?: Partial<CreationAttributes<BotChat>>
    ): Promise<BotChat>;
    // public abstract getBotChat(peerId: number | string): Promise<BotChat>;
    public abstract event: AbstractBotEventListener;

    protected readonly acceptTool: RequestKey = new RequestKey(config.encrypt_key);
    public readonly input: BotInput = new BotInput();
    public readonly cache: Storage;
    public readonly app: App;
    protected readonly logger: Logger;

    public service: BotServiceName;

    constructor(app: App, service: BotServiceName) {
        this.app = app;
        this.service = service;
        this.cache = new Storage(this.service);
        this.logger = new Logger(`Bot:${this.service}`);
    }

    protected getBotService() {
        return this.app.getService('bot');
    }

    private getCallback(context: AbstractCallbackContext): AbstractCallback | null {
        return this.getBotService().getCallbackByPayload(context.parsedPayload);
    }

    protected async handleMessage(handlerParams: CmdHandlerParams, advancedOptions: HandleMessageOptions = {}) {
        const { chat, context, keyboard } = handlerParams;
        const { selfMention } = Object.assign(
            {},
            {
                selfMention: true
            },
            advancedOptions
        );

        try {
            setLogContext({
                service: this.service,
                peerId: context.peerId,
                userId: context.userId,
                messageId: context.messageId
            });
            if (!chat.allowSendMess) {
                chat.allowSendMess = true;
            }

            if (chat.accepted && !chat.eula) {
                chat.eula = true;

                context.send(defines.eula).catch(() => {});
            }

            if (chat.accepted && chat.needUpdateButtons) {
                chat.needUpdateButtons = false;
                chat.scene = null;

                context
                    .send('Клавиатура была принудительно пересоздана (обновлена)', {
                        keyboard: keyboard.MainMenu
                    })
                    .catch(() => {});
            }

            const cmdResult = this.getBotService().getCommand(context, chat);
            if (!cmdResult) {
                if (selfMention || !context.isChat) {
                    if (chat.accepted) {
                        if (this.input.has(String(context.peerId))) {
                            return this.input.resolve(String(context.peerId), context.text, 'message');
                        }

                        return this.notFound(chat, context, keyboard.MainMenu, selfMention);
                    } else {
                        return this.notAccepted(handlerParams);
                    }
                }

                return;
            }

            const { cmd, regexp } = cmdResult;
            handlerParams.regexp = regexp;
            setLogContext({ commandId: cmd.id, commandRegexp: regexp });

            if (cmd.acceptRequired && !chat.accepted) {
                if (context.isChat && !selfMention) return;

                return this.notAccepted(handlerParams);
            }

            chat.lastMsgTime = Date.now();

            try {
                if (!cmd.preHandle(handlerParams)) {
                    return this.notFound(chat, context, keyboard.MainMenu, selfMention);
                }

                if (this.input.has(String(context.peerId))) {
                    this.input.cancel(String(context.peerId));
                }

                await cmd.handler(handlerParams);
            } catch (err: any) {
                if (err instanceof InputCancel) return;
                this.logger.error('command_handler_error', {
                    commandId: cmd.id,
                    peerId: context.peerId,
                    error: err
                });

                this.handleMessageError(cmd, context, err);
            }
        } catch (err: any) {
            this.logger.error('message_pipeline_error', {
                peerId: context.peerId,
                error: err
            });
        } finally {
            await this.saveChanges(handlerParams).catch((err) => {
                this.logger.error('db_save_changes_error', { error: err });
            });
        }
    }

    protected async handleCallback(handlerParams: CbHandlerParams) {
        const { chat, context } = handlerParams;

        try {
            setLogContext({
                service: this.service,
                peerId: context.peerId,
                userId: context.userId,
                messageId: context.messageId
            });
            if (!chat.allowSendMess) {
                chat.allowSendMess = true;
            }

            if (chat.accepted && !chat.eula) {
                chat.eula = true;
                context.send(defines.eula).catch(() => {});
            }

            const cb = this.getCallback(context);
            if (!cb) {
                await context.answer('Callback not found');

                return;
            }

            if (cb.acceptRequired && !chat.accepted) {
                await context.answer('Чат не подтверждён, чтобы использовать это');

                return;
            }

            chat.lastMsgTime = Date.now();
            setLogContext({ callbackId: cb.id });

            try {
                if (!cb.preHandle(handlerParams)) {
                    await context.answer('Вы не можете использовать это');

                    return;
                }

                await cb.handler(handlerParams);
            } catch (err: any) {
                if (err instanceof InputCancel) return;
                this.logger.error('callback_handler_error', {
                    callbackId: cb.id,
                    peerId: context.peerId,
                    error: err
                });

                this.handleMessageError(cb, context, err);
            }

            if (!context.callbackAnswered) {
                await context.answer().catch(() => {});
            }
        } catch (err: any) {
            this.logger.error('callback_pipeline_error', {
                peerId: context.peerId,
                error: err
            });
        } finally {
            await this.saveChanges(handlerParams).catch((err) => {
                this.logger.error('db_save_changes_error', { error: err });
            });
        }
    }

    private async saveChanges({ chat, serviceChat }: CbHandlerParams | CmdHandlerParams): Promise<void> {
        if (!serviceChat.changed() && !chat.changed()) {
            return;
        }

        await sequelize.transaction((transaction) => {
            return Promise.all([serviceChat.save({ transaction }), chat.save({ transaction })]);
        });
    }

    protected handleMessageError(
        cmd: AbstractCommand | AbstractCallback,
        context: AbstractCommandContext | AbstractCallbackContext,
        err: Error
    ) {
        const name = (cmd as any).__proto__.__proto__.constructor.name.replace(/^Abstract/, '').toLowerCase();

        if (context instanceof AbstractCallbackContext) {
            context.answer('Произошла ошибка').catch(() => {});
        }

        context.send(`🚫 Произошла ошибка во время выполнения ${name}#${cmd.id}: ${err.toString()}`).catch(() => {});
    }

    protected notFound(chat: BotChat, context: AbstractCommandContext, keyboard: any, selfMention: boolean = true) {
        chat.scene = null; //set main scene

        return context.send('Команда не найдена', {
            ...(selfMention
                ? {
                      keyboard: keyboard
                  }
                : {})
        });
    }

    protected notAccepted({ context, service }: CmdHandlerParams) {
        return context.send(format(defines['need.accept'], this.acceptTool.getKey(this._getAcceptKeyParams(context))), {
            disable_mentions: true,
            keyboard: StaticKeyboard.NeedAccept(service)
        });
    }
}
