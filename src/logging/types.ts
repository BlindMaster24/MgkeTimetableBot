export type LogLevel = 'error' | 'warn' | 'info' | 'debug';

export type LogContext = Record<string, unknown>;

export type LogRecord = {
    timestamp: string;
    level: LogLevel;
    logger: string;
    message: string;
    context?: LogContext;
};

export type LoggingConfig = {
    level: LogLevel;
    format: 'json' | 'text';
    output: {
        stdout: boolean;
        file: {
            enabled: boolean;
            path: string;
            maxSizeMb: number;
            maxFiles: number;
        };
    };
    redact: {
        messageText: boolean;
        maxPreviewLen: number;
    };
};
