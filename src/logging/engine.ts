import { formatJson, formatText } from './formatter';
import { redactContext, redactMessageText } from './redact';
import { writeFileLine } from './transport/file';
import { writeStdout } from './transport/stdout';
import { LogContext, LogLevel, LogRecord, LoggingConfig } from './types';

const LEVEL_WEIGHT: Record<LogLevel, number> = {
    error: 40,
    warn: 30,
    info: 20,
    debug: 10
};

function shouldWrite(target: LogLevel, current: LogLevel): boolean {
    return LEVEL_WEIGHT[target] >= LEVEL_WEIGHT[current];
}

function normalizeError(context: LogContext): LogContext {
    const next = { ...context };

    if (next.error instanceof Error) {
        const err = next.error;
        next.error = {
            name: err.name,
            message: err.message,
            stack: err.stack
        };
    }

    return next;
}

export class LoggingEngine {
    constructor(private cfg: LoggingConfig) {}

    public async write(level: LogLevel, logger: string, message: string, context?: LogContext): Promise<void> {
        if (!shouldWrite(level, this.cfg.level)) {
            return;
        }

        let normalized = context ? normalizeError(context) : undefined;

        if (normalized) {
            normalized = redactContext(normalized, this.cfg.redact.maxPreviewLen) as LogContext;
            if (this.cfg.redact.messageText) {
                normalized = redactMessageText(normalized, this.cfg.redact.maxPreviewLen);
            }
        }

        const record: LogRecord = {
            timestamp: new Date().toISOString(),
            level,
            logger,
            message,
            ...(normalized ? { context: normalized } : {})
        };

        const line = this.cfg.format === 'json' ? formatJson(record) : formatText(record);

        if (this.cfg.output.stdout) {
            writeStdout(line);
        }

        if (this.cfg.output.file.enabled) {
            await writeFileLine(
                this.cfg.output.file.path,
                line,
                this.cfg.output.file.maxSizeMb,
                this.cfg.output.file.maxFiles
            );
        }
    }
}
