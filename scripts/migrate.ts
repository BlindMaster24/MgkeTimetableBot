import { sequelize } from '../src/db';
import { createMigrator } from '../src/db/migrator';

async function main() {
    const cmd = process.argv[2] ?? 'up';
    const migrator = createMigrator(sequelize);

    switch (cmd) {
        case 'up': {
            const applied = await migrator.up();
            console.log(`Applied ${applied.length} migration(s):`);
            for (const m of applied) console.log('  -', m.name);
            break;
        }
        case 'down': {
            const reverted = await migrator.down();
            console.log(`Reverted ${reverted.length} migration(s):`);
            for (const m of reverted) console.log('  -', m.name);
            break;
        }
        case 'pending': {
            const pending = await migrator.pending();
            console.log(`Pending: ${pending.length}`);
            for (const m of pending) console.log('  -', m.name);
            break;
        }
        case 'executed': {
            const executed = await migrator.executed();
            console.log(`Executed: ${executed.length}`);
            for (const m of executed) console.log('  -', m.name);
            break;
        }
        default:
            console.error(`Unknown command: ${cmd}`);
            console.error('Usage: pnpm run migrate [up|down|pending|executed]');
            process.exitCode = 1;
    }

    await sequelize.close();
}

main().catch((err) => {
    console.error(err);
    process.exit(1);
});
