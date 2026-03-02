import { TgBot } from ".";
import { AbstractAction, AbstractCommandContext } from "../abstract";
import { BotChat } from "../chat";
import { TgCommandContext, TgMessageRealContext } from "./context";
import { Logger } from "../../../logger";

export class TgBotAction extends AbstractAction {
    protected context: TgMessageRealContext;
    protected chat: BotChat;
    protected _context: AbstractCommandContext;
    private readonly logger: Logger;

    constructor(bot: TgBot, context: TgMessageRealContext, chat: BotChat) {
        super();
        this.context = context;
        this.chat = chat;
        this._context = new TgCommandContext(bot, context);
        this.logger = new Logger('Bot:tg:action');
    }

    async deleteLastMsg() {
        if (!this.chat.deleteLastMsg) return false;
        if (this.chat.lastMsgId == null) return false;

        try {
            await this.context.api.deleteMessage(this.context.chat.id, this.chat.lastMsgId);
        } catch (err: any) {
            this.logger.error('action_delete_last_message_error', {
                error: err,
                chatId: this.context.chat.id,
                messageId: this.chat.lastMsgId
            });
            return false;
        }

        this.chat.lastMsgId = null;

        return true;
    }

    async deleteUserMsg() {
        if (!this.chat.deleteUserMsg) return false;

        try {
            await this.context.api.deleteMessage(this.context.chat.id, this.context.msg.message_id);
        } catch (err: any) {
            /*if (err.code == 15) {
                if (err.message.includes('(admin message)')) return false;

                if (!await this._context.isAdmin()) {
                    db.prepare('UPDATE `vk_bot_chats` SET `deleteUserMsg` = 0 WHERE `peerId` = ?').run(this.context.peerId)
                    await this.context.send('Удаление сообщений при нажатии кнопки выключено.\nПричина: нет прав администратора')
                    return false
                }

                console.error('actionDeleteUserMsg_1', err, this.context)

                return false;
            }*/

            this.logger.error('action_delete_user_message_error', {
                error: err,
                chatId: this.context.chat.id,
                messageId: this.context.msg.message_id
            });
            return false;
        }

        return true;
    }

    async handlerLastMsgUpdate(messageId: string) {
        if (!this.chat.deleteLastMsg) return false;

        this.chat.lastMsgId = Number(messageId);

        return true;
    }
}
