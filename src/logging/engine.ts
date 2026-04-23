import { createPinoLogger, PinoLogger } from './pino';
import { redactContext, redactMessageText } from './redact';
import { LogContext, LoggingConfig, LogLevel } from './types';

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
    private pino: PinoLogger;

    constructor(private cfg: LoggingConfig) {
        this.pino = createPinoLogger(cfg);
    }

    public async write(level: LogLevel, logger: string, message: string, context?: LogContext): Promise<void> {
        let normalized = context ? normalizeError(context) : undefined;

        if (normalized) {
            normalized = redactContext(normalized, this.cfg.redact.maxPreviewLen) as LogContext;
            if (this.cfg.redact.messageText) {
                normalized = redactMessageText(normalized, this.cfg.redact.maxPreviewLen);
            }
        }

        const payload: Record<string, unknown> = { logger };
        if (normalized) {
            payload.context = normalized;
        }

        this.pino[level](payload, message);
    }
}
