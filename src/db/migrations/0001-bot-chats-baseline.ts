import { DataTypes, QueryInterface } from 'sequelize';

type Params = { context: QueryInterface };

export async function up({ context: qi }: Params): Promise<void> {
    const table = await qi.describeTable('bot_chats').catch(() => null);
    if (!table) {
        return;
    }
    const indexes = ((await qi.showIndex('bot_chats')) as Array<{ name: string }>) ?? [];
    const hasIndex = (name: string) => indexes.some((idx) => idx.name === name);

    if (!('noticeCalls' in table)) {
        await qi.addColumn('bot_chats', 'noticeCalls', {
            type: DataTypes.BOOLEAN,
            allowNull: false,
            defaultValue: true
        });
    }
    if (!('diffEnabled' in table)) {
        await qi.addColumn('bot_chats', 'diffEnabled', {
            type: DataTypes.BOOLEAN,
            allowNull: false,
            defaultValue: true
        });
    }
    if (!('diffAutoInWeek' in table)) {
        await qi.addColumn('bot_chats', 'diffAutoInWeek', {
            type: DataTypes.BOOLEAN,
            allowNull: false,
            defaultValue: true
        });
    }
    if (!('diffAutoInUpdates' in table)) {
        await qi.addColumn('bot_chats', 'diffAutoInUpdates', {
            type: DataTypes.BOOLEAN,
            allowNull: false,
            defaultValue: true
        });
    }
    if (!('diffShowBeforeAfter' in table)) {
        await qi.addColumn('bot_chats', 'diffShowBeforeAfter', {
            type: DataTypes.BOOLEAN,
            allowNull: false,
            defaultValue: true
        });
    }
    if (!('diffMaxLines' in table)) {
        await qi.addColumn('bot_chats', 'diffMaxLines', {
            type: DataTypes.INTEGER,
            allowNull: false,
            defaultValue: 20
        });
    }

    if (!hasIndex('bot_chats_service_accepted_allow_send_mess')) {
        await qi.addIndex('bot_chats', ['service', 'accepted', 'allowSendMess'], {
            name: 'bot_chats_service_accepted_allow_send_mess'
        });
    }

    if (!hasIndex('bot_chats_service_mode_group')) {
        await qi.addIndex('bot_chats', ['service', 'mode', 'group'], {
            name: 'bot_chats_service_mode_group'
        });
    }

    if (!hasIndex('bot_chats_service_mode_teacher')) {
        await qi.addIndex('bot_chats', ['service', 'mode', 'teacher'], {
            name: 'bot_chats_service_mode_teacher'
        });
    }
}

export async function down({ context: qi }: Params): Promise<void> {
    const table = await qi.describeTable('bot_chats').catch(() => null);
    if (!table) {
        return;
    }

    const indexes = ((await qi.showIndex('bot_chats')) as Array<{ name: string }>) ?? [];
    const hasIndex = (name: string) => indexes.some((idx) => idx.name === name);

    const dropIndex = async (name: string) => {
        if (hasIndex(name)) {
            await qi.removeIndex('bot_chats', name);
        }
    };
    await dropIndex('bot_chats_service_mode_teacher');
    await dropIndex('bot_chats_service_mode_group');
    await dropIndex('bot_chats_service_accepted_allow_send_mess');

    const dropColumn = async (name: string) => {
        if (name in table) {
            await qi.removeColumn('bot_chats', name);
        }
    };
    await dropColumn('diffMaxLines');
    await dropColumn('diffShowBeforeAfter');
    await dropColumn('diffAutoInUpdates');
    await dropColumn('diffAutoInWeek');
    await dropColumn('diffEnabled');
    await dropColumn('noticeCalls');
}
