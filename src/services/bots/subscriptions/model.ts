import { CreationOptional, DataTypes, InferAttributes, InferCreationAttributes, Model } from 'sequelize';
import { sequelize } from '../../../db';

export type SubscriptionType = 'group' | 'teacher';

class Subscription extends Model<InferAttributes<Subscription>, InferCreationAttributes<Subscription>> {
    declare id: CreationOptional<number>;
    declare chatId: number;
    declare type: SubscriptionType;
    declare value: string;
}

Subscription.init({
    id: {
        type: DataTypes.INTEGER,
        primaryKey: true,
        autoIncrement: true
    },
    chatId: {
        type: DataTypes.INTEGER,
        allowNull: false
    },
    type: {
        type: DataTypes.ENUM<SubscriptionType>('group', 'teacher'),
        allowNull: false
    },
    value: {
        type: DataTypes.STRING,
        allowNull: false
    }
}, {
    sequelize: sequelize,
    tableName: 'bot_subscriptions',
    indexes: [
        {
            fields: ['chatId']
        },
        {
            fields: ['type', 'value']
        },
        {
            fields: ['chatId', 'type', 'value'],
            unique: true
        }
    ]
});

export { Subscription };
