import { Context, InputFile } from 'grammy';
import { TgBot } from '.';
import { config } from '../../../../config';
import { ParsedPayload, parsePayload } from '../../../utils';
import { ImageFile } from '../../image/builder';
import { AbstractCallbackContext, AbstractCommandContext, MessageOptions } from '../abstract';
import { StaticKeyboard } from '../keyboard';
import { convertAbstractToTg } from './keyboard';

type TgUser = {
    id: number;
    is_bot?: boolean;
    username?: string;
    first_name?: string;
    last_name?: string;
    language_code?: string;
};

type TgChat = {
    id: number;
    type: string;
    username?: string;
};

type TgMessage = {
    message_id: number;
    chat: TgChat;
    from?: TgUser;
    text?: string;
    photo?: Array<{ file_id: string }>;
};

type TgCallbackQueryData = {
    id: string;
    from: TgUser;
    data?: string;
    message?: TgMessage;
};

export type TgMessageRealContext = Context & {
    chat: TgChat;
    from?: TgUser;
    msg: TgMessage;
};

export type TgCallbackRealContext = Context & {
    from: TgUser;
    callbackQuery: TgCallbackQueryData;
};

function resolveReplyTo(replyTo?: string): number | undefined {
    if (replyTo == null) {
        return;
    }

    const value = Number(replyTo);

    if (!Number.isFinite(value)) {
        return;
    }

    return value;
}

function appendCancelHint(text: string, options: MessageOptions): { text: string; options: MessageOptions } {
    if (options?.keyboard?.name !== StaticKeyboard.Cancel.name) {
        return { text, options };
    }

    const nextOptions = { ...options };
    delete nextOptions.keyboard;

    return {
        text: `${text}\n\nНапишите /cancel для отмены`,
        options: nextOptions
    };
}

function buildSendOptions(options: MessageOptions = {}, replyTo?: number): any {
    return {
        ...(!options.disableHtmlParser ? { parse_mode: 'HTML' } : {}),
        ...(replyTo
            ? {
                  reply_parameters: {
                      message_id: replyTo,
                      allow_sending_without_reply: true
                  }
              }
            : {}),
        disable_notification: options.disable_mentions,
        reply_markup: convertAbstractToTg(options.keyboard)
    };
}

function buildEditOptions(options: MessageOptions = {}): any {
    return {
        ...(!options.disableHtmlParser ? { parse_mode: 'HTML' } : {}),
        reply_markup: convertAbstractToTg(options.keyboard)
    };
}

export class TgCommandContext extends AbstractCommandContext {
    public text: string;
    public parsedPayload: undefined;
    public peerId: number;
    public userId: number;
    public messageId?: number;

    private context: TgMessageRealContext;

    constructor(bot: TgBot, context: TgMessageRealContext) {
        super(bot);

        this.context = context;
        this.text = context.msg?.text || '';

        this.peerId = context.chat.id;
        this.userId = context.from?.id || 0;
    }

    get isChat(): boolean {
        return this.context.chat.type !== 'private';
    }

    public async send(text: string, options: MessageOptions = {}): Promise<string> {
        const replyTo = resolveReplyTo(options.reply_to);
        const prepared = appendCancelHint(text, options);

        const result = await this.context.api.sendMessage(
            this.peerId,
            prepared.text,
            buildSendOptions(prepared.options, replyTo)
        );

        this.messageId = result.message_id;

        return String(result.message_id);
    }

    public async editOrSend(text: string, options: MessageOptions = {}): Promise<boolean> {
        if (!this.messageId) {
            const result = await this.send(text, options);

            return Boolean(result);
        }

        const prepared = appendCancelHint(text, options);

        try {
            await this.context.api.editMessageText(
                this.peerId,
                this.messageId,
                prepared.text,
                buildEditOptions(prepared.options)
            );

            return true;
        } catch {
            const result = await this.send(text, options);

            return Boolean(result);
        }
    }

    public async sendPhoto(image: ImageFile, options: MessageOptions = {}): Promise<string> {
        const replyTo = resolveReplyTo(options.reply_to);

        let fileId = await this.cache.get(image.id);

        const photo = fileId || new InputFile(await image.data(), `${image.id}.png`);

        const result = await this.context.api.sendPhoto(this.peerId, photo, buildSendOptions(options, replyTo));

        const photoList = (result as TgMessage).photo;
        if (!fileId && Array.isArray(photoList) && photoList.length > 0) {
            fileId = photoList[photoList.length - 1].file_id;
            await this.cache.add(image.id, fileId);
        }

        return String(result.message_id);
    }

    public async sendFile(data: Buffer, filename: string, options: MessageOptions = {}): Promise<string> {
        const replyTo = resolveReplyTo(options.reply_to);

        const result = await this.context.api.sendDocument(
            this.peerId,
            new InputFile(data, filename),
            buildSendOptions(options, replyTo)
        );

        return String(result.message_id);
    }

    public async delete(id?: string): Promise<boolean> {
        if (!id && !this.messageId) {
            throw new Error('there are no message to delete');
        }

        const targetId = id ? Number(id) : this.messageId;

        if (!targetId) {
            return false;
        }

        return this.context.api.deleteMessage(this.peerId, targetId);
    }

    public async isChatAdmin(): Promise<boolean> {
        return false;
    }
}

export class TgCallbackContext extends AbstractCallbackContext {
    public peerId: number;
    public userId: number;
    public messageId: number;
    public callbackAnswered: boolean = false;
    public parsedPayload?: ParsedPayload;

    private context: TgCallbackRealContext;
    private message: TgMessage;

    constructor(bot: TgBot, context: TgCallbackRealContext) {
        super(bot);

        this.context = context;

        if (!context.callbackQuery.message) {
            throw new Error('there is no message context');
        }

        this.message = context.callbackQuery.message;
        this.messageId = this.message.message_id;

        this.peerId = this.message.chat.id;
        this.userId = context.from.id;

        this.parsedPayload = context.callbackQuery.data ? parsePayload(context.callbackQuery.data) : undefined;
    }

    get isChat(): boolean {
        return this.message.chat.type !== 'private';
    }

    public async answer(text?: string): Promise<boolean> {
        this.callbackAnswered = true;

        await this.context.answerCallbackQuery({ text });

        return true;
    }

    public async send(text: string, options: MessageOptions = {}): Promise<string> {
        const replyTo = resolveReplyTo(options.reply_to);
        const prepared = appendCancelHint(text, options);

        const result = await this.context.api.sendMessage(
            this.peerId,
            prepared.text,
            buildSendOptions(prepared.options, replyTo)
        );

        return String(result.message_id);
    }

    public async editOrSend(text: string, options: MessageOptions = {}): Promise<boolean> {
        const prepared = appendCancelHint(text, options);

        try {
            await this.context.api.editMessageText(
                this.peerId,
                this.messageId,
                prepared.text,
                buildEditOptions(prepared.options)
            );

            return true;
        } catch {
            const result = await this.send(text, options);

            return Boolean(result);
        }
    }

    public async sendPhoto(image: ImageFile, options: MessageOptions = {}): Promise<string> {
        const replyTo = resolveReplyTo(options.reply_to);

        let fileId = await this.cache.get(image.id);

        const photo = !config.dev && fileId ? fileId : new InputFile(await image.data(), `${image.id}.png`);

        const result = await this.context.api.sendPhoto(this.peerId, photo, buildSendOptions(options, replyTo));

        const photoList = (result as TgMessage).photo;
        if (!fileId && Array.isArray(photoList) && photoList.length > 0) {
            fileId = photoList[photoList.length - 1].file_id;

            await this.cache.add(image.id, fileId);
        }

        return String(result.message_id);
    }

    public async sendFile(data: Buffer, filename: string, options: MessageOptions = {}): Promise<string> {
        const replyTo = resolveReplyTo(options.reply_to);

        const result = await this.context.api.sendDocument(
            this.peerId,
            new InputFile(data, filename),
            buildSendOptions(options, replyTo)
        );

        return String(result.message_id);
    }

    public async delete(id?: string): Promise<boolean> {
        return this.context.api.deleteMessage(this.peerId, id ? Number(id) : this.messageId);
    }

    public async isChatAdmin(): Promise<boolean> {
        return false;
    }
}
