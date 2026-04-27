import { AsyncLocalStorage } from 'async_hooks';
import { randomUUID } from 'crypto';
import { LogContext } from './types';

const store = new AsyncLocalStorage<LogContext>();

export function runWithLogContext<T>(context: LogContext, fn: () => T): T {
    const parent = store.getStore() ?? {};
    return store.run({ ...parent, ...context }, fn);
}

export function getLogContext(): LogContext {
    return store.getStore() ?? {};
}

export function setLogContext(context: LogContext): void {
    const current = store.getStore();
    if (!current) {
        return;
    }

    Object.assign(current, context);
}

export function newTraceId(): string {
    return randomUUID();
}
