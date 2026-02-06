import { BotChat } from './Chat';
import { LessonAlias } from './LessonAlias';
import { Subscription } from '../subscriptions/model';

BotChat.hasMany(LessonAlias, {
    sourceKey: 'id',
    foreignKey: 'chatId'
});

LessonAlias.belongsTo(BotChat, {
    foreignKey: 'chatId',
    targetKey: 'id'
});

BotChat.hasMany(Subscription, {
    sourceKey: 'id',
    foreignKey: 'chatId',
    as: 'subscriptions'
});

Subscription.belongsTo(BotChat, {
    foreignKey: 'chatId',
    targetKey: 'id',
    as: 'chat'
});

export * from './Abstract';
export * from './Chat';
export * from './LessonAlias';
