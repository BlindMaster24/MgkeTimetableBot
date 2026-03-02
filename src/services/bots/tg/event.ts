import { Bot, GrammyError } from "grammy";
import StatusCode from "status-code-enum";
import { config } from "../../../../config";
import { App } from "../../../app";
import { Logger } from "../../../logger";
import { BotServiceName, MessageOptions } from "../abstract";
import { BotChat } from "../chat";
import { AbstractBotEventListener } from "../events";
import { Keyboard } from '../keyboard';
import { TgChat } from './chat';
import { convertAbstractToTg } from "./keyboard";

export class TgEventListener extends AbstractBotEventListener {
    protected _model = TgChat;
    public readonly service: BotServiceName = 'tg';

    private bot: Bot;
    private readonly logger = new Logger('Bot:tg:event');

    constructor(app: App, bot: Bot) {
        super(app)
        this.bot = bot
    }

    protected getAdminPeerIds(): number[] {
        return config.telegram.admin_ids;
    }

    public async sendMessage(chat: BotChat<TgChat>, message: string, options: MessageOptions = {}) {
        return this.bot.api.sendMessage(
            chat.serviceChat.peerId,
            message,
            {
                ...(!options.disableHtmlParser ? {
                    parse_mode: 'HTML',
                } : {}),
                disable_notification: options.disable_mentions,
                reply_markup: convertAbstractToTg(options.keyboard ? options.keyboard : new Keyboard(this.app, chat).MainMenu)
            }
        ).catch((err: Error) => {
            if (err instanceof GrammyError && err.error_code === StatusCode.ClientErrorForbidden) {
                chat.allowSendMess = false;
                return;
            }

            this.logger.error('tg_send_event_error', {
                error: err,
                peerId: chat.serviceChat.peerId
            });
        })
    }
}
