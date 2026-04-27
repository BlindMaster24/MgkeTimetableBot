import path from 'path';
import { QueryInterface, Sequelize } from 'sequelize';
import { SequelizeStorage, Umzug } from 'umzug';
import { sequelize as defaultSequelize } from './index';

export type MigrationContext = QueryInterface;

export type Migration = {
    up: (params: { context: MigrationContext }) => Promise<void>;
    down: (params: { context: MigrationContext }) => Promise<void>;
};

const migrationsGlob = path.resolve(__dirname, 'migrations/*.{ts,js}');

export function createMigrator(sequelize: Sequelize = defaultSequelize): Umzug<MigrationContext> {
    return new Umzug<MigrationContext>({
        migrations: {
            glob: migrationsGlob,
            resolve: ({ name, path: filePath, context }) => ({
                name,
                up: async () => {
                    if (!filePath) {
                        throw new Error(`Migration ${name} has no path`);
                    }
                    const migration: Migration = await import(filePath);
                    await migration.up({ context });
                },
                down: async () => {
                    if (!filePath) {
                        throw new Error(`Migration ${name} has no path`);
                    }
                    const migration: Migration = await import(filePath);
                    await migration.down({ context });
                }
            })
        },
        context: sequelize.getQueryInterface(),
        storage: new SequelizeStorage({ sequelize }),
        logger: undefined
    });
}

export async function runMigrations(sequelize: Sequelize = defaultSequelize): Promise<string[]> {
    const migrator = createMigrator(sequelize);
    const applied = await migrator.up();
    return applied.map((m) => m.name);
}

export async function getAppliedMigrations(sequelize: Sequelize = defaultSequelize): Promise<string[]> {
    const migrator = createMigrator(sequelize);
    const executed = await migrator.executed();
    return executed.map((m) => m.name);
}
