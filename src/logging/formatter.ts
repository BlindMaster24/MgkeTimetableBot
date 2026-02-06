import { LogRecord } from './types';

export function formatJson(record: LogRecord): string {
    return JSON.stringify(record);
}

export function formatText(record: LogRecord): string {
    const context = record.context ? ` ${JSON.stringify(record.context)}` : '';

    return `${record.timestamp} [${record.level}] [${record.logger}] ${record.message}${context}`;
}
