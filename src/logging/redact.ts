import { LogContext } from './types';

const SECRET_KEYS = ['token', 'secret', 'password', 'privatekey', 'apikey', 'authorization'];

function shouldRedactKey(key: string): boolean {
    const lowered = key.toLowerCase();

    return SECRET_KEYS.some((item) => lowered.includes(item));
}

function redactString(value: string, maxPreviewLen: number): string {
    if (value.length <= maxPreviewLen) {
        return value;
    }

    return `${value.slice(0, maxPreviewLen)}...(${value.length})`;
}

export function redactContext(input: unknown, maxPreviewLen: number): unknown {
    if (input === null || input === undefined) {
        return input;
    }

    if (typeof input === 'string') {
        return redactString(input, maxPreviewLen);
    }

    if (typeof input !== 'object') {
        return input;
    }

    if (Array.isArray(input)) {
        return input.map((item) => redactContext(item, maxPreviewLen));
    }

    const source = input as Record<string, unknown>;
    const result: Record<string, unknown> = {};

    for (const [key, value] of Object.entries(source)) {
        if (shouldRedactKey(key)) {
            result[key] = '[REDACTED]';
            continue;
        }

        result[key] = redactContext(value, maxPreviewLen);
    }

    return result;
}

export function redactMessageText(context: LogContext | undefined, maxPreviewLen: number): LogContext | undefined {
    if (!context) {
        return context;
    }

    const result: LogContext = { ...context };

    if (typeof result.messageText === 'string') {
        const text = result.messageText;
        result.messageTextLength = text.length;
        result.messageText = redactString(text, maxPreviewLen);
    }

    if (typeof result.text === 'string') {
        const text = result.text;
        result.textLength = text.length;
        result.text = redactString(text, maxPreviewLen);
    }

    return result;
}
