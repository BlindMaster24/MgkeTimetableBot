import { InferAttributes, ModelStatic, WhereOptions } from "sequelize";
import { config } from "../../../../config";
import { AbstractServiceChat, BotChat } from "../chat";

export function buildBaseChatWhere(service: string, where?: WhereOptions<InferAttributes<BotChat>>): WhereOptions<InferAttributes<BotChat>> {
    return Object.assign({
        accepted: true,
        allowSendMess: true,
        service,
        ...(config.dev ? {
            noticeParserErrors: true
        } : {})
    }, where);
}

export function attachServiceChat(chats: BotChat[], model: ModelStatic<AbstractServiceChat>): BotChat[] {
    return chats.map(chat => {
        chat.serviceChat = (chat as any)[model.name];

        return chat;
    });
}

