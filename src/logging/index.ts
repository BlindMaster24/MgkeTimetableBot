import { config } from '../../config';
import { LoggingEngine } from './engine';
import { LoggingConfig } from './types';

const defaultConfig: LoggingConfig = {
    level: config.logging?.level ?? (config.dev ? 'debug' : 'info'),
    format: config.logging?.format ?? 'json',
    output: {
        stdout: config.logging?.output.stdout ?? true,
        file: {
            enabled: config.logging?.output.file.enabled ?? false,
            path: config.logging?.output.file.path ?? './logs/app.log',
            maxSizeMb: config.logging?.output.file.maxSizeMb ?? 10,
            maxFiles: config.logging?.output.file.maxFiles ?? 5
        }
    },
    redact: {
        messageText: config.logging?.redact.messageText ?? true,
        maxPreviewLen: config.logging?.redact.maxPreviewLen ?? 128
    }
};

export const loggingEngine = new LoggingEngine(defaultConfig);

export * from './types';
export * from './context';
