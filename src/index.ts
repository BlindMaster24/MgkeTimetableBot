import { config } from '../config';
import { App } from './app';
import { validateConfig } from './config/validate';
import { startVanishCronJob as setupVanishCron } from './db/clean';
import { Logger } from './logger';

validateConfig(config);

const app = new App(config.services);

app.runServices();

setupVanishCron(app);

const shutdownLogger = new Logger('CORE');
let shuttingDown = false;
const shutdown = async (signal: NodeJS.Signals) => {
    if (shuttingDown) return;
    shuttingDown = true;
    shutdownLogger.log(`Получен ${signal}, завершаем работу...`);

    const forceExit = setTimeout(() => {
        shutdownLogger.error('shutdown_force_exit', { reason: 'timeout' });
        process.exit(1);
    }, 30_000);
    forceExit.unref();

    try {
        await app.stop();
        clearTimeout(forceExit);
        process.exit(0);
    } catch (error) {
        shutdownLogger.error('shutdown_failed', { error });
        clearTimeout(forceExit);
        process.exit(1);
    }
};

process.once('SIGTERM', () => void shutdown('SIGTERM'));
process.once('SIGINT', () => void shutdown('SIGINT'));

export { app };
