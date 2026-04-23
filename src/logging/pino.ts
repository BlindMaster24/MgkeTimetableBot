import pino, { Logger as PinoLogger, TransportTargetOptions } from 'pino';
import { LoggingConfig, LogLevel } from './types';

const LEVEL_MAP: Record<LogLevel, string> = {
    error: 'error',
    warn: 'warn',
    info: 'info',
    debug: 'debug'
};

export function createPinoLogger(cfg: LoggingConfig): PinoLogger {
    const targets: TransportTargetOptions[] = [];

    if (cfg.output.stdout) {
        if (cfg.format === 'text') {
            targets.push({
                target: 'pino-pretty',
                level: LEVEL_MAP[cfg.level],
                options: {
                    colorize: true,
                    translateTime: 'SYS:yyyy-mm-dd HH:MM:ss.l',
                    ignore: 'pid,hostname',
                    messageFormat: '[{logger}] {msg}',
                    singleLine: false
                }
            });
        } else {
            targets.push({
                target: 'pino/file',
                level: LEVEL_MAP[cfg.level],
                options: { destination: 1 }
            });
        }
    }

    if (cfg.output.file.enabled) {
        targets.push({
            target: 'pino/file',
            level: LEVEL_MAP[cfg.level],
            options: {
                destination: cfg.output.file.path,
                mkdir: true,
                append: true
            }
        });
    }

    if (targets.length === 0) {
        return pino({ level: 'silent' });
    }

    return pino({
        level: LEVEL_MAP[cfg.level],
        base: null,
        timestamp: () => `,"timestamp":"${new Date().toISOString()}"`,
        transport: { targets }
    });
}

export type { PinoLogger };
