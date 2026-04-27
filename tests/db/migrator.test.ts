import { DataTypes, Sequelize } from 'sequelize';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { createMigrator } from '../../src/db/migrator';

describe('db migrator (umzug)', () => {
    let sequelize: Sequelize;

    beforeEach(async () => {
        sequelize = new Sequelize({ dialect: 'sqlite', storage: ':memory:', logging: false });
        sequelize.define(
            'BotChat',
            {
                id: { type: DataTypes.INTEGER, primaryKey: true, autoIncrement: true },
                service: { type: DataTypes.STRING, allowNull: false },
                peerId: { type: DataTypes.STRING, allowNull: false },
                accepted: { type: DataTypes.BOOLEAN, allowNull: false, defaultValue: false },
                allowSendMess: { type: DataTypes.BOOLEAN, allowNull: false, defaultValue: true },
                mode: { type: DataTypes.STRING, allowNull: false, defaultValue: 'student' },
                group: { type: DataTypes.STRING, allowNull: true },
                teacher: { type: DataTypes.STRING, allowNull: true }
            },
            { tableName: 'bot_chats', timestamps: false }
        );
        await sequelize.sync();
    });

    afterEach(async () => {
        await sequelize.close();
    });

    it('applies the baseline migration on a fresh DB', async () => {
        const umzug = createMigrator(sequelize);
        const applied = await umzug.up();

        const names = applied.map((m) => m.name);
        expect(names.some((n) => n.includes('0001-bot-chats-baseline'))).toBe(true);

        const qi = sequelize.getQueryInterface();
        const table = await qi.describeTable('bot_chats');
        expect('diffEnabled' in table).toBe(true);
        expect('diffMaxLines' in table).toBe(true);
        expect('noticeCalls' in table).toBe(true);

        const indexes = (await qi.showIndex('bot_chats')) as Array<{ name: string }>;
        expect(indexes.some((i) => i.name === 'bot_chats_service_mode_group')).toBe(true);
    });

    it('is idempotent when applied twice (no pending on second run)', async () => {
        const umzug = createMigrator(sequelize);
        await umzug.up();
        const pending = await umzug.pending();
        expect(pending.length).toBe(0);
    });

    it('tracks applied migrations in SequelizeMeta', async () => {
        const umzug = createMigrator(sequelize);
        await umzug.up();

        const executed = await umzug.executed();
        expect(executed.length).toBeGreaterThan(0);

        const [rows] = await sequelize.query('SELECT name FROM "SequelizeMeta"');
        expect((rows as Array<{ name: string }>).length).toBe(executed.length);
    });

    it('down() removes added columns and indexes', async () => {
        const umzug = createMigrator(sequelize);
        await umzug.up();
        await umzug.down({ to: 0 });

        const qi = sequelize.getQueryInterface();
        const table = await qi.describeTable('bot_chats');
        expect('diffMaxLines' in table).toBe(false);
        expect('noticeCalls' in table).toBe(false);

        const indexes = (await qi.showIndex('bot_chats')) as Array<{ name: string }>;
        expect(indexes.some((i) => i.name === 'bot_chats_service_mode_group')).toBe(false);
    });
});
